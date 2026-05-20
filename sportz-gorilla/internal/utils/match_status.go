package utils

import (
	"time"

	"websockets/internal/database"
)

// GetMatchStatus computes the status of a match based on its start/end times.
// Mirrors getMatchStatus() in match-status.js and get_match_status() in match_status.py.
func GetMatchStatus(startTime, endTime *time.Time) database.MatchStatus {
	if startTime == nil || endTime == nil {
		return database.StatusScheduled
	}

	now := time.Now().UTC()
	start := startTime.UTC()
	end := endTime.UTC()

	if now.Before(start) {
		return database.StatusScheduled
	}
	if now.Before(end) {
		return database.StatusLive
	}
	return database.StatusFinished
}