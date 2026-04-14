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

// GeminiCLIDetector scans Google Gemini CLI data
type GeminiCLIDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewGeminiCLIDetector() scanner.Detector {
	paths := []string{
		"~/.gemini",
		"~/.config/gemini",
		"~/.local/share/gemini",
		"~/Library/Application Support/Gemini",
	}
	return &GeminiCLIDetector{
		BaseDetector: scanner.NewBaseDetector("gemini_cli", "Google Gemini CLI conversations", paths, 50),
	}
}

func (d *GeminiCLIDetector) Detect(ctx context.Context) (bool, error) {
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

func (d *GeminiCLIDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	historyDir := filepath.Join(d.configDir, "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read history dir: %w", err)
	}

	var results []scanner.ScanResult
	newState := state

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(historyDir, entry.Name())
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
				PromptTokens     int     `json:"input_tokens"`
				CompletionTokens int     `json:"output_tokens"`
				TotalTokens      int     `json:"total_tokens"`
				Model            string  `json:"model"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		if session.Modified <= state.LastScanTime {
			continue
		}

		for _, msg := range session.Messages {
			if msg.Role != "model" && msg.Role != "assistant" {
				continue
			}

			result := scanner.ScanResult{
				AgentType:        "gemini_cli",
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
	scanner.Register(NewGeminiCLIDetector)
}
