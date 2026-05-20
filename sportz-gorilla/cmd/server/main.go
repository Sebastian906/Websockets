package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"websockets/internal/config"
	"websockets/internal/database"
	"websockets/internal/routes"
	wsHub "websockets/internal/websocket"
)

func main() {
	cfg := config.Load()

	// Database 
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Database connection established")

	// WebSocket Hub 
	hub := wsHub.NewHub()

	// HTTP Router 
	mux := http.NewServeMux()

	matchesHandler := routes.NewMatchesHandler(pool, hub)
	commentaryHandler := routes.NewCommentaryHandler(pool, hub)

	// Root health-check — mirrors app.get('/')
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "Welcome to Sportz Go API!")
	})

	// REST routes
	// GET /matches and POST /matches
	mux.Handle("/matches", withCORS(cfg.ViteFrontendURL, matchesHandler))
	// GET /matches/{id}/commentary and POST /matches/{id}/commentary
	// Register a prefix handler for all /matches/* paths and let the
	// handler extract the match ID from the URL.
	mux.Handle("/matches/", withCORS(cfg.ViteFrontendURL, commentaryHandler))

	// WebSocket endpoint — mirrors /websocket in both servers
	mux.HandleFunc("/websocket", func(w http.ResponseWriter, r *http.Request) {
		wsHub.ServeWS(hub, w, r)
	})

	// Start server 
	addr := cfg.Addr()
	baseURL := "http://" + addr
	if cfg.Host == "0.0.0.0" {
		baseURL = "http://localhost:" + fmt.Sprint(cfg.Port)
	}

	log.Printf("Server is running on %s", baseURL)
	log.Printf("WebSocket server is running on %s", strings.Replace(baseURL, "http", "ws", 1)+"/websocket")

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// withCORS wraps a handler with CORS headers that only allow the configured
// frontend origin. Mirrors `app.use(cors({ origin: VITE_FRONTEND_URL }))`.
func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}