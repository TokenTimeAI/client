package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// OpenCodeDetector scans OpenCode conversation data
type OpenCodeDetector struct {
	configDir string
}

func init() {
	Register(&OpenCodeDetector{})
}

func (d *OpenCodeDetector) Name() string {
	return "opencode"
}

func (d *OpenCodeDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".opencode"),
		filepath.Join(home, ".config", "opencode"),
	}
}

func (d *OpenCodeDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.configDir = path
			return true, nil
		}
	}
	return false, nil
}

func (d *OpenCodeDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	// OpenCode stores conversations in JSON format
	conversationsDir := filepath.Join(d.configDir, "conversations")
	entries, err := os.ReadDir(conversationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read conversations dir: %w", err)
	}

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for _, entry := range entries {
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

		// Skip if already processed
		if conversation.Modified <= state.LastScanTime {
			continue
		}

		for _, msg := range conversation.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := ScanResult{
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