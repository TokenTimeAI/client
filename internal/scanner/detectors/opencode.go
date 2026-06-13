package detectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

// OpenCodeDetector scans OpenCode conversation data
type OpenCodeDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewOpenCodeDetector() scanner.Detector {
	paths := []string{
		"~/.opencode",
		"~/.config/opencode",
		"~/.local/share/opencode",
	}
	return &OpenCodeDetector{
		BaseDetector: scanner.NewBaseDetector("opencode", "OpenCode CLI conversations", paths, 50),
	}
}

func (d *OpenCodeDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		expanded, err := scanner.ExpandHome(path)
		if err != nil {
			continue
		}
		if scanner.DirExists(expanded) {
			d.configDir = expanded
			d.SetFoundPath(expanded)
			return true, nil
		}
	}
	return false, nil
}

func (d *OpenCodeDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	results, newState, err := d.scanSQLite(ctx, state)
	if err != nil {
		return nil, state, err
	}
	storageResults, storageState, err := d.scanStorage(ctx, state)
	if err != nil {
		return nil, state, err
	}
	results = append(results, storageResults...)
	newState = newerOpenCodeState(newState, storageState)
	legacyResults, legacyState, err := d.scanLegacyJSON(ctx, state)
	if err != nil {
		return nil, state, err
	}
	results = append(results, legacyResults...)
	newState = newerOpenCodeState(newState, legacyState)
	return results, newState, nil
}

