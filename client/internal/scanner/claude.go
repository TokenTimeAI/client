package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeDetector scans Claude Code conversation data
type ClaudeDetector struct {
	dataDir string
}

func init() {
	Register(&ClaudeDetector{})
}

func (d *ClaudeDetector) Name() string {
	return "claude_code"
}

func (d *ClaudeDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".config", "claude"),
		filepath.Join(home, "Library", "Application Support", "Claude"), // macOS
	}
}

func (d *ClaudeDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.dataDir = path
			return true, nil
		}
	}
	return false, nil
}

func (d *ClaudeDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	// Claude stores projects with conversation history
	projectsDir := filepath.Join(d.dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read projects dir: %w", err)
	}

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for _, projectEntry := range entries {
		if !projectEntry.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, projectEntry.Name())

		// Look for chat history files
		historyFiles := []string{
			filepath.Join(projectPath, "chat_history.json"),
			filepath.Join(projectPath, "conversations.json"),
		}

		for _, historyPath := range historyFiles {
			if !FileExists(historyPath) {
				continue
			}

			data, err := os.ReadFile(historyPath)
			if err != nil {
				continue
			}

			var history struct {
				Sessions []struct {
					ID       string `json:"id"`
					Project  string `json:"project"`
					Path     string `json:"path"`
					Started  int64  `json:"started"`
					Modified int64  `json:"modified"`
					Messages []struct {
						ID               string  `json:"id"`
						Role             string  `json:"role"`
						Content          string  `json:"content"`
						Timestamp        int64   `json:"timestamp"`
						PromptTokens     int     `json:"input_tokens"`
						CompletionTokens int     `json:"output_tokens"`
						Model            string  `json:"model"`
						CostUSD          float64 `json:"cost_usd"`
					} `json:"messages"`
				} `json:"sessions"`
			}

			if err := json.Unmarshal(data, &history); err != nil {
				continue
			}

			for _, session := range history.Sessions {
				if session.Modified <= state.LastScanTime {
					continue
				}

				for _, msg := range session.Messages {
					// Claude tracks both, but we want assistant responses
					if msg.Role != "assistant" {
						continue
					}

					result := ScanResult{
						AgentType:        "claude_code",
						Type:             "conversation",
						Entity:           session.Path,
						Time:             float64(msg.Timestamp),
						Timestamp:        time.Unix(msg.Timestamp, 0),
						ConversationID:   session.ID,
						MessageID:        msg.ID,
						PromptTokens:     msg.PromptTokens,
						CompletionTokens: msg.CompletionTokens,
						TotalTokens:      msg.PromptTokens + msg.CompletionTokens,
						Model:            msg.Model,
						CostUSD:          msg.CostUSD,
						Project:          session.Project,
					}
					results = append(results, result)

					if msg.Timestamp > newState.LastScanTime {
						newState.LastScanTime = msg.Timestamp
						newState.LastRecordID = msg.ID
					}
				}
			}
		}

		// Also check for individual conversation files
		convDir := filepath.Join(projectPath, "conversations")
		if DirExists(convDir) {
			convEntries, _ := os.ReadDir(convDir)
			for _, convEntry := range convEntries {
				if convEntry.IsDir() || !strings.HasSuffix(convEntry.Name(), ".json") {
					continue
				}

				// Parse individual conversation files
				convPath := filepath.Join(convDir, convEntry.Name())
				data, err := os.ReadFile(convPath)
				if err != nil {
					continue
				}

				var conv struct {
					ID       string `json:"id"`
					Project  string `json:"project"`
					Path     string `json:"path"`
					Modified int64  `json:"modified"`
					Messages []struct {
						ID               string `json:"id"`
						Role             string `json:"role"`
						Timestamp        int64  `json:"timestamp"`
						InputTokens      int    `json:"input_tokens"`
						OutputTokens     int    `json:"output_tokens"`
						Model            string `json:"model"`
					} `json:"messages"`
				}

				if err := json.Unmarshal(data, &conv); err != nil {
					continue
				}

				if conv.Modified <= state.LastScanTime {
					continue
				}

				for _, msg := range conv.Messages {
					if msg.Role != "assistant" {
						continue
					}

					result := ScanResult{
						AgentType:        "claude_code",
						Type:             "conversation",
						Entity:           conv.Path,
						Time:             float64(msg.Timestamp),
						Timestamp:        time.Unix(msg.Timestamp, 0),
						ConversationID:   conv.ID,
						MessageID:        msg.ID,
						PromptTokens:     msg.InputTokens,
						CompletionTokens: msg.OutputTokens,
						TotalTokens:      msg.InputTokens + msg.OutputTokens,
						Model:            msg.Model,
						Project:          conv.Project,
					}
					results = append(results, result)

					if msg.Timestamp > newState.LastScanTime {
						newState.LastScanTime = msg.Timestamp
						newState.LastRecordID = msg.ID
					}
				}
			}
		}
	}

	return results, newState, nil
}