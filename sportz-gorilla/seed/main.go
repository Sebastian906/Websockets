
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	delayMS              int
	matchDurationMinutes int
	forceLive            bool
	apiURL               string
)

func init() {
	_ = godotenv.Load()

	delayMS = intEnv("DELAY_MS", 250)
	matchDurationMinutes = intEnv("SEED_MATCH_DURATION_MINUTES", 120)
	forceLive = boolEnv("SEED_FORCE_LIVE", true)
	apiURL = os.Getenv("API_URL")
	if apiURL == "" {
		log.Fatal("API_URL is required to seed via REST endpoints.")
	}
}

func intEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func boolEnv(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v != "0" && strings.ToLower(v) != "false"
}

type Match struct {
	ID        int        `json:"id"`
	Sport     string     `json:"sport"`
	HomeTeam  string     `json:"homeTeam"`
	AwayTeam  string     `json:"awayTeam"`
	Status    string     `json:"status"`
	StartTime *time.Time `json:"startTime"`
	EndTime   *time.Time `json:"endTime"`
	HomeScore int        `json:"homeScore"`
	AwayScore int        `json:"awayScore"`
}

type SeedMatch struct {
	ID        *int       `json:"id"`
	Sport     string     `json:"sport"`
	HomeTeam  string     `json:"homeTeam"`
	AwayTeam  string     `json:"awayTeam"`
	StartTime *time.Time `json:"startTime"`
	EndTime   *time.Time `json:"endTime"`
	HomeScore int        `json:"homeScore"`
	AwayScore int        `json:"awayScore"`
}

