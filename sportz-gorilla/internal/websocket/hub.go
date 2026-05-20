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
// Silently drops the message if the send buffer is full (mirroring the
// `if socket.readyState !== WebSocket.OPEN` guard in server.js).
func (c *Client) sendJSON(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// Buffer full — drop silently (client is too slow).
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
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			// Treat normal closes and "no status" (1005) as expected and
			// avoid noisy logs. Only log truly unexpected close errors.
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
func (c *Client) handleMessage(raw []byte) {
	var msg struct {
		Type     string        `json:"type"`
		MatchID  *int          `json:"matchId"`
		MatchIDs []interface{} `json:"matchIds"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendJSON(map[string]string{"type": "error", "message": "Invalid JSON"})
		return
	}

	switch msg.Type {
	case "subscribe":
		if msg.MatchID == nil {
			return
		}
		c.hub.subscribe(*msg.MatchID, c)
		c.sendJSON(map[string]any{"type": "subscribed", "matchId": *msg.MatchID})

	case "unsubscribe":
		if msg.MatchID == nil {
			return
		}
		c.hub.unsubscribe(*msg.MatchID, c)
		c.sendJSON(map[string]any{"type": "unsubscribed", "matchId": *msg.MatchID})

	case "setSubscriptions":
		if len(msg.MatchIDs) == 0 {
			return
		}
		// Accept numbers or numeric strings in the provided array.
		ids := make([]int, 0, len(msg.MatchIDs))
		for _, v := range msg.MatchIDs {
			switch t := v.(type) {
			case float64:
				ids = append(ids, int(t))
			case string:
				if n, err := strconv.Atoi(t); err == nil {
					ids = append(ids, n)
				}
			}
		}
		for _, id := range ids {
			c.hub.subscribe(id, c)
		}
		c.sendJSON(map[string]any{"type": "subscriptions", "matchIds": ids})

	// Unknown messages are silently ignored — same behaviour as both JS and Python.
	}
}

// Upgrader upgrades an HTTP connection to WebSocket.
// CheckOrigin is intentionally permissive here; CORS is enforced at the HTTP
// layer (same approach as the Express `cors` middleware).
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}