func (d *OpenCodeDetector) scanLegacyJSON(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	conversationsDir := filepath.Join(d.configDir, "conversations")
	entries, err := os.ReadDir(conversationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read conversations dir: %w", err)
	}

	var results []scanner.ScanResult
	newState := state

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		convPath := filepath.Join(conversationsDir, entry.Name())
		data, err := os.ReadFile(convPath)
		if err != nil {
			continue
		}

		var conversation struct {
			ID       string `json:"id"`
			Project  string `json:"project"`
			Created  int64  `json:"created"`
			Modified int64  `json:"modified"`
			Messages []struct {
				ID               string  `json:"id"`
				Role             string  `json:"role"`
				Content          string  `json:"content"`
				Timestamp        int64   `json:"timestamp"`
				PromptTokens     int     `json:"prompt_tokens"`
				CompletionTokens int     `json:"completion_tokens"`
				TotalTokens      int     `json:"total_tokens"`
				Model            string  `json:"model"`
				CostUSD          float64 `json:"cost_usd"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &conversation); err != nil {
			continue
		}

		if conversation.Modified <= state.LastScanTime {
			continue
		}

		for _, msg := range conversation.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := scanner.ScanResult{
				AgentType:        "opencode",
				Type:             "conversation",
				Entity:           conversation.Project,
				Time:             float64(msg.Timestamp),
				Timestamp:        time.Unix(msg.Timestamp, 0),
				ConversationID:   conversation.ID,
				MessageID:        msg.ID,
				PromptTokens:     msg.PromptTokens,
				CompletionTokens: msg.CompletionTokens,
				TotalTokens:      msg.TotalTokens,
				Model:            msg.Model,
				CostUSD:          msg.CostUSD,
				Project:          conversation.Project,
			}
			results = append(results, result)

			if msg.Timestamp > newState.LastScanTime {
				newState.LastScanTime = msg.Timestamp
				newState.LastRecordID = msg.ID
			}
		}
	}

	return results, newState, nil
}

func (d *OpenCodeDetector) scanSQLite(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	dbPath := filepath.Join(d.configDir, "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, err
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, state, err
	}
	defer db.Close()

	projects := openCodeSQLiteProjects(db)
	sessions, err := openCodeSQLiteSessions(db)
	if err != nil {
		return nil, state, err
	}

	var results []scanner.ScanResult
	newState := state
	for _, session := range sessions {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}

		worktree := projects[session.ProjectID]
		messages, err := openCodeSQLiteMessages(db, session.ID)
		if err != nil {
			continue
		}
		parts, err := openCodeSQLiteParts(db, session.ID)
		if err != nil {
			continue
		}
		for _, msg := range messages {
			if msg.Role != "assistant" {
				continue
			}
			timestamp := time.UnixMilli(msg.TimeCreated).UTC()
			endUnix := timestamp.Unix()
			if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && msg.ID <= state.LastRecordID) {
				continue
			}

			promptTokens := msg.InputTokens + msg.CacheReadTokens + msg.CacheWriteTokens
			completionTokens := msg.OutputTokens
			fileEdits := make(map[string]scanner.FileEdit)
			for _, part := range parts[msg.ID] {
				mergeFileEdits(fileEdits, openCodeFileEditsFromPart(part.Data))
			}

			result := scanner.ScanResult{
				AgentType:              "opencode",
				Type:                   "conversation",
				Entity:                 worktree,
				Time:                   float64(endUnix),
				Timestamp:              timestamp,
				ConversationID:         session.ID,
				MessageID:              msg.ID,
				PromptTokens:           promptTokens,
				CompletionTokens:       completionTokens,
				TotalTokens:            promptTokens + completionTokens,
				Model:                  msg.Model,
				FileEdits:              flattenFileEdits(fileEdits),
				Project:                projectNameFromPath(worktree),
				SessionStartedAt:       timePtr(time.UnixMilli(session.TimeCreated).UTC()),
				SessionEndedAt:         timePtr(time.UnixMilli(session.TimeUpdated).UTC()),
				SessionDurationSeconds: intPtr(durationSeconds(time.UnixMilli(session.TimeCreated).UTC(), time.UnixMilli(session.TimeUpdated).UTC())),
				Metadata: map[string]any{
					"parser": "opencode_sqlite",
				},
			}
			results = append(results, result)

			if endUnix > newState.LastScanTime || (endUnix == newState.LastScanTime && msg.ID > newState.LastRecordID) {
				newState.LastScanTime = endUnix
				newState.LastRecordID = msg.ID
			}
		}
	}
	return results, newState, nil
}

type openCodeSQLiteSession struct {
	ID          string
	ProjectID   string
	TimeCreated int64
	TimeUpdated int64
}

type openCodeSQLiteMessage struct {
	ID               string
	Role             string
	Model            string
	TimeCreated      int64
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
}

type openCodeSQLitePart struct {
	MessageID string
	Data      string
}

func openCodeSQLiteProjects(db *sql.DB) map[string]string {
	rows, err := db.Query(`SELECT id, COALESCE(worktree, '') FROM project`)
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()

	projects := make(map[string]string)
	for rows.Next() {
		var id, worktree string
		if err := rows.Scan(&id, &worktree); err == nil {
			projects[id] = strings.TrimSpace(worktree)
		}
	}
	return projects
}

func openCodeSQLiteSessions(db *sql.DB) ([]openCodeSQLiteSession, error) {
	rows, err := db.Query(`
		SELECT id,
		       project_id,
		       time_created,
		       time_updated
		FROM session
		ORDER BY time_created, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []openCodeSQLiteSession
	for rows.Next() {
		var session openCodeSQLiteSession
		if err := rows.Scan(&session.ID, &session.ProjectID, &session.TimeCreated, &session.TimeUpdated); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func openCodeSQLiteMessages(db *sql.DB, sessionID string) ([]openCodeSQLiteMessage, error) {
	rows, err := db.Query(`
		SELECT id,
		       COALESCE(data, '{}'),
		       time_created
		FROM message
		WHERE session_id = ?
		ORDER BY time_created, id
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []openCodeSQLiteMessage
	for rows.Next() {
		var id, data string
		var timeCreated int64
		if err := rows.Scan(&id, &data, &timeCreated); err != nil {
			return nil, err
		}
		message := openCodeSQLiteMessage{ID: id, TimeCreated: timeCreated}
		applyOpenCodeSQLiteMessageData(&message, data)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func openCodeSQLiteParts(db *sql.DB, sessionID string) (map[string][]openCodeSQLitePart, error) {
	rows, err := db.Query(`
		SELECT message_id,
		       COALESCE(data, '{}')
		FROM part
		WHERE session_id = ?
		ORDER BY time_created, id
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parts := make(map[string][]openCodeSQLitePart)
	for rows.Next() {
		var part openCodeSQLitePart
		if err := rows.Scan(&part.MessageID, &part.Data); err != nil {
			return nil, err
		}
		parts[part.MessageID] = append(parts[part.MessageID], part)
	}
	return parts, rows.Err()
}

func applyOpenCodeSQLiteMessageData(message *openCodeSQLiteMessage, raw string) {
	var body struct {
		Role    string `json:"role"`
		ModelID string `json:"modelID"`
		Model   struct {
			ModelID string `json:"modelID"`
		} `json:"model"`
		Tokens struct {
			Input  int `json:"input"`
			Output int `json:"output"`
			Cache  struct {
				Read  int `json:"read"`
				Write int `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return
	}
	message.Role = strings.TrimSpace(body.Role)
	message.Model = firstNonEmptyString(body.ModelID, body.Model.ModelID)
	message.InputTokens = body.Tokens.Input
	message.OutputTokens = body.Tokens.Output
	message.CacheReadTokens = body.Tokens.Cache.Read
	message.CacheWriteTokens = body.Tokens.Cache.Write
}

func openCodeFileEditsFromPart(raw string) map[string]scanner.FileEdit {
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil
	}
	if stringValue(body["type"]) != "tool" {
		return nil
	}
	toolName := firstNonEmptyString(stringValue(body["tool"]), stringValue(body["toolName"]), stringValue(body["name"]))
	state, ok := body["state"].(map[string]any)
	if !ok {
		return nil
	}
	input, ok := state["input"].(map[string]any)
	if !ok {
		return nil
	}
	return fileEditsFromToolCall(toolName, input)
}

func init() {
	scanner.Register(NewOpenCodeDetector)
}