type FeedEntry struct {
	MatchID   *int           `json:"matchId"`
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

func fetchWithRetry(ctx context.Context, method, url string, body any, attempts int) (*http.Response, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error

	for i := 0; i < attempts; i++ {
		var reqBody io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reqBody = bytes.NewReader(data)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		} else {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			_ = resp.Body.Close()
		}

		if i < attempts-1 {
			delay := time.Duration(math.Min(5000, float64(200)*math.Pow(2, float64(i))))*time.Millisecond +
				time.Duration(rand.Intn(100))*time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, lastErr
}

func fetchMatches(ctx context.Context) ([]Match, error) {
	resp, err := fetchWithRetry(ctx, "GET", apiURL+"/matches?limit=100", nil, 5)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Data []Match `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func createMatch(ctx context.Context, sm SeedMatch) (*Match, error) {
	startTime, endTime := buildMatchTimes(sm)
	body := map[string]any{
		"sport":     sm.Sport,
		"homeTeam":  sm.HomeTeam,
		"awayTeam":  sm.AwayTeam,
		"startTime": startTime.Format(time.RFC3339),
		"endTime":   endTime.Format(time.RFC3339),
		"homeScore": sm.HomeScore,
		"awayScore": sm.AwayScore,
	}
	resp, err := fetchWithRetry(ctx, "POST", apiURL+"/matches", body, 5)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Data Match `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload.Data, nil
}

func insertCommentary(ctx context.Context, matchID int, entry FeedEntry) (*FeedEntry, error) {
	body := map[string]any{"message": entry.Message}
	if entry.Minute != nil {
		body["minute"] = *entry.Minute
	}
	if entry.Sequence != nil {
		body["sequence"] = *entry.Sequence
	}
	if entry.Period != nil {
		body["period"] = *entry.Period
	}
	if entry.EventType != nil {
		body["eventType"] = *entry.EventType
	}
	if entry.Actor != nil {
		body["actor"] = *entry.Actor
	}
	if entry.Team != nil {
		body["team"] = *entry.Team
	}
	if entry.Metadata != nil {
		body["metadata"] = entry.Metadata
	}
	if entry.Tags != nil {
		body["tags"] = entry.Tags
	}

	url := fmt.Sprintf("%s/matches/%d/commentary", apiURL, matchID)
	resp, err := fetchWithRetry(ctx, "POST", url, body, 5)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Data FeedEntry `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload.Data, nil
}

func isLiveMatch(m Match) bool {
	if m.StartTime == nil || m.EndTime == nil {
		return false
	}
	now := time.Now().UTC()
	return !now.Before(*m.StartTime) && now.Before(*m.EndTime)
}

func buildMatchTimes(sm SeedMatch) (time.Time, time.Time) {
	now := time.Now().UTC()
	duration := time.Duration(matchDurationMinutes) * time.Minute

	start := sm.StartTime
	end := sm.EndTime

	if start == nil && end == nil {
		s := now.Add(-5 * time.Minute)
		e := s.Add(duration)
		start, end = &s, &e
	} else if start != nil && end == nil {
		e := start.Add(duration)
		end = &e
	} else if start == nil && end != nil {
		s := end.Add(-duration)
		start = &s
	}

	if forceLive && start != nil && end != nil {
		if now.Before(*start) || !now.Before(*end) {
			s := now.Add(-5 * time.Minute)
			e := s.Add(duration)
			start, end = &s, &e
		}
	}

	return *start, *end
}

func loadSeedData() ([]FeedEntry, []SeedMatch, error) {
	// Resolve path relative to this file (mirrors DATA_FILE in Python seeder).
	_, file, _, _ := runtime.Caller(0)
	dataPath := filepath.Join(filepath.Dir(file), "..", "src", "data", "data.json")

	// Fallback locations: first try the repo-level `data/data.json`,
	// then a `data.json` next to the binary (current working dir).
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		alt := filepath.Join(filepath.Dir(file), "..", "data", "data.json")
		if _, err2 := os.Stat(alt); err2 == nil {
			dataPath = alt
		} else {
			dataPath = "data.json"
		}
	}

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read seed data: %w", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// Try as a plain array.
		var feed []FeedEntry
		if err2 := json.Unmarshal(raw, &feed); err2 != nil {
			return nil, nil, errors.New("seed data must be an array or contain a commentary/feed array")
		}
		return feed, nil, nil
	}

	var feed []FeedEntry
	if v, ok := parsed["commentary"]; ok {
		_ = json.Unmarshal(v, &feed)
	} else if v, ok := parsed["feed"]; ok {
		_ = json.Unmarshal(v, &feed)
	} else {
		return nil, nil, errors.New("seed data must contain a commentary or feed key")
	}

	var seedMatches []SeedMatch
	if v, ok := parsed["matches"]; ok {
		_ = json.Unmarshal(v, &seedMatches)
	}

	return feed, seedMatches, nil
}

type matchState struct {
	match     Match
	fakeNext  string
}

func main() {
	log.Printf("Seeding via API: %s", apiURL)

	ctx := context.Background()

	feed, seedMatches, err := loadSeedData()
	if err != nil {
		log.Fatalf("Failed to load seed data: %v", err)
	}

	existingMatches, err := fetchMatches(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch existing matches: %v", err)
	}

	matchMap := make(map[int]*matchState)     // id → state
	matchKeyMap := make(map[string]*Match)    // "sport|home|away" → match

	for i := range existingMatches {
		m := &existingMatches[i]
		if forceLive && !isLiveMatch(*m) {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s", m.Sport, m.HomeTeam, m.AwayTeam)
		if _, exists := matchKeyMap[key]; !exists {
			matchKeyMap[key] = m
		}
		matchMap[m.ID] = &matchState{match: *m, fakeNext: randomSide()}
	}

	for _, sm := range seedMatches {
		key := fmt.Sprintf("%s|%s|%s", sm.Sport, sm.HomeTeam, sm.AwayTeam)
		match := matchKeyMap[key]
		if match == nil || (forceLive && !isLiveMatch(*match)) {
			created, err := createMatch(ctx, sm)
			if err != nil {
				log.Fatalf("Failed to create match %s: %v", key, err)
			}
			match = created
			matchKeyMap[key] = match
			delay := 2000 + rand.Intn(1001)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		if sm.ID != nil {
			matchMap[*sm.ID] = &matchState{match: *match, fakeNext: randomSide()}
		}
		matchMap[match.ID] = &matchState{match: *match, fakeNext: randomSide()}
	}

	if len(matchMap) == 0 {
		log.Fatal("No matches found or created in the database.")
	}

	for _, entry := range feed {
		if entry.MatchID == nil {
			log.Printf("Skipping entry — matchId missing: %s", entry.Message)
			continue
		}
		state := matchMap[*entry.MatchID]
		if state == nil {
			log.Printf("Skipping entry — matchId %d not found: %s", *entry.MatchID, entry.Message)
			continue
		}

		row, err := insertCommentary(ctx, state.match.ID, entry)
		if err != nil {
			log.Printf("[Match %d] Failed to insert commentary: %v", state.match.ID, err)
			continue
		}
		log.Printf("[Match %d] %s", state.match.ID, row.Message)

		if delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}
	}
}

func randomSide() string {
	if rand.Float64() < 0.5 {
		return "home"
	}
	return "away"
}