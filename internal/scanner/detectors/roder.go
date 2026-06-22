package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

var roderSyntheticThreadIDs = map[string]struct{}{
	"app-server":       {},
	"runtime":          {},
	"thread-workflow":  {},
}

type RoderDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewRoderDetector() scanner.Detector {
	return &RoderDetector{
		BaseDetector: scanner.NewBaseDetector(
			"roder",
			"Roder agent harness threads",
			[]string{"~/.roder"},
			50,
		),
	}
}

func init() {
	scanner.Register(NewRoderDetector)
}

func (d *RoderDetector) Detect(ctx context.Context) (bool, error) {
	for _, candidate := range roderDataDirs() {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		expanded, err := scanner.ExpandHome(candidate)
		if err != nil {
			continue
		}
		threadsDir := filepath.Join(expanded, "threads")
		if !scanner.DirExists(threadsDir) {
			continue
		}
		d.dataDir = expanded
		d.SetFoundPath(expanded)
		return true, nil
	}
	return false, nil
}

func roderDataDirs() []string {
	dirs := make([]string, 0, 3)
	if value := strings.TrimSpace(os.Getenv("RODER_CONFIG_DIR")); value != "" {
		dirs = append(dirs, value)
	}
	if value := strings.TrimSpace(os.Getenv("RODER_DATA_DIR")); value != "" {
		dirs = append(dirs, value)
	}
	dirs = append(dirs, "~/.roder")
	return dirs
}

type roderThreadMetadata struct {
	ThreadID     string              `json:"thread_id"`
	Title        *string             `json:"title"`
	Workspace    string              `json:"workspace"`
	Provider     *string             `json:"provider"`
	Model        *string             `json:"model"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	MessageCount int                 `json:"message_count"`
	Usage        *roderThreadUsage   `json:"usage"`
}

type roderThreadUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type roderThreadSummary struct {
	ThreadID         string
	Title            string
	Workspace        string
	Provider         string
	Model            string
	StartedAt        time.Time
	EndedAt          time.Time
	MessageCount     int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	AgentActive      time.Duration
	FileEdits        map[string]scanner.FileEdit
}

func (d *RoderDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	summaries, err := d.loadThreadSummaries(ctx)
	if err != nil {
		return nil, state, err
	}

	results := make([]scanner.ScanResult, 0, len(summaries))
	newState := state

	for _, summary := range summaries {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}

		endUnix := summary.EndedAt.Unix()
		if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && summary.ThreadID <= state.LastRecordID) {
			continue
		}

		sessionSeconds := durationSeconds(summary.StartedAt, summary.EndedAt)

		metadata := map[string]any{
			"title":          summary.Title,
			"provider":       summary.Provider,
			"message_count":  summary.MessageCount,
			"thread_id":      summary.ThreadID,
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "roder",
			Type:                   "conversation",
			Entity:                 summary.Workspace,
			Time:                   float64(endUnix),
			Timestamp:              summary.EndedAt,
			Duration:               float64(sessionSeconds),
			ConversationID:         summary.ThreadID,
			MessageID:              summary.ThreadID,
			PromptTokens:           summary.PromptTokens,
			CompletionTokens:       summary.CompletionTokens,
			TotalTokens:            summary.TotalTokens,
			Model:                  summary.Model,
			FileEdits:              flattenFileEdits(summary.FileEdits),
			Project:                projectNameFromPath(summary.Workspace),
			Metadata:               metadata,
		})

		newState.LastScanTime = endUnix
		newState.LastRecordID = summary.ThreadID
	}

	return results, newState, nil
}

func (d *RoderDetector) loadThreadSummaries(ctx context.Context) ([]roderThreadSummary, error) {
	threadRoots := []string{
		filepath.Join(d.dataDir, "threads"),
		filepath.Join(d.dataDir, "archived_threads"),
	}

	summaries := make([]roderThreadSummary, 0, 32)
	for _, root := range threadRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read roder threads dir %s: %w", root, err)
		}

		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if !entry.IsDir() {
				continue
			}
			if _, synthetic := roderSyntheticThreadIDs[entry.Name()]; synthetic {
				continue
			}

			summary, ok, err := summarizeRoderThread(filepath.Join(root, entry.Name()))
			if err != nil {
				return nil, err
			}
			if ok {
				summaries = append(summaries, summary)
			}
		}
	}

	return summaries, nil
}

func summarizeRoderThread(threadDir string) (roderThreadSummary, bool, error) {
	metadataPath := filepath.Join(threadDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return roderThreadSummary{}, false, nil
		}
		return roderThreadSummary{}, false, err
	}

	var metadata roderThreadMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return roderThreadSummary{}, false, nil
	}

	threadID := strings.TrimSpace(metadata.ThreadID)
	if threadID == "" {
		threadID = filepath.Base(threadDir)
	}
	if _, synthetic := roderSyntheticThreadIDs[threadID]; synthetic {
		return roderThreadSummary{}, false, nil
	}

	startedAt := parseRFC3339Any(metadata.CreatedAt).UTC()
	endedAt := parseRFC3339Any(metadata.UpdatedAt).UTC()
	if endedAt.IsZero() {
		return roderThreadSummary{}, false, nil
	}
	if startedAt.IsZero() {
		startedAt = endedAt
	}

	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	if metadata.Usage != nil {
		promptTokens = metadata.Usage.PromptTokens
		completionTokens = metadata.Usage.CompletionTokens
		totalTokens = metadata.Usage.TotalTokens
	}
	if totalTokens == 0 && metadata.MessageCount == 0 {
		return roderThreadSummary{}, false, nil
	}

	title := ""
	if metadata.Title != nil {
		title = strings.TrimSpace(*metadata.Title)
	}
	provider := ""
	if metadata.Provider != nil {
		provider = strings.TrimSpace(*metadata.Provider)
	}
	model := ""
	if metadata.Model != nil {
		model = strings.TrimSpace(*metadata.Model)
	}
	if model == "" {
		model = provider
	} else if provider != "" && !strings.Contains(model, provider) {
		model = provider + "/" + model
	}

	workspace := strings.TrimSpace(metadata.Workspace)
	if workspace == "" {
		workspace = threadID
	}

	agentActive, fileEdits := summarizeRoderEvents(filepath.Join(threadDir, "events.jsonl"))

	return roderThreadSummary{
		ThreadID:         threadID,
		Title:            title,
		Workspace:        workspace,
		Provider:         provider,
		Model:            model,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		MessageCount:     metadata.MessageCount,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		AgentActive:      agentActive,
		FileEdits:        fileEdits,
	}, true, nil
}

func summarizeRoderEvents(path string) (time.Duration, map[string]scanner.FileEdit) {
	fileEdits := make(map[string]scanner.FileEdit)
	turnStarts := make(map[string]time.Time)
	var agentActive time.Duration

	file, err := os.Open(path)
	if err != nil {
		return agentActive, fileEdits
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var envelope struct {
			Kind      string          `json:"kind"`
			Timestamp string          `json:"timestamp"`
			TurnID    *string         `json:"turn_id"`
			Event     json.RawMessage `json:"event"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			continue
		}

		switch envelope.Kind {
		case "turn.started":
			turnID := roderTurnID(envelope.TurnID, envelope.Event, "TurnStarted")
			if turnID == "" {
				continue
			}
			startedAt := parseRFC3339Any(envelope.Timestamp)
			if startedAt.IsZero() {
				startedAt = roderEventTimestamp(envelope.Event)
			}
			if !startedAt.IsZero() {
				turnStarts[turnID] = startedAt.UTC()
			}
		case "turn.completed":
			turnID := roderTurnID(envelope.TurnID, envelope.Event, "TurnCompleted")
			if turnID == "" {
				continue
			}
			startedAt, ok := turnStarts[turnID]
			if !ok {
				continue
			}
			completedAt := parseRFC3339Any(envelope.Timestamp)
			if completedAt.IsZero() {
				completedAt = roderEventTimestamp(envelope.Event)
			}
			if completedAt.After(startedAt) {
				agentActive += completedAt.Sub(startedAt)
			}
			delete(turnStarts, turnID)
		case "workspace/changeObserved":
			mergeFileEdits(fileEdits, parseRoderWorkspaceChange(envelope.Event))
		case "file.change_preview_ready":
			mergeFileEdits(fileEdits, parseRoderFileChangePreview(envelope.Event))
		}
	}

	return agentActive, fileEdits
}

