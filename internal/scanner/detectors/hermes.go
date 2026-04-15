package detectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"

	_ "github.com/mattn/go-sqlite3"
)

// HermesDetector scans Hermes Agent conversation data
type HermesDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewHermesDetector() scanner.Detector {
	paths := []string{
		"~/.hermes",
	}
	return &HermesDetector{
		BaseDetector: scanner.NewBaseDetector("hermes", "Hermes Agent conversations", paths, 50),
	}
}

func (d *HermesDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		expanded, err := scanner.ExpandHome(path)
		if err != nil {
			continue
		}
		if scanner.DirExists(expanded) {
			d.dataDir = expanded
			d.SetFoundPath(expanded)
			return true, nil
		}
	}
	return false, nil
}

type hermesSession struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Source        string  `json:"source"`
	StartedAt     float64 `json:"started_at"`
	EndedAt       float64 `json:"ended_at"`
	MessageCount  int     `json:"message_count"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	Model         string  `json:"model"`
	ParentSession string  `json:"parent_session_id"`
}

func (d *HermesDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	dbPath := filepath.Join(d.dataDir, "state.db")
	if !scanner.FileExists(dbPath) {
		// Try alternative path
		return d.scanJSONL(ctx, state)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, state, fmt.Errorf("open hermes state db: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, title, source, started_at, ended_at, message_count, 
		       input_tokens, output_tokens, model, parent_session_id
		FROM sessions
		WHERE ended_at IS NOT NULL AND (ended_at > ? OR (ended_at = ? AND id > ?))
		ORDER BY ended_at ASC, id ASC
	`, float64(state.LastScanTime), float64(state.LastScanTime), state.LastRecordID)
	if err != nil {
		return nil, state, fmt.Errorf("query hermes sessions: %w", err)
	}
	defer rows.Close()

	var sessions []hermesSession
	for rows.Next() {
		var s hermesSession
		var endedAt sql.NullFloat64
		var title sql.NullString
		var source sql.NullString
		var model sql.NullString
		var parentSession sql.NullString

		if err := rows.Scan(&s.ID, &title, &source, &s.StartedAt, &endedAt,
			&s.MessageCount, &s.InputTokens, &s.OutputTokens, &model, &parentSession); err != nil {
			continue
		}
		if endedAt.Valid {
			s.EndedAt = endedAt.Float64
		}
		s.Title = title.String
		s.Source = source.String
		s.Model = model.String
		s.ParentSession = parentSession.String
		sessions = append(sessions, s)
	}

	var results []scanner.ScanResult
	newState := state

	for _, session := range sessions {
		select {
		case <-ctx.Done():
			return results, newState, ctx.Err()
		default:
		}

		startedAt := time.Unix(int64(session.StartedAt), 0).UTC()
		endedAt := time.Unix(int64(session.EndedAt), 0).UTC()
		sessionSeconds := durationSeconds(startedAt, endedAt)

		// Determine entity - use source as the closest proxy to CWD/project
		entity := session.Source
		if entity == "" {
			entity = session.Title
		}
		if entity == "" {
			entity = "hermes"
		}

		project := session.Source
		if project == "" {
			project = "hermes"
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "hermes",
			Type:                   "conversation",
			Entity:                 entity,
			Time:                   session.EndedAt,
			Timestamp:              endedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(startedAt),
			SessionEndedAt:         timePtr(endedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			ConversationID:         session.ID,
			MessageID:              session.ID,
			PromptTokens:           session.InputTokens,
			CompletionTokens:       session.OutputTokens,
			TotalTokens:            session.InputTokens + session.OutputTokens,
			Model:                  session.Model,
			Project:                project,
			Metadata: map[string]any{
				"title":         session.Title,
				"source":        session.Source,
				"message_count": session.MessageCount,
			},
		})

		newState.LastScanTime = int64(session.EndedAt)
		newState.LastRecordID = session.ID
	}

	return results, newState, nil
}

// scanJSONL scans exported JSONL session files if SQLite is not available
func (d *HermesDetector) scanJSONL(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	// Check for exported sessions directory
	sessionsDir := filepath.Join(d.dataDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read hermes sessions dir: %w", err)
	}

	type jsonlSession struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Source    string `json:"source"`
		StartedAt int64  `json:"started_at"`
		EndedAt   int64  `json:"ended_at"`
		Messages  []struct {
			Role       string `json:"role"`
			TokenCount int    `json:"token_count"`
			Model      string `json:"model"`
		} `json:"messages"`
	}

	var sessions []jsonlSession

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		path := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var session jsonlSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	// Sort by ended_at
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].EndedAt != sessions[j].EndedAt {
			return sessions[i].EndedAt < sessions[j].EndedAt
		}
		return sessions[i].ID < sessions[j].ID
	})

	var results []scanner.ScanResult
	newState := state

	for _, session := range sessions {
		select {
		case <-ctx.Done():
			return results, newState, ctx.Err()
		default:
		}

		if session.EndedAt < state.LastScanTime || (session.EndedAt == state.LastScanTime && session.ID <= state.LastRecordID) {
			continue
		}

		// Calculate token counts from messages (Hermes JSONL uses token_count per message)
		var totalTokens int
		var model string
		for _, msg := range session.Messages {
			totalTokens += msg.TokenCount
			if model == "" && msg.Model != "" {
				model = msg.Model
			}
		}

		startedAt := time.Unix(session.StartedAt, 0).UTC()
		endedAt := time.Unix(session.EndedAt, 0).UTC()
		sessionSeconds := durationSeconds(startedAt, endedAt)

		entity := session.Source
		if entity == "" {
			entity = session.Title
		}
		if entity == "" {
			entity = "hermes"
		}

		project := session.Source
		if project == "" {
			project = "hermes"
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "hermes",
			Type:                   "conversation",
			Entity:                 entity,
			Time:                   float64(session.EndedAt),
			Timestamp:              endedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(startedAt),
			SessionEndedAt:         timePtr(endedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			ConversationID:         session.ID,
			MessageID:              session.ID,
			TotalTokens:            totalTokens,
			Model:                  model,
			Project:                project,
			Metadata: map[string]any{
				"title":  session.Title,
				"source": session.Source,
			},
		})

		newState.LastScanTime = session.EndedAt
		newState.LastRecordID = session.ID
	}

	return results, newState, nil
}

func init() {
	scanner.Register(NewHermesDetector)
}
