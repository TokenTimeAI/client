package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CursorDetector scans Cursor editor data
type CursorDetector struct {
	dataDir string
}

func init() {
	Register(&CursorDetector{})
}

func (d *CursorDetector) Name() string {
	return "cursor"
}

func (d *CursorDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, "Library", "Application Support", "Cursor"),              // macOS
		filepath.Join(home, ".config", "Cursor"),                                      // Linux
		filepath.Join(home, "AppData", "Roaming", "Cursor"),                          // Windows
		filepath.Join(home, ".cursor"),
	}
}

func (d *CursorDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.dataDir = path
			return true, nil
		}
	}
	return false, nil
}

func (d *CursorDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	// Cursor stores conversations in workspace storage
	workspacesDir := filepath.Join(d.dataDir, "User", "workspaceStorage")
	if !DirExists(workspacesDir) {
		// Try alternative locations
		workspacesDir = filepath.Join(d.dataDir, "workspaces")
	}

	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read workspaces dir: %w", err)
	}

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for _, workspaceEntry := range entries {
		if !workspaceEntry.IsDir() {
			continue
		}

		// Look for cursor conversation files
		cursorDir := filepath.Join(workspacesDir, workspaceEntry.Name(), "cursor")
		if !DirExists(cursorDir) {
			continue
		}

		convPath := filepath.Join(cursorDir, "conversations.json")
		if !FileExists(convPath) {
			continue
		}

		data, err := os.ReadFile(convPath)
		if err != nil {
			continue
		}

		var conversations []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Workspace string `json:"workspace"`
			Created   int64  `json:"created"`
			Modified  int64  `json:"modified"`
			Messages  []struct {
				ID               string  `json:"id"`
				Role             string  `json:"role"`
				Content          string  `json:"content"`
				Timestamp        int64   `json:"timestamp"`
				PromptTokens     int     `json:"prompt_tokens"`
				CompletionTokens int     `json:"completion_tokens"`
				TotalTokens      int     `json:"total_tokens"`
				Model            string  `json:"model"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &conversations); err != nil {
			continue
		}

		for _, conv := range conversations {
			if conv.Modified <= state.LastScanTime {
				continue
			}

			for _, msg := range conv.Messages {
				if msg.Role != "assistant" {
					continue
				}

				result := ScanResult{
					AgentType:        "cursor",
					Type:             "conversation",
					Entity:           conv.Workspace,
					Time:             float64(msg.Timestamp),
					Timestamp:        time.Unix(msg.Timestamp, 0),
					ConversationID:   conv.ID,
					MessageID:        msg.ID,
					PromptTokens:     msg.PromptTokens,
					CompletionTokens: msg.CompletionTokens,
					TotalTokens:      msg.TotalTokens,
					Model:            msg.Model,
					Project:          conv.Workspace,
					Metadata: map[string]any{
						"title": conv.Title,
					},
				}
				results = append(results, result)

				if msg.Timestamp > newState.LastScanTime {
					newState.LastScanTime = msg.Timestamp
					newState.LastRecordID = msg.ID
				}
			}
		}
	}

	return results, newState, nil
}