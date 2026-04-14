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

// FactoryDetector scans Factory.ai conversation data
type FactoryDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewFactoryDetector() scanner.Detector {
	paths := []string{
		"~/.factory",
		"~/.config/factory",
		"~/.local/share/factory",
		"~/Library/Application Support/Factory",
	}
	return &FactoryDetector{
		BaseDetector: scanner.NewBaseDetector("factory", "Factory.ai conversations", paths, 50),
	}
}

func (d *FactoryDetector) Detect(ctx context.Context) (bool, error) {
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

func (d *FactoryDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

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
				AgentType:        "factory",
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

	return results, newState, nil
}

func init() {
	scanner.Register(NewFactoryDetector)
}
