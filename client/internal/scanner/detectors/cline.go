package detectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"

	_ "github.com/mattn/go-sqlite3"
)

// ClineDetector scans Cline VS Code extension data
type ClineDetector struct {
	scanner.BaseDetector
	globalStorageDir string
}

func NewClineDetector() scanner.Detector {
	paths := []string{
		"~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev",
		"~/.config/Code/User/globalStorage/saoudrizwan.claude-dev",
		"~/AppData/Roaming/Code/User/globalStorage/saoudrizwan.claude-dev",
	}
	return &ClineDetector{
		BaseDetector: scanner.NewBaseDetector("cline", "Cline VS Code extension conversations", paths, 50),
	}
}

func (d *ClineDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		expanded, err := scanner.ExpandHome(path)
		if err != nil {
			continue
		}
		if scanner.DirExists(expanded) {
			d.globalStorageDir = expanded
			d.SetFoundPath(expanded)
			return true, nil
		}
	}
	return false, nil
}

func (d *ClineDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.globalStorageDir == "" {
		return nil, state, nil
	}

	dbPath := filepath.Join(d.globalStorageDir, "cline.db")
	if scanner.FileExists(dbPath) {
		return d.scanSQLite(ctx, state, dbPath)
	}
	return d.scanTaskHistory(ctx, state)
}

func (d *ClineDetector) scanSQLite(ctx context.Context, state scanner.SourceState, dbPath string) ([]scanner.ScanResult, scanner.SourceState, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, state, fmt.Errorf("open cline db: %w", err)
	}
	defer db.Close()

	query := `
		SELECT c.id as conversation_id, m.id as message_id, c.workspace_path,
			m.role, m.content, m.tokens_input, m.tokens_output, m.model, m.timestamp
		FROM messages m
		JOIN conversations c ON m.conversation_id = c.id
		WHERE m.timestamp > ? OR (m.timestamp = ? AND m.id > ?)
		ORDER BY m.timestamp, m.id`

	rows, err := db.QueryContext(ctx, query, state.LastScanTime, state.LastScanTime, state.LastRecordID)
	if err != nil {
		return nil, state, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var results []scanner.ScanResult
	newState := state

	for rows.Next() {
		var convID, msgID, workspacePath, role, content, model string
		var tokensIn, tokensOut int
		var timestamp int64

		if err := rows.Scan(&convID, &msgID, &workspacePath, &role, &content, &tokensIn, &tokensOut, &model, &timestamp); err != nil {
			continue
		}

		if role != "assistant" {
			continue
		}

		result := scanner.ScanResult{
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
			Metadata:         map[string]any{"role": role},
		}
		results = append(results, result)

		if timestamp > newState.LastScanTime {
			newState.LastScanTime = timestamp
			newState.LastRecordID = msgID
		}
	}

	return results, newState, rows.Err()
}

func (d *ClineDetector) scanTaskHistory(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	tasksDir := filepath.Join(d.globalStorageDir, "tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, state, nil
	}

	var results []scanner.ScanResult
	newState := state

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskPath := filepath.Join(tasksDir, entry.Name(), "task.json")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			continue
		}

		var task struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Messages []struct {
				ID           string `json:"id"`
				Role         string `json:"role"`
				Timestamp    int64  `json:"ts"`
				TokensInput  int    `json:"tokens_input"`
				TokensOutput int    `json:"tokens_output"`
				Model        string `json:"model"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}

		for _, msg := range task.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := scanner.ScanResult{
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

func init() {
	scanner.Register(NewClineDetector)
}
