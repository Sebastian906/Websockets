package validation

import (
	"errors"
	"fmt"
	"time"
)

// CreateMatchBody mirrors createMatchSchema (Zod) / CreateMatchBody (Pydantic).
type CreateMatchBody struct {
	Sport     string    `json:"sport"`
	HomeTeam  string    `json:"homeTeam"`
	AwayTeam  string    `json:"awayTeam"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	HomeScore *int      `json:"homeScore"`
	AwayScore *int      `json:"awayScore"`
}

// Validate performs the same checks as the Zod superRefine / Pydantic model_validator.
func (b *CreateMatchBody) Validate() error {
	if b.Sport == "" {
		return errors.New("sport must not be empty")
	}
	if b.HomeTeam == "" {
		return errors.New("homeTeam must not be empty")
	}
	if b.AwayTeam == "" {
		return errors.New("awayTeam must not be empty")
	}
	if b.StartTime.IsZero() {
		return errors.New("startTime is required")
	}
	if b.EndTime.IsZero() {
		return errors.New("endTime is required")
	}
	if !b.EndTime.After(b.StartTime) {
		return errors.New("endTime must be chronologically after startTime")
	}
	if b.HomeScore != nil && *b.HomeScore < 0 {
		return fmt.Errorf("homeScore must be >= 0")
	}
	if b.AwayScore != nil && *b.AwayScore < 0 {
		return fmt.Errorf("awayScore must be >= 0")
	}
	return nil
}

// ListMatchesQuery mirrors listMatchesQuerySchema (Zod) / ListMatchesQuery (Pydantic).
type ListMatchesQuery struct {
	Limit int
}

// ParseListMatchesQuery parses and validates the `limit` query param.
func ParseListMatchesQuery(rawLimit string) (ListMatchesQuery, error) {
	limit := 50
	if rawLimit != "" {
		n := 0
		if _, err := fmt.Sscanf(rawLimit, "%d", &n); err != nil || n < 1 || n > 100 {
			return ListMatchesQuery{}, errors.New("limit must be an integer between 1 and 100")
		}
		limit = n
	}
	return ListMatchesQuery{Limit: limit}, nil
}