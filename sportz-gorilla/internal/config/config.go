package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Settings mirrors the environment variables used by sportz-express and sportz-fastapi.
type Settings struct {
	DatabaseURL     string
	Port            int
	Host            string
	ViteFrontendURL string
	ArcjetKey       string
	ArcjetMode      string
}

// Load reads the .env file (if present) and populates Settings.
// Panics if DATABASE_URL is missing.
func Load() *Settings {
	// godotenv.Load is a no-op if the file is missing; real deployments use
	// environment variables directly.
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		panic("DATABASE_URL is not set in the environment")
	}

	port := 8000
	if raw := os.Getenv("PORT"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil {
			port = p
		}
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	frontendURL := os.Getenv("VITE_FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	arcjetMode := os.Getenv("ARCJET_MODE")
	if arcjetMode == "" {
		arcjetMode = "DRY_RUN"
	}

	return &Settings{
		DatabaseURL:     dbURL,
		Port:            port,
		Host:            host,
		ViteFrontendURL: frontendURL,
		ArcjetKey:       os.Getenv("ARCJET_KEY"),
		ArcjetMode:      arcjetMode,
	}
}

// Addr returns the listening address string (e.g. "0.0.0.0:8000").
func (s *Settings) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}