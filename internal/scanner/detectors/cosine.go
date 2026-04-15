package detectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type CosineDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewCosineDetector() scanner.Detector {
	return &CosineDetector{
		BaseDetector: scanner.NewBaseDetector("cosine", "Cosine/COS CLI conversations", []string{"~/.cosine", "~/.config/cosine"}, 50),
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

type cosineIndex struct {
	Sessions []struct {
		SessionID string `json:"session_id"`
		Title     string `json:"title"`
		CWD       string `json:"cwd"`
		Path      string `json:"path"`
		EndUnix   int64  `json:"end_unix"`
		Branch    string `json:"branch"`
	} `json:"sessions"`
}

type cosineMetadata struct {
	SessionID          string    `toml:"session_id"`
	Title              string    `toml:"title"`
	CWD                string    `toml:"cwd"`
	TimeStarted        time.Time `toml:"time_started"`
	TimeEnded          time.Time `toml:"time_ended"`
	DurationSeconds    int       `toml:"duration_seconds"`
	LinesAdded         int       `toml:"lines_added"`
	LinesRemoved       int       `toml:"lines_removed"`
	Model              string    `toml:"model"`
	AgentActiveSeconds int       `toml:"agent_active_seconds"`
	HumanActiveSeconds int       `toml:"human_active_seconds"`
	IdleSeconds        int       `toml:"idle_seconds"`
	PromptTokens       int       `toml:"prompt_tokens"`
	CompletionTokens   int       `toml:"completion_tokens"`
	TotalTokens        int       `toml:"total_tokens"`
}

func (d *CosineDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	indexPath := filepath.Join(d.configDir, "sessions.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("read cosine sessions index: %w", err)
	}

	var index cosineIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, state, fmt.Errorf("decode cosine sessions index: %w", err)
	}

	sort.Slice(index.Sessions, func(i, j int) bool {
		if index.Sessions[i].EndUnix != index.Sessions[j].EndUnix {
			return index.Sessions[i].EndUnix < index.Sessions[j].EndUnix
		}
		return index.Sessions[i].SessionID < index.Sessions[j].SessionID
	})

	results := make([]scanner.ScanResult, 0, len(index.Sessions))
	newState := state

	for _, entry := range index.Sessions {
		select {
		case <-ctx.Done():
			return results, newState, ctx.Err()
		default:
		}

		sessionID := strings.TrimSpace(entry.SessionID)
		if sessionID == "" {
			continue
		}
		if entry.EndUnix < state.LastScanTime || (entry.EndUnix == state.LastScanTime && sessionID <= state.LastRecordID) {
			continue
		}

		mdBytes, err := os.ReadFile(filepath.Join(strings.TrimSpace(entry.Path), "metadata.toml"))
		if err != nil {
			continue
		}

		var md cosineMetadata
		if err := toml.Unmarshal(mdBytes, &md); err != nil {
			continue
		}
		var mdFields map[string]any
		_ = toml.Unmarshal(mdBytes, &mdFields)

		endedAt := time.Unix(entry.EndUnix, 0).UTC()
		if !md.TimeEnded.IsZero() {
			endedAt = md.TimeEnded.UTC()
		}
		startedAt := endedAt
		if !md.TimeStarted.IsZero() {
			startedAt = md.TimeStarted.UTC()
		}

		result := scanner.ScanResult{
			AgentType:              "cosine",
			Type:                   "conversation",
			Entity:                 md.CWD,
			Time:                   float64(endedAt.Unix()),
			Timestamp:              endedAt,
			Duration:               float64(md.DurationSeconds),
			SessionStartedAt:       timePtr(startedAt),
			SessionEndedAt:         timePtr(endedAt),
			SessionDurationSeconds: intPtr(md.DurationSeconds),
			ConversationID:         sessionID,
			MessageID:              sessionID,
			PromptTokens:           md.PromptTokens,
			CompletionTokens:       md.CompletionTokens,
			TotalTokens:            md.TotalTokens,
			TokenUsageKnown:        hasAnyTOMLKey(mdFields, "prompt_tokens", "completion_tokens", "total_tokens"),
			Model:                  md.Model,
			Project:                projectNameFromPath(md.CWD),
			LinesAdded:             md.LinesAdded,
			LinesDeleted:           md.LinesRemoved,
			Metadata: map[string]any{
				"title": md.Title,
			},
		}
		if md.AgentActiveSeconds > 0 {
			result.AgentActiveSeconds = intPtr(md.AgentActiveSeconds)
		}
		if md.HumanActiveSeconds > 0 {
			result.HumanActiveSeconds = intPtr(md.HumanActiveSeconds)
		}
		if md.IdleSeconds > 0 {
			result.IdleSeconds = intPtr(md.IdleSeconds)
		}

		results = append(results, result)
		newState.LastScanTime = entry.EndUnix
		newState.LastRecordID = sessionID
	}

	return results, newState, nil
}

func init() {
	scanner.Register(NewCosineDetector)
}
