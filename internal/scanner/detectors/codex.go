package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type CodexDetector struct {
	scanner.BaseDetector
	configDir string
}

func NewCodexDetector() scanner.Detector {
	paths := []string{"~/.codex", "~/.config/codex", "~/.local/share/codex", "~/Library/Application Support/Codex"}
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

type codexThreadIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
}

type codexSessionSummary struct {
	SessionID        string
	Title            string
	CWD              string
	Model            string
	StartedAt        time.Time
	EndedAt          time.Time
	AgentActive      time.Duration
	HumanActive      time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	TokensKnown      bool
	HasCumulative    bool
}

type codexTokenInfo struct {
	TotalTokenUsage codexTokenUsage `json:"total_token_usage"`
	LastTokenUsage  codexTokenUsage `json:"last_token_usage"`
}

type codexTokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func (d *CodexDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.configDir == "" {
		return nil, state, nil
	}

	titles := d.loadThreadTitles()
	paths := make([]string, 0, 128)
	sessionsDir := filepath.Join(d.configDir, "sessions")
	if err := filepath.WalkDir(sessionsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, state, fmt.Errorf("walk codex sessions: %w", err)
	}

	summaries := make([]codexSessionSummary, 0, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}

		summary, ok := summarizeCodexSession(path, titles)
		if !ok || summary.SessionID == "" || summary.EndedAt.IsZero() {
			continue
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if !summaries[i].EndedAt.Equal(summaries[j].EndedAt) {
			return summaries[i].EndedAt.Before(summaries[j].EndedAt)
		}
		return summaries[i].SessionID < summaries[j].SessionID
	})

	results := make([]scanner.ScanResult, 0, len(summaries))
	newState := state

	for _, summary := range summaries {
		endUnix := summary.EndedAt.Unix()
		if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && summary.SessionID <= state.LastRecordID) {
			continue
		}

		sessionSeconds := durationSeconds(summary.StartedAt, summary.EndedAt)
		agentSeconds := int(summary.AgentActive.Round(time.Second).Seconds())
		humanSeconds := int(summary.HumanActive.Round(time.Second).Seconds())
		idleSeconds := sessionSeconds - agentSeconds - humanSeconds
		if idleSeconds < 0 {
			idleSeconds = 0
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "codex",
			Type:                   "conversation",
			Entity:                 summary.CWD,
			Time:                   float64(endUnix),
			Timestamp:              summary.EndedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(summary.StartedAt),
			SessionEndedAt:         timePtr(summary.EndedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			AgentActiveSeconds:     intPtr(agentSeconds),
			HumanActiveSeconds:     intPtr(humanSeconds),
			IdleSeconds:            intPtr(idleSeconds),
			ConversationID:         summary.SessionID,
			MessageID:              summary.SessionID,
			PromptTokens:           summary.PromptTokens,
			CompletionTokens:       summary.CompletionTokens,
			TotalTokens:            summary.TotalTokens,
			Model:                  summary.Model,
			Project:                projectNameFromPath(summary.CWD),
			Metadata: map[string]any{
				"title": summary.Title,
			},
		})

		newState.LastScanTime = endUnix
		newState.LastRecordID = summary.SessionID
	}

	return results, newState, nil
}

func (d *CodexDetector) loadThreadTitles() map[string]string {
	indexPath := filepath.Join(d.configDir, "session_index.jsonl")
	file, err := os.Open(indexPath)
	if err != nil {
		return map[string]string{}
	}
	defer file.Close()

	titles := make(map[string]string)
	lineScanner := bufio.NewScanner(file)
	for lineScanner.Scan() {
		var entry codexThreadIndexEntry
		if err := json.Unmarshal(lineScanner.Bytes(), &entry); err != nil {
			continue
		}
		if strings.TrimSpace(entry.ID) != "" {
			titles[entry.ID] = strings.TrimSpace(entry.ThreadName)
		}
	}
	return titles
}

func summarizeCodexSession(path string, titles map[string]string) (codexSessionSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return codexSessionSummary{}, false
	}
	defer file.Close()

	var summary codexSessionSummary
	var lastTaskCompletedAt *time.Time
	seenSessionMeta := false
	lineScanner := bufio.NewScanner(file)

	for lineScanner.Scan() {
		var envelope struct {
			Timestamp string          `json:"timestamp"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(lineScanner.Bytes(), &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "session_meta":
			var payload struct {
				ID        string `json:"id"`
				Timestamp string `json:"timestamp"`
				CWD       string `json:"cwd"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				continue
			}
			summary.SessionID = strings.TrimSpace(payload.ID)
			summary.CWD = strings.TrimSpace(payload.CWD)
			summary.Title = titles[summary.SessionID]
			summary.StartedAt = parseRFC3339Any(payload.Timestamp).UTC()
			seenSessionMeta = true
		case "turn_context":
			var payload struct {
				Model string `json:"model"`
				CWD   string `json:"cwd"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err == nil {
				if summary.Model == "" {
					summary.Model = strings.TrimSpace(payload.Model)
				}
				if summary.CWD == "" {
					summary.CWD = strings.TrimSpace(payload.CWD)
				}
			}
		case "event_msg":
			var payload struct {
				Type        string `json:"type"`
				StartedAt   int64  `json:"started_at"`
				CompletedAt int64  `json:"completed_at"`
				DurationMS  int64  `json:"duration_ms"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				continue
			}
			switch payload.Type {
			case "task_started":
				startedAt := time.Unix(payload.StartedAt, 0).UTC()
				if summary.StartedAt.IsZero() {
					summary.StartedAt = startedAt
				}
				if lastTaskCompletedAt != nil && startedAt.After(*lastTaskCompletedAt) {
					summary.HumanActive += startedAt.Sub(*lastTaskCompletedAt)
				}
			case "task_complete":
				completedAt := time.Unix(payload.CompletedAt, 0).UTC()
				if payload.DurationMS > 0 {
					summary.AgentActive += time.Duration(payload.DurationMS) * time.Millisecond
				}
				if completedAt.After(summary.EndedAt) {
					summary.EndedAt = completedAt
				}
				lastTaskCompletedAt = &completedAt
			case "token_count":
				var tokenPayload struct {
					Info codexTokenInfo `json:"info"`
				}
				if err := json.Unmarshal(envelope.Payload, &tokenPayload); err == nil {
					summary.TokensKnown = true

					totalUsage := tokenPayload.Info.TotalTokenUsage
					if totalUsage.TotalTokens > 0 {
						summary.HasCumulative = true
						if totalUsage.TotalTokens > summary.TotalTokens {
							summary.PromptTokens = totalUsage.InputTokens
							summary.CompletionTokens = totalUsage.OutputTokens
							summary.TotalTokens = totalUsage.TotalTokens
						}
						continue
					}

					if summary.HasCumulative {
						continue
					}

					lastUsage := tokenPayload.Info.LastTokenUsage
					summary.PromptTokens += lastUsage.InputTokens
					summary.CompletionTokens += lastUsage.OutputTokens
					if lastUsage.TotalTokens > 0 {
						summary.TotalTokens += lastUsage.TotalTokens
					} else {
						summary.TotalTokens += lastUsage.InputTokens + lastUsage.OutputTokens
					}
				}
			}
		}

		if ts := parseRFC3339Any(envelope.Timestamp).UTC(); !ts.IsZero() && ts.After(summary.EndedAt) {
			summary.EndedAt = ts
		}
	}

	if !seenSessionMeta {
		return codexSessionSummary{}, false
	}
	if summary.TotalTokens == 0 && summary.TokensKnown {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	return summary, true
}

func init() {
	scanner.Register(NewCodexDetector)
}
