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

// CodexDetector scans OpenAI Codex CLI data
type CodexDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewCodexDetector() scanner.Detector {
	paths := []string{
		"~/.codex",
		"~/.config/codex",
		"~/.local/share/codex",
		"~/Library/Application Support/Codex",
	}
	return &CodexDetector{
		BaseDetector: scanner.NewBaseDetector("codex", "OpenAI Codex CLI conversations", paths, 50),
	}
}

func (d *CodexDetector) Detect(ctx context.Context) (bool, error) {
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

func (d *CodexDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	sessionsDir := filepath.Join(d.configDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read sessions dir: %w", err)
	}

	var results []scanner.ScanResult
	newState := state

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue
		}

		var session struct {
			ID       string `json:"id"`
			Project  string `json:"project"`
			Path     string `json:"path"`
			Modified int64  `json:"modified"`
			Messages []struct {
				ID               string  `json:"id"`
				Role             string  `json:"role"`
				Timestamp        int64   `json:"timestamp"`
				PromptTokens     int     `json:"prompt_tokens"`
				CompletionTokens int     `json:"completion_tokens"`
				TotalTokens      int     `json:"total_tokens"`
				Model            string  `json:"model"`
				CostUSD          float64 `json:"cost_usd"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		if session.Modified <= state.LastScanTime {
			continue
		}

		for _, msg := range session.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := scanner.ScanResult{
				AgentType:        "codex",
				Type:             "conversation",
				Entity:           session.Path,
				Time:             float64(msg.Timestamp),
				Timestamp:        time.Unix(msg.Timestamp, 0),
				ConversationID:   session.ID,
				MessageID:        msg.ID,
				PromptTokens:     msg.PromptTokens,
				CompletionTokens: msg.CompletionTokens,
				TotalTokens:      msg.TotalTokens,
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

	return results, newState, nil
}

func init() {
	scanner.Register(NewCodexDetector)
}
