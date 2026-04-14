package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CosineDetector scans Cosine/COS CLI data
type CosineDetector struct {
	configDir string
}

func init() {
	Register(&CosineDetector{})
}

func (d *CosineDetector) Name() string {
	return "cosine"
}

func (d *CosineDetector) DefaultPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".cosine"),
		filepath.Join(home, ".config", "cosine"),
	}
}

func (d *CosineDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		if DirExists(path) {
			d.configDir = path
			return true, nil
		}
	}
	return false, nil
}

// cosineSession represents Cosine's session data structure
type cosineSession struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	CreatedAt time.Time `json:"created_at"`
	Messages  []cosineMessage `json:"messages"`
	Metadata  map[string]any `json:"metadata"`
}

type cosineMessage struct {
	ID               string `json:"id"`
	Role             string `json:"role"` // user, assistant, system
	Content          string `json:"content"`
	Timestamp        int64  `json:"timestamp"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model"`
	CostUSD          float64 `json:"cost_usd"`
}

func (d *CosineDetector) Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	// Scan sessions directory
	sessionsDir := filepath.Join(d.configDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read sessions dir: %w", err)
	}

	var results []ScanResult
	newState := state
	newState.LastScanTime = time.Now().Unix()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue // Skip unreadable files
		}

		var session cosineSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue // Skip malformed files
		}

		// Skip if we've seen this session before
		if session.ID == state.LastRecordID && session.CreatedAt.Unix() <= state.LastScanTime {
			continue
		}

		// Process each message as a separate result
		for _, msg := range session.Messages {
			// Skip user messages (we want assistant/token usage data)
			if msg.Role != "assistant" {
				continue
			}

			result := ScanResult{
				AgentType:        "cosine",
				Type:             "conversation",
				Entity:           session.Project,
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
				Metadata:         session.Metadata,
			}
			results = append(results, result)
		}

		// Update state to latest
		if session.CreatedAt.Unix() > newState.LastScanTime {
			newState.LastScanTime = session.CreatedAt.Unix()
			newState.LastRecordID = session.ID
		}
	}

	return results, newState, nil
}