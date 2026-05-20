package validation

import (
	"errors"
	"fmt"
)

// CreateCommentaryBody mirrors createCommentarySchema (Zod) / CreateCommentaryBody (Pydantic).
type CreateCommentaryBody struct {
	Minute    *int           `json:"minute"`
	Sequence  *int           `json:"sequence"`
	Period    *string        `json:"period"`
	EventType *string        `json:"eventType"`
	Actor     *string        `json:"actor"`
	Team      *string        `json:"team"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
	Tags      []string       `json:"tags"`
}

// Validate checks required fields and constraints.
func (b *CreateCommentaryBody) Validate() error {
	if b.Message == "" {
		return errors.New("message must not be empty")
	}
	if b.Minute != nil && *b.Minute < 0 {
		return fmt.Errorf("minute must be >= 0")
	}
	return nil
}

// ListCommentaryQuery mirrors listCommentaryQuerySchema.
type ListCommentaryQuery struct {
	Limit int
}

// ParseListCommentaryQuery parses and validates the `limit` query param.
func ParseListCommentaryQuery(rawLimit string) (ListCommentaryQuery, error) {
	limit := 10
	if rawLimit != "" {
		n := 0
		if _, err := fmt.Sscanf(rawLimit, "%d", &n); err != nil || n < 1 || n > 100 {
			return ListCommentaryQuery{}, errors.New("limit must be an integer between 1 and 100")
		}
		limit = n
	}
	return ListCommentaryQuery{Limit: limit}, nil
}