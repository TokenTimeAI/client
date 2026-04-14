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

// CosineDetector scans Cosine/COS CLI data
type CosineDetector struct {
	scanner.BaseDetector
	configDir string
}

// NewCosineDetector creates a new Cosine detector
func NewCosineDetector() scanner.Detector {
	paths := []string{
		"~/.cosine",
		"~/.config/cosine",
	}
	return &CosineDetector{
		BaseDetector: scanner.NewBaseDetector("cosine", "Cosine/COS CLI conversations", paths, 50),
	}
}

func (d *CosineDetector) Detect(ctx context.Context) (bool, error) {
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

// cosineSession represents Cosine's session data structure
type cosineSession struct {
	ID        string          `json:"id"`
	Project   string          `json:"project"`
	CreatedAt time.Time       `json:"created_at"`
	Messages  []cosineMessage `json:"messages"`
	Metadata  map[string]any  `json:"metadata"`
}

type cosineMessage struct {
	ID               string  `json:"id"`
	Role             string  `json:"role"` // user, assistant, system
	Content          string  `json:"content"`
	Timestamp        int64   `json:"timestamp"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Model            string  `json:"model"`
	CostUSD          float64 `json:"cost_usd"`
}

func (d *CosineDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
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
	newState.LastScanTime = time.Now().Unix()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue
		}

		var session cosineSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		if session.ID == state.LastRecordID && session.CreatedAt.Unix() <= state.LastScanTime {
			continue
		}

		for _, msg := range session.Messages {
			if msg.Role != "assistant" {
				continue
			}

			result := scanner.ScanResult{
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

		if session.CreatedAt.Unix() > newState.LastScanTime {
			newState.LastScanTime = session.CreatedAt.Unix()
			newState.LastRecordID = session.ID
		}
	}

	return results, newState, nil
}

func init() {
	scanner.Register(NewCosineDetector)
}