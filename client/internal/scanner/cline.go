package scanner

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ClineDetector scans Cline's SQLite database
type ClineDetector struct {
	globalStorageDir string
}

func init() {
	Register(&ClineDetector{})
}

func (d *ClineDetector) Name() string {
	return "cline"
}

func (d *ClineDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	// Cline stores data in VS Code extension storage
	return []string{
		filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev"), // macOS
		filepath.Join(home, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev"),                         // Linux
		filepath.Join(home, "AppData", "Roaming", "Code", "User", "globalStorage", "saoudrizwan.claude-dev"),            // Windows
	}
}

func (d *ClineDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.globalStorageDir = path
			return true, nil
		}
	}
	return false, nil
}

func (d *ClineDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.globalStorageDir == "" {
		return nil, state, nil
	}

	// Cline uses SQLite for conversation history
	dbPath := filepath.Join(d.globalStorageDir, "cline.db")
	if !FileExists(dbPath) {
		// Fallback to task history directory
		return d.scanTaskHistory(ctx, state)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, state, fmt.Errorf("open cline db: %w", err)
	}
	defer db.Close()

	// Query for new conversations/messages since last scan
	query := `
		SELECT 
			c.id as conversation_id,
			m.id as message_id,
			c.workspace_path,
			m.role,
			m.content,
			m.tokens_input,
			m.tokens_output,
			m.model,
			m.timestamp,
			c.task_id
		FROM messages m
		JOIN conversations c ON m.conversation_id = c.id
		WHERE m.timestamp > ? OR (m.timestamp = ? AND m.id > ?)
		ORDER BY m.timestamp, m.id
	`

	rows, err := db.QueryContext(ctx, query, state.LastScanTime, state.LastScanTime, state.LastRecordID)
	if err != nil {
		return nil, state, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for rows.Next() {
		var convID, msgID, workspacePath, role, content, model, taskID string
		var tokensIn, tokensOut int
		var timestamp int64

		if err := rows.Scan(&convID, &msgID, &workspacePath, &role, &content, &tokensIn, &tokensOut, &model, &timestamp, &taskID); err != nil {
			continue
		}

		// Only track assistant messages for token counts
		if role != "assistant" {
			continue
		}

		result := ScanResult{
			AgentType:        "cline",
			Type:             "conversation",
			Entity:           workspacePath,
			Time:             float64(timestamp),
			Timestamp:        time.Unix(timestamp, 0),
			ConversationID:   convID,
			MessageID:        msgID,
			PromptTokens:     tokensIn,
			CompletionTokens: tokensOut,
			TotalTokens:      tokensIn + tokensOut,
			Model:            model,
			Project:          workspacePath,
			Metadata: map[string]any{
				"task_id": taskID,
				"role":    role,
			},
		}
		results = append(results, result)

		// Update state
		if timestamp > newState.LastScanTime {
			newState.LastScanTime = timestamp
			newState.LastRecordID = msgID
		}
	}

	return results, newState, rows.Err()
}

// scanTaskHistory scans Cline's JSON task history if SQLite is not available
func (d *ClineDetector) scanTaskHistory(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	tasksDir := filepath.Join(d.globalStorageDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, state, nil // No tasks dir is OK
	}

	var results []ScanResult
	newState := state

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Each task has a task.json with conversation data
		taskPath := filepath.Join(tasksDir, entry.Name(), "task.json")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			continue
		}

		var task struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Messages []struct {
				ID               string `json:"id"`
				Role             string `json:"role"`
				Content          string `json:"content"`
				Timestamp        int64  `json:"ts"`
				TokensInput      int    `json:"tokens_input"`
				TokensOutput     int    `json:"tokens_output"`
				Model            string `json:"model"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		for _, msg := range task.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := ScanResult{
				AgentType:        "cline",
				Type:             "conversation",
				Entity:           task.Path,
				Time:             float64(msg.Timestamp),
				Timestamp:        time.Unix(msg.Timestamp, 0),
				ConversationID:   task.ID,
				MessageID:        msg.ID,
				PromptTokens:     msg.TokensInput,
				CompletionTokens: msg.TokensOutput,
				TotalTokens:      msg.TokensInput + msg.TokensOutput,
				Model:            msg.Model,
				Project:          task.Path,
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