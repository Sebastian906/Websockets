package database

import "time"

// MatchStatus mirrors the match_status PG enum.
type MatchStatus string

const (
	StatusScheduled MatchStatus = "scheduled"
	StatusLive      MatchStatus = "live"
	StatusFinished  MatchStatus = "finished"
)

// Match mirrors the `matches` table.
type Match struct {
	ID        int         `json:"id"`
	Sport     string      `json:"sport"`
	HomeTeam  string      `json:"homeTeam"`
	AwayTeam  string      `json:"awayTeam"`
	Status    MatchStatus `json:"status"`
	StartTime *time.Time  `json:"startTime"`
	EndTime   *time.Time  `json:"endTime"`
	HomeScore int         `json:"homeScore"`
	AwayScore int         `json:"awayScore"`
	CreatedAt time.Time   `json:"createdAt"`
}

// Commentary mirrors the `commentary` table.
type Commentary struct {
	ID        int            `json:"id"`
	MatchID   int            `json:"matchId"`
	Minute    *int           `json:"minute"`
	Sequence  *int           `json:"sequence"`
	Period    *string        `json:"period"`
	EventType *string        `json:"eventType"`
	Actor     *string        `json:"actor"`
	Team      *string        `json:"team"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
	Tags      []string       `json:"tags"`
	CreatedAt time.Time      `json:"createdAt"`
}