func roderTurnID(turnID *string, event json.RawMessage, variant string) string {
	if turnID != nil {
		if id := strings.TrimSpace(*turnID); id != "" {
			return id
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(event, &payload); err != nil {
		return ""
	}
	nested, ok := payload[variant].(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue(nested["turn_id"]))
}

func roderEventTimestamp(event json.RawMessage) time.Time {
	var payload map[string]any
	if err := json.Unmarshal(event, &payload); err != nil {
		return time.Time{}
	}
	for _, value := range payload {
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if parsed := parseRFC3339Any(stringValue(nested["timestamp"])); !parsed.IsZero() {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func parseRoderWorkspaceChange(event json.RawMessage) map[string]scanner.FileEdit {
	edits := make(map[string]scanner.FileEdit)

	var payload struct {
		WorkspaceChangeObserved struct {
			Change struct {
				Files []struct {
					Path       string `json:"path"`
					Additions  int    `json:"additions"`
					Deletions  int    `json:"deletions"`
				} `json:"files"`
			} `json:"change"`
		} `json:"WorkspaceChangeObserved"`
	}
	if err := json.Unmarshal(event, &payload); err != nil {
		return edits
	}

	for _, file := range payload.WorkspaceChangeObserved.Change.Files {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}
		edit := edits[path]
		edit.Path = path
		edit.EditCount++
		edit.LinesAdded += file.Additions
		edit.LinesDeleted += file.Deletions
		edits[path] = edit
	}

	return edits
}

func parseRoderFileChangePreview(event json.RawMessage) map[string]scanner.FileEdit {
	edits := make(map[string]scanner.FileEdit)

	var payload struct {
		FileChangePreviewReady struct {
			Path       string `json:"path"`
			ChangeType string `json:"change_type"`
		} `json:"FileChangePreviewReady"`
	}
	if err := json.Unmarshal(event, &payload); err != nil {
		return edits
	}

	path := strings.TrimSpace(payload.FileChangePreviewReady.Path)
	if path == "" {
		return edits
	}

	edit := edits[path]
	edit.Path = path
	edit.EditCount++
	if edit.LinesAdded == 0 && edit.LinesDeleted == 0 && payload.FileChangePreviewReady.ChangeType == "create" {
		edit.LinesAdded = 1
	}
	edits[path] = edit

	return edits
}
