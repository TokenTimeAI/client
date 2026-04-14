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

// TemplateDetector is a template for implementing new agent detectors.
// Copy this file and replace "Template" with your agent name.
type TemplateDetector struct {
	scanner.BaseDetector
	dataDir string // Stores the detected path after Detect() succeeds
}

// NewTemplateDetector creates a new detector instance.
// Rename this function to match your agent (e.g., NewMyAgentDetector).
func NewTemplateDetector() scanner.Detector {
	paths := []string{
		// Cross-platform paths where your agent stores data
		"~/Library/Application Support/YourAgent", // macOS
		"~/.config/youragent",                      // Linux
		"~/AppData/Roaming/YourAgent",             // Windows
	}
	return &TemplateDetector{
		BaseDetector: scanner.NewBaseDetector(
			"template",                    // Unique agent name (lowercase, no spaces)
			"Your Agent Description",       // Human-readable description
			paths,
			50,                            // Priority (higher = scanned first)
		),
	}
}

// Detect checks if this agent's data exists on the system.
// Store the found path in d.dataDir for use by Scan().
func (d *TemplateDetector) Detect(ctx context.Context) (bool, error) {
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

// Scan retrieves new conversation data since the last scan.
// Implement incremental scanning using state.LastScanTime and state.LastRecordID.
func (d *TemplateDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	// Example: Scanning JSON files in a directory
	dataDir := filepath.Join(d.dataDir, "conversations")
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read data dir: %w", err)
	}

	var results []scanner.ScanResult
	newState := state

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(dataDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		// Parse your agent's data format
		var conversation struct {
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

		if err := json.Unmarshal(data, &conversation); err != nil {
			continue // Skip malformed files
		}

		// Skip already processed conversations (incremental scanning)
		if conversation.Modified <= state.LastScanTime {
			continue
		}

		// Process each message
		for _, msg := range conversation.Messages {
			// Skip user messages - we only want assistant/model responses
			if msg.Role != "assistant" && msg.Role != "model" {
				continue
			}

			result := scanner.ScanResult{
				AgentType:        d.Name(),
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

			// Update state tracking
			if msg.Timestamp > newState.LastScanTime {
				newState.LastScanTime = msg.Timestamp
				newState.LastRecordID = msg.ID
			}
		}
	}

	return results, newState, nil
}

// init registers the detector with the global registry.
// The underscore ensures the init runs even if the detector isn't directly referenced.
var _ = func() bool {
	scanner.Register(NewTemplateDetector)
	return true
}()

// Alternative init pattern (choose one):
// func init() {
//     scanner.Register(NewTemplateDetector)
// }
