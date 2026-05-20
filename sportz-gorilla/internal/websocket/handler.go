package websocket

import (
	"log"
	"net/http"
)

// ServeWS upgrades the HTTP request to WebSocket, registers the client with the
// hub, and starts the read/write pumps in goroutines.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] upgrade error: %v", err)
		return
	}

	client := newClient(hub, conn)
	hub.register(client)

	// Send welcome message — mirrors sendJson(socket, { type: 'welcome' }).
	client.sendJSON(map[string]string{"type": "welcome"})

	// Start write pump in a separate goroutine; read pump runs on the current goroutine.
	go client.writePump()
	client.readPump()
}