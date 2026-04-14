package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CopilotDetector scans GitHub Copilot CLI data
type CopilotDetector struct {
	dataDir string
}

func init() {
	Register(&CopilotDetector{})
}

func (d *CopilotDetector) Name() string {
	return "copilot_cli"
}

func (d *CopilotDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".config", "github-copilot"),
		filepath.Join(home, "AppData", "Local", "github-copilot"), // Windows
	}
}

func (d *CopilotDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.dataDir = path
			return true, nil
		}
	}
	return false, nil
}

func (d *CopilotDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	// Copilot CLI stores usage data in copilot-usage.json or similar
	usagePath := filepath.Join(d.dataDir, "usage.json")
	if !FileExists(usagePath) {
		// Try alternative location
		usagePath = filepath.Join(d.dataDir, "copilot-usage.json")
	}

	if !FileExists(usagePath) {
		return nil, state, nil
	}

	data, err := os.ReadFile(usagePath)
	if err != nil {
		return nil, state, fmt.Errorf("read usage file: %w", err)
	}

	var usage struct {
		Sessions []struct {
			ID        string    `json:"id"`
			StartTime int64     `json:"start_time"`
			EndTime   int64     `json:"end_time"`
			Command   string    `json:"command"`
			Prompt    string    `json:"prompt"`
			Response  string    `json:"response"`
			Tokens    struct {
				Prompt     int `json:"prompt"`
				Completion int `json:"completion"`
				Total      int `json:"total"`
			} `json:"tokens"`
			Model   string  `json:"model"`
			CostUSD float64 `json:"cost_usd"`
		} `json:"sessions"`
	}

	if err := json.Unmarshal(data, &usage); err != nil {
		return nil, state, fmt.Errorf("parse usage: %w", err)
	}

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for _, session := range usage.Sessions {
		// Skip already processed sessions
		if session.StartTime <= state.LastScanTime {
			continue
		}

		result := ScanResult{
			AgentType:        "copilot_cli",
			Type:             "command",
			Entity:           session.Command,
			Time:             float64(session.EndTime),
			Timestamp:        time.Unix(session.EndTime, 0),
			Duration:         float64(session.EndTime - session.StartTime),
			ConversationID:   session.ID,
			PromptTokens:     session.Tokens.Prompt,
			CompletionTokens: session.Tokens.Completion,
			TotalTokens:      session.Tokens.Total,
			Model:            session.Model,
			CostUSD:          session.CostUSD,
			Metadata: map[string]any{
				"command": session.Command,
				"prompt":  session.Prompt,
			},
		}
		results = append(results, result)

		if session.EndTime > newState.LastScanTime {
			newState.LastScanTime = session.EndTime
			newState.LastRecordID = session.ID
		}
	}

	return results, newState, nil
}