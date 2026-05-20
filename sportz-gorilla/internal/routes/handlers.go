package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"websockets/internal/database"
	"websockets/internal/utils"
	"websockets/internal/validation"
	ws "websockets/internal/websocket"
)

const maxMatchLimit = 100

// MatchesHandler holds the dependencies for the /matches routes.
type MatchesHandler struct {
	db  *pgxpool.Pool
	hub *ws.Hub
}

// NewMatchesHandler constructs a MatchesHandler.
func NewMatchesHandler(db *pgxpool.Pool, hub *ws.Hub) *MatchesHandler {
	return &MatchesHandler{db: db, hub: hub}
}

// ServeHTTP dispatches GET / POST based on the request method.
func (h *MatchesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listMatches(w, r)
	case http.MethodPost:
		h.createMatch(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// listMatches — GET /matches
// Mirrors the GET / handler in matches.js and list_matches() in matches.py.
func (h *MatchesHandler) listMatches(w http.ResponseWriter, r *http.Request) {
	q, err := validation.ParseListMatchesQuery(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	limit := q.Limit
	if limit > maxMatchLimit {
		limit = maxMatchLimit
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, sport, home_team, away_team, status,
		       start_time, end_time, home_score, away_score, created_at
		FROM matches
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to list matches"})
		return
	}
	defer rows.Close()

	matches := make([]database.Match, 0)
	for rows.Next() {
		var m database.Match
		var statusStr string
		if err := rows.Scan(
			&m.ID, &m.Sport, &m.HomeTeam, &m.AwayTeam, &statusStr,
			&m.StartTime, &m.EndTime, &m.HomeScore, &m.AwayScore, &m.CreatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to scan match"})
			return
		}
		m.Status = database.MatchStatus(statusStr)
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Row iteration error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": matches})
}

// createMatch — POST /matches
// Mirrors the POST / handler in matches.js and create_match() in matches.py.
func (h *MatchesHandler) createMatch(w http.ResponseWriter, r *http.Request) {
	var body validation.CreateMatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload"})
		return
	}
	if err := body.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	homeScore := 0
	if body.HomeScore != nil {
		homeScore = *body.HomeScore
	}
	awayScore := 0
	if body.AwayScore != nil {
		awayScore = *body.AwayScore
	}

	status := utils.GetMatchStatus(&body.StartTime, &body.EndTime)

	var m database.Match
	var statusStr string
	err := h.db.QueryRow(r.Context(), `
		INSERT INTO matches (sport, home_team, away_team, status, start_time, end_time, home_score, away_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, sport, home_team, away_team, status,
		          start_time, end_time, home_score, away_score, created_at
	`,
		body.Sport, body.HomeTeam, body.AwayTeam, string(status),
		body.StartTime, body.EndTime, homeScore, awayScore,
	).Scan(
		&m.ID, &m.Sport, &m.HomeTeam, &m.AwayTeam, &statusStr,
		&m.StartTime, &m.EndTime, &m.HomeScore, &m.AwayScore, &m.CreatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "Failed to create match",
			"details": err.Error(),
		})
		return
	}
	m.Status = database.MatchStatus(statusStr)

	h.hub.BroadcastMatchCreated(m)
	writeJSON(w, http.StatusCreated, map[string]any{"data": m})
}

// CommentaryHandler holds the dependencies for the /matches/{id}/commentary routes.
type CommentaryHandler struct {
	db  *pgxpool.Pool
	hub *ws.Hub
}

// NewCommentaryHandler constructs a CommentaryHandler.
func NewCommentaryHandler(db *pgxpool.Pool, hub *ws.Hub) *CommentaryHandler {
	return &CommentaryHandler{db: db, hub: hub}
}

// ServeHTTP dispatches GET / POST and extracts the match ID from the URL.
// The path is expected to be /matches/{id}/commentary.
func (h *CommentaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	matchID, err := extractMatchID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid match ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listCommentary(w, r, matchID)
	case http.MethodPost:
		h.createCommentary(w, r, matchID)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// listCommentary — GET /matches/{id}/commentary
func (h *CommentaryHandler) listCommentary(w http.ResponseWriter, r *http.Request, matchID int) {
	q, err := validation.ParseListCommentaryQuery(r.URL.Query().Get("limit"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	limit := q.Limit
	if limit > 100 {
		limit = 100
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, match_id, minute, sequence, period, event_type,
		       actor, team, message, metadata, tags, created_at
		FROM commentary
		WHERE match_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, matchID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch commentary"})
		return
	}
	defer rows.Close()

	entries := make([]database.Commentary, 0)
	for rows.Next() {
		c, err := scanCommentary(rows)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to scan commentary"})
			return
		}
		entries = append(entries, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Row iteration error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": entries})
}

// createCommentary — POST /matches/{id}/commentary
func (h *CommentaryHandler) createCommentary(w http.ResponseWriter, r *http.Request, matchID int) {
	var body validation.CreateCommentaryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload"})
		return
	}
	if err := body.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var metadataJSON []byte
	if body.Metadata != nil {
		var jsonErr error
		metadataJSON, jsonErr = json.Marshal(body.Metadata)
		if jsonErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid metadata"})
			return
		}
	}

	row := h.db.QueryRow(r.Context(), `
		INSERT INTO commentary
		  (match_id, minute, sequence, period, event_type, actor, team, message, metadata, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10)
		RETURNING id, match_id, minute, sequence, period, event_type,
		          actor, team, message, metadata, tags, created_at
	`,
		matchID,
		body.Minute,
		body.Sequence,
		body.Period,
		body.EventType,
		body.Actor,
		body.Team,
		body.Message,
		nullableBytes(metadataJSON),
		body.Tags,
	)

	entry, err := scanCommentaryRow(row)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create commentary"})
		return
	}

	h.hub.BroadcastCommentary(matchID, entry)
	writeJSON(w, http.StatusCreated, map[string]any{"data": entry})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// extractMatchID reads the match ID from the URL path.
// The path pattern is /matches/{id}/commentary.
func extractMatchID(r *http.Request) (int, error) {
	// Expected path: /matches/{id}/commentary
	// Split the path and extract the ID segment.
	parts := strings.Split(r.URL.Path, "/")
	// parts[0] == "", parts[1] == "matches", parts[2] == id, parts[3] == "commentary"
	if len(parts) < 3 || parts[1] != "matches" {
		return 0, fmt.Errorf("missing match id")
	}
	idStr := parts[2]
	if idStr == "" {
		return 0, fmt.Errorf("missing match id")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid match id")
	}
	return id, nil
}

// nullableBytes returns nil if b is empty, otherwise b.
// Used to avoid sending an empty JSON string to a JSONB column.
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// scanCommentary scans a pgx.Rows cursor into a Commentary struct.
func scanCommentary(rows interface{ Scan(...any) error }) (database.Commentary, error) {
	return scanCommentaryFrom(rows)
}

// scanCommentaryRow scans a single pgx.Row into a Commentary struct.
func scanCommentaryRow(row interface{ Scan(...any) error }) (database.Commentary, error) {
	return scanCommentaryFrom(row)
}

func scanCommentaryFrom(s interface{ Scan(...any) error }) (database.Commentary, error) {
	var c database.Commentary
	var rawMeta []byte

	if err := s.Scan(
		&c.ID, &c.MatchID, &c.Minute, &c.Sequence, &c.Period, &c.EventType,
		&c.Actor, &c.Team, &c.Message, &rawMeta, &c.Tags, &c.CreatedAt,
	); err != nil {
		return c, err
	}
	if rawMeta != nil {
		_ = json.Unmarshal(rawMeta, &c.Metadata)
	}
	return c, nil
}