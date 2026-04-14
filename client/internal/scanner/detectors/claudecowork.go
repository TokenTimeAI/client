package detectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

// ClaudeCoworkDetector scans Claude Cowork (Claude for Work) data
type ClaudeCoworkDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewClaudeCoworkDetector() scanner.Detector {
	paths := []string{
		"~/.claude-cowork",
		"~/.config/claude-cowork",
		"~/.local/share/claude-cowork",
		"~/Library/Application Support/Claude Cowork",
	}
	return &ClaudeCoworkDetector{
		BaseDetector: scanner.NewBaseDetector("claude_cowork", "Claude Cowork conversations", paths, 50),
	}
}

func (d *ClaudeCoworkDetector) Detect(ctx context.Context) (bool, error) {
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

func (d *ClaudeCoworkDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	workspacesDir := filepath.Join(d.dataDir, "workspaces")
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read workspaces dir: %w", err)
	}

	var results []scanner.ScanResult
	newState := state

	for _, workspaceEntry := range entries {
		if !workspaceEntry.IsDir() {
			continue
		}

		workspacePath := filepath.Join(workspacesDir, workspaceEntry.Name())
		conversationsDir := filepath.Join(workspacePath, "conversations")

		convEntries, err := os.ReadDir(conversationsDir)
		if err != nil {
			continue
		}

		for _, entry := range convEntries {
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
				Path     string `json:"path"`
				Modified int64  `json:"modified"`
				Messages []struct {
					ID               string  `json:"id"`
					Role             string  `json:"role"`
					Timestamp        int64   `json:"timestamp"`
					PromptTokens     int     `json:"input_tokens"`
					CompletionTokens int     `json:"output_tokens"`
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
					AgentType:        "claude_cowork",
					Type:             "conversation",
					Entity:           conversation.Path,
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
	}

	return results, newState, nil
}

func init() {
	scanner.Register(NewClaudeCoworkDetector)
}
