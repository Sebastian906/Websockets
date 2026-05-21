package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Constants
const (
	pingInterval   = 30 * time.Second
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	maxMessageSize = 1024 * 1024 // 1 MB — matches maxPayload in server.js
)

// Hub manages all WebSocket clients and their per-match subscriptions.
// It is the Go equivalent of the module-level maps in server.js / server.py.
type Hub struct {
	mu sync.RWMutex

	// All connected clients (for global broadcasts like match_created).
	allClients map[*Client]struct{}

	// matchId → set of clients subscribed to that match.
	matchSubscribers map[int]map[*Client]struct{}
}

// NewHub creates and returns an initialised Hub.
func NewHub() *Hub {
	return &Hub{
		allClients:       make(map[*Client]struct{}),
		matchSubscribers: make(map[int]map[*Client]struct{}),
	}
}

// registration helpers
func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allClients[c] = struct{}{}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.allClients, c)
	for matchID := range c.subscriptions {
		h.removeSubscription(matchID, c)
	}
}

func (h *Hub) subscribe(matchID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.matchSubscribers[matchID] == nil {
		h.matchSubscribers[matchID] = make(map[*Client]struct{})
	}
	h.matchSubscribers[matchID][c] = struct{}{}

	c.mu.Lock()
	c.subscriptions[matchID] = struct{}{}
	c.mu.Unlock()
}

func (h *Hub) unsubscribe(matchID int, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeSubscription(matchID, c)

	c.mu.Lock()
	delete(c.subscriptions, matchID)
	c.mu.Unlock()
}

// removeSubscription must be called with h.mu held.
func (h *Hub) removeSubscription(matchID int, c *Client) {
	subs := h.matchSubscribers[matchID]
	if subs == nil {
		return
	}
	delete(subs, c)
	if len(subs) == 0 {
		delete(h.matchSubscribers, matchID)
	}
}

// setClientSubscriptions replaces the client's subscription set with the
// provided list of match IDs, removing stale ones and adding new ones.
// Mirrors the setSubscriptions message sent by the frontend on reconnect.
func (h *Hub) setClientSubscriptions(c *Client, ids []int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	requested := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		requested[id] = struct{}{}
	}

	// Collect stale subscriptions.
	toRemove := make([]int, 0)
	c.mu.Lock()
	for mid := range c.subscriptions {
		if _, ok := requested[mid]; !ok {
			toRemove = append(toRemove, mid)
		}
	}
	c.mu.Unlock()

	// Remove stale.
	for _, mid := range toRemove {
		h.removeSubscription(mid, c)
		c.mu.Lock()
		delete(c.subscriptions, mid)
		c.mu.Unlock()
	}

	// Add new.
	for _, id := range ids {
		if h.matchSubscribers[id] == nil {
			h.matchSubscribers[id] = make(map[*Client]struct{})
		}
		if _, ok := h.matchSubscribers[id][c]; !ok {
			h.matchSubscribers[id][c] = struct{}{}
			c.mu.Lock()
			c.subscriptions[id] = struct{}{}
			c.mu.Unlock()
		}
	}
}

// BroadcastMatchCreated sends a match_created event to every connected client.
// Mirrors broadcastToAll / broadcast_match_created.
func (h *Hub) BroadcastMatchCreated(match any) {
	payload := map[string]any{"type": "match_created", "data": match}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.allClients))
	for c := range h.allClients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.sendJSON(payload)
	}
}

// BroadcastCommentary sends a commentary event to clients subscribed to matchID.
// Mirrors broadcastToMatch / broadcast_commentary.
func (h *Hub) BroadcastCommentary(matchID int, comment any) {
	payload := map[string]any{"type": "commentary", "data": comment}
	h.mu.RLock()
	subs := h.matchSubscribers[matchID]
	clients := make([]*Client, 0, len(subs))
	for c := range subs {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.sendJSON(payload)
	}
}

// BroadcastScoreUpdate sends a score_update event to subscribed clients.
// Mirrors broadcastScoreUpdate / broadcast_score_update.
func (h *Hub) BroadcastScoreUpdate(matchID, homeScore, awayScore int) {
	payload := map[string]any{
		"type":    "score_update",
		"matchId": matchID,
		"data": map[string]int{
			"homeScore": homeScore,
			"awayScore": awayScore,
		},
	}
	h.mu.RLock()
	subs := h.matchSubscribers[matchID]
	clients := make([]*Client, 0, len(subs))
	for c := range subs {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.sendJSON(payload)
	}
}

// Client represents a single WebSocket connection.
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	mu            sync.Mutex
	subscriptions map[int]struct{}
}

// newClient allocates a Client linked to the hub.
func newClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[int]struct{}),
	}
}

// sendJSON marshals payload and queues it for delivery.
func (c *Client) sendJSON(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// Buffer full — drop silently.
	}
}

// writePump drains the send channel and forwards messages to the WebSocket.
// It also sends periodic pings (mirrors setInterval ping in server.js).
func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads incoming messages and dispatches them to handleMessage.
// It tears down the client on disconnect (mirrors socket.on('close')).
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		close(c.send)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	// SetPongHandler must be set BEFORE SetReadDeadline so the initial deadline
	// is properly tracked. The handler resets the deadline on every pong.
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// Read the first message with no delay — the deadline from above covers us.

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure, websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] read error: %v", err)
			}
			break
		}
		c.handleMessage(raw)
	}
}

// handleMessage parses an incoming client message and acts on it.
// Mirrors handleMessage() in server.js and _handle_message() in server.py.
//
// Uses a generic map decode so that matchId values sent as a JSON number
// (float64 after decode) or as a numeric string (from Set<string> on
// reconnect) are both accepted via parseMatchID.
func (c *Client) handleMessage(raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendJSON(map[string]string{"type": "error", "message": "Invalid JSON"})
		return
	}

	t, _ := msg["type"].(string)

	switch t {
	case "subscribe":
		id, ok := parseMatchID(msg["matchId"])
		if !ok {
			return
		}
		c.hub.subscribe(id, c)
		c.sendJSON(map[string]any{"type": "subscribed", "matchId": id})

	case "unsubscribe":
		id, ok := parseMatchID(msg["matchId"])
		if !ok {
			return
		}
		c.hub.unsubscribe(id, c)
		c.sendJSON(map[string]any{"type": "unsubscribed", "matchId": id})

	case "setSubscriptions":
		// The frontend sends this on reconnect to restore its subscription set.
		// matchIds is []interface{} containing float64 or string elements.
		rawIDs, _ := msg["matchIds"].([]interface{})
		if len(rawIDs) == 0 {
			return
		}
		ids := make([]int, 0, len(rawIDs))
		for _, v := range rawIDs {
			if id, ok := parseMatchID(v); ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return
		}
		c.hub.setClientSubscriptions(c, ids)
		c.sendJSON(map[string]any{"type": "subscriptions", "matchIds": ids})

	// Unknown messages are silently ignored — same behaviour as JS and Python.
	}
}

// parseMatchID converts a JSON-decoded value (float64 from a JSON number, or
// a numeric string) to an integer match ID.  Returns (0, false) on failure.
func parseMatchID(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n, true
		}
	case int:
		return t, true
	}
	return 0, false
}

// Upgrader upgrades an HTTP connection to WebSocket.
// CheckOrigin is permissive — CORS is enforced at the HTTP layer.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}