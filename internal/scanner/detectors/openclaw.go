package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

// OpenClawDetector scans OpenClaw agent conversation data
type OpenClawDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewOpenClawDetector() scanner.Detector {
	paths := []string{
		"~/.openclaw",
		"~/.openclaw-dev",
	}
	return &OpenClawDetector{
		BaseDetector: scanner.NewBaseDetector("openclaw", "OpenClaw agent conversations", paths, 50),
	}
}

func (d *OpenClawDetector) Detect(ctx context.Context) (bool, error) {
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

// openclawSessionsJSON represents the sessions.json structure
// where keys are agent IDs (e.g., "agent:main", "agent:subagent:main")
// and values contain session metadata
type openclawSessionsJSON map[string]struct {
	SessionID   string `json:"sessionId"`
	SessionFile string `json:"sessionFile"`
	UpdatedAt   int64  `json:"updatedAt"` // milliseconds timestamp
	CreatedAt   int64  `json:"createdAt"` // milliseconds timestamp
	AgentID     string `json:"agentId"`
	Title       string `json:"title"`
}

type openclawEvent struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"` // milliseconds
	Role      string `json:"role,omitempty"`
	Model     string `json:"model,omitempty"`
	Usage     struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

func (d *OpenClawDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	// Find all session files across agents
	agentsDir := filepath.Join(d.dataDir, "agents")
	agents, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read agents dir: %w", err)
	}

	type sessionInfo struct {
		SessionKey  string
		SessionID   string
		SessionFile string
		UpdatedAt   int64
		CreatedAt   int64
		AgentID     string
		Title       string
	}

	var sessions []sessionInfo

	for _, agent := range agents {
		if !agent.IsDir() {
			continue
		}

		sessionsPath := filepath.Join(agentsDir, agent.Name(), "sessions", "sessions.json")
		data, err := os.ReadFile(sessionsPath)
		if err != nil {
			continue
		}

		var sessionsData openclawSessionsJSON
		if err := json.Unmarshal(data, &sessionsData); err != nil {
			continue
		}

		for sessionKey, sessionData := range sessionsData {
			sessions = append(sessions, sessionInfo{
				SessionKey:  sessionKey,
				SessionID:   sessionData.SessionID,
				SessionFile: sessionData.SessionFile,
				UpdatedAt:   sessionData.UpdatedAt,
				CreatedAt:   sessionData.CreatedAt,
				AgentID:     agent.Name(),
				Title:       sessionData.Title,
			})
		}
	}

	// Sort by UpdatedAt (milliseconds)
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt != sessions[j].UpdatedAt {
			return sessions[i].UpdatedAt < sessions[j].UpdatedAt
		}
		return sessions[i].SessionID < sessions[j].SessionID
	})

	var results []scanner.ScanResult
	newState := state

	for _, session := range sessions {
		select {
		case <-ctx.Done():
			return results, newState, ctx.Err()
		default:
		}

		// Convert milliseconds to seconds for comparison
		updatedAtSec := session.UpdatedAt / 1000

		if updatedAtSec < state.LastScanTime || (updatedAtSec == state.LastScanTime && session.SessionID <= state.LastRecordID) {
			continue
		}

		// Read transcript to extract token counts
		// SessionFile is relative path like "sessions/<sessionId>.jsonl"
		transcriptPath := filepath.Join(agentsDir, session.AgentID, session.SessionFile)
		if session.SessionFile == "" {
			// Fallback: construct from session ID
			transcriptPath = filepath.Join(agentsDir, session.AgentID, "sessions", session.SessionID+".jsonl")
		}

		totalPromptTokens, totalCompletionTokens, totalTokens, model := d.parseTranscript(transcriptPath)

		startedAt := time.UnixMilli(session.CreatedAt).UTC()
		endedAt := time.UnixMilli(session.UpdatedAt).UTC()
		sessionSeconds := durationSeconds(startedAt, endedAt)

		results = append(results, scanner.ScanResult{
			AgentType:              "openclaw",
			Type:                   "conversation",
			Entity:                 session.SessionKey,
			Time:                   float64(updatedAtSec),
			Timestamp:              endedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(startedAt),
			SessionEndedAt:         timePtr(endedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			ConversationID:         session.SessionID,
			MessageID:              session.SessionID,
			PromptTokens:           totalPromptTokens,
			CompletionTokens:       totalCompletionTokens,
			TotalTokens:            totalTokens,
			Model:                  model,
			Project:                session.AgentID,
			Metadata: map[string]any{
				"title":       session.Title,
				"agent_id":    session.AgentID,
				"session_key": session.SessionKey,
			},
		})

		newState.LastScanTime = updatedAtSec
		newState.LastRecordID = session.SessionID
	}

	return results, newState, nil
}

func (d *OpenClawDetector) parseTranscript(path string) (promptTokens, completionTokens, totalTokens int, model string) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event openclawEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		if event.Type == "assistant" {
			if event.Usage.InputTokens > 0 {
				promptTokens += event.Usage.InputTokens
			}
			if event.Usage.OutputTokens > 0 {
				completionTokens += event.Usage.OutputTokens
			}
			if event.Model != "" && model == "" {
				model = event.Model
			}
		}
	}

	totalTokens = promptTokens + completionTokens
	return promptTokens, completionTokens, totalTokens, model
}

func init() {
	scanner.Register(NewOpenClawDetector)
}
