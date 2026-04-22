package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Event struct {
	Entity                 string         `json:"entity"`
	Type                   string         `json:"type"`
	Project                string         `json:"project,omitempty"`
	Branch                 string         `json:"branch,omitempty"`
	Language               string         `json:"language,omitempty"`
	AgentType              string         `json:"agent_type,omitempty"`
	Time                   float64        `json:"time"`
	Duration               float64        `json:"duration,omitempty"`
	SessionStartedAt       *string        `json:"session_started_at,omitempty"`
	SessionEndedAt         *string        `json:"session_ended_at,omitempty"`
	SessionDurationSeconds *int           `json:"session_duration_seconds,omitempty"`
	AgentActiveSeconds     *int           `json:"agent_active_seconds,omitempty"`
	HumanActiveSeconds     *int           `json:"human_active_seconds,omitempty"`
	IdleSeconds            *int           `json:"idle_seconds,omitempty"`
	IsWrite                bool           `json:"is_write,omitempty"`
	TokensUsed             int            `json:"tokens_used,omitempty"`
	LinesAdded             int            `json:"lines_added,omitempty"`
	LinesDeleted           int            `json:"lines_deleted,omitempty"`
	CostUSD                float64        `json:"cost_usd,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`

	// Conversation tracking fields
	ConversationID   string `json:"conversation_id,omitempty"`
	MessageID        string `json:"message_id,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty"`
	Model            string `json:"model,omitempty"`
	ImportRunID      string `json:"import_run_id,omitempty"`
	SourceFingerprint string `json:"source_fingerprint,omitempty"`
	FileEdits        []FileEdit `json:"file_edits,omitempty"`
}

type FileEdit struct {
	Path         string `json:"path"`
	EditCount    int    `json:"edit_count,omitempty"`
	LinesAdded   int    `json:"lines_added,omitempty"`
	LinesDeleted int    `json:"lines_deleted,omitempty"`
}

type JSONLCollector struct {
	InboxDir  string
	StatePath string
}

type fileOffsets map[string]int64

func NewJSONLCollector(inboxDir, statePath string) *JSONLCollector {
	return &JSONLCollector{
		InboxDir:  inboxDir,
		StatePath: statePath,
	}
}

func (c *JSONLCollector) Collect(_ context.Context) ([]Event, error) {
	if err := os.MkdirAll(c.InboxDir, 0o755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(c.InboxDir)
	if err != nil {
		return nil, err
	}

	offsets, err := c.loadOffsets()
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		path := filepath.Join(c.InboxDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		offset := offsets[path]
		if info.Size() < offset {
			offset = 0
		}

		fileEvents, nextOffset, err := readEventsFromFile(path, offset)
		if err != nil {
			return nil, err
		}
		offsets[path] = nextOffset
		events = append(events, fileEvents...)
	}

	if err := c.saveOffsets(offsets); err != nil {
		return nil, err
	}

	return events, nil
}

func readEventsFromFile(path string, startOffset int64) ([]Event, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, startOffset, err
	}
	defer file.Close()

	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return nil, startOffset, err
	}

	reader := bufio.NewReader(file)
	currentOffset := startOffset
	var events []Event

	for {
		line, err := reader.ReadBytes('\n')
		currentOffset += int64(len(line))

		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			if event, parseErr := parseEvent([]byte(trimmed)); parseErr == nil {
				events = append(events, event)
			}
		}

		if errors.Is(err, io.EOF) {
			return events, currentOffset, nil
		}
		if err != nil {
			return nil, startOffset, err
		}
	}
}

func parseEvent(line []byte) (Event, error) {
	var body map[string]any
	if err := json.Unmarshal(line, &body); err != nil {
		return Event{}, err
	}

	return Event{
		Entity:                 getString(body, "entity"),
		Type:                   getString(body, "type"),
		Project:                getString(body, "project"),
		Branch:                 getString(body, "branch"),
		Language:               getString(body, "language"),
		AgentType:              getString(body, "agent_type"),
		Time:                   getFloat(body, "time"),
		Duration:               getFloat(body, "duration"),
		SessionStartedAt:       getOptionalString(body, "session_started_at"),
		SessionEndedAt:         getOptionalString(body, "session_ended_at"),
		SessionDurationSeconds: getOptionalInt(body, "session_duration_seconds"),
		AgentActiveSeconds:     getOptionalInt(body, "agent_active_seconds"),
		HumanActiveSeconds:     getOptionalInt(body, "human_active_seconds"),
		IdleSeconds:            getOptionalInt(body, "idle_seconds"),
		IsWrite:                getBool(body, "is_write"),
		TokensUsed:             getInt(body, "tokens_used"),
		LinesAdded:             getInt(body, "lines_added"),
		LinesDeleted:           getInt(body, "lines_deleted"),
		CostUSD:                getFloat(body, "cost_usd"),
		Metadata:               getMap(body, "metadata"),
		ConversationID:         getString(body, "conversation_id"),
		MessageID:              getString(body, "message_id"),
		PromptTokens:           getInt(body, "prompt_tokens"),
		CompletionTokens:       getInt(body, "completion_tokens"),
		TotalTokens:            getInt(body, "total_tokens"),
		Model:                  getString(body, "model"),
		ImportRunID:            getString(body, "import_run_id"),
		SourceFingerprint:      getString(body, "source_fingerprint"),
		FileEdits:              getFileEdits(body, "file_edits"),
	}, nil
}

func (c *JSONLCollector) loadOffsets() (fileOffsets, error) {
	data, err := os.ReadFile(c.StatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileOffsets{}, nil
		}
		return nil, err
	}

	var offsets fileOffsets
	if err := json.Unmarshal(data, &offsets); err != nil {
		return nil, err
	}
	return offsets, nil
}

func (c *JSONLCollector) saveOffsets(offsets fileOffsets) error {
	if err := os.MkdirAll(filepath.Dir(c.StatePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(offsets, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(c.StatePath, data, 0o600)
}

func getOptionalString(body map[string]any, key string) *string {
	value := getString(body, key)
	if value == "" {
		return nil
	}
	return &value
}

func getOptionalInt(body map[string]any, key string) *int {
	value, ok := body[key]
	if !ok || value == nil {
		return nil
	}
	parsed := getInt(body, key)
	return &parsed
}

func getFileEdits(body map[string]any, key string) []FileEdit {
	raw, ok := body[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	edits := make([]FileEdit, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		path := getString(m, "path")
		if path == "" {
			continue
		}
		edits = append(edits, FileEdit{
			Path:         path,
			EditCount:    getInt(m, "edit_count"),
			LinesAdded:   getInt(m, "lines_added"),
			LinesDeleted: getInt(m, "lines_deleted"),
		})
	}
	return edits
}

func getString(body map[string]any, key string) string {
	value, ok := body[key]
	if !ok || value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	return ""
}

func getFloat(body map[string]any, key string) float64 {
	value, ok := body[key]
	if !ok || value == nil {
		return 0
	}

	switch typed := value.(type) {
	case float64:
		return typed
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	}
	return 0
}

func getInt(body map[string]any, key string) int {
	return int(getFloat(body, key))
}

func getBool(body map[string]any, key string) bool {
	value, ok := body[key]
	if !ok || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	}
	return false
}

func getMap(body map[string]any, key string) map[string]any {
	value, ok := body[key]
	if !ok || value == nil {
		return nil
	}
	if nested, ok := value.(map[string]any); ok {
		return nested
	}
	return nil
}
