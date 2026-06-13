package detectors

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	results, newState, err := d.scanCurrentChats(ctx, state)
	if err != nil {
		return nil, state, err
	}
	legacyResults, legacyState, err := d.scanHistory(ctx, state)
	if err != nil {
		return nil, state, err
	}
	results = append(results, legacyResults...)
	if legacyState.LastScanTime > newState.LastScanTime ||
		(legacyState.LastScanTime == newState.LastScanTime && legacyState.LastRecordID > newState.LastRecordID) {
		newState = legacyState
	}
	return results, newState, nil
}

func (d *GeminiCLIDetector) scanHistory(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
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
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}
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
				ID               string `json:"id"`
				Role             string `json:"role"`
				Timestamp        int64  `json:"timestamp"`
				PromptTokens     int    `json:"input_tokens"`
				CompletionTokens int    `json:"output_tokens"`
				TotalTokens      int    `json:"total_tokens"`
				Model            string `json:"model"`
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

func (d *GeminiCLIDetector) scanCurrentChats(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	files, err := geminiCurrentChatFiles(ctx, d.configDir)
	if err != nil {
		return nil, state, err
	}
	projects := geminiProjectMap(d.configDir)

	var results []scanner.ScanResult
	newState := state
	for _, path := range files {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}
		session, ok := parseGeminiCurrentSession(path)
		if !ok {
			continue
		}
		projectKey := filepath.Base(filepath.Dir(filepath.Dir(path)))
		project := projects[projectKey]
		if project.Path == "" {
			project.Path = projectKey
			project.Name = projectNameFromPath(projectKey)
		}
		for _, msg := range session.Messages {
			if msg.Type != "gemini" && msg.Type != "assistant" && msg.Type != "model" {
				continue
			}
			timestamp := parseRFC3339Any(msg.Timestamp)
			if timestamp.IsZero() {
				timestamp = parseRFC3339Any(session.LastUpdated)
			}
			if timestamp.IsZero() {
				continue
			}
			endUnix := timestamp.Unix()
			if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && msg.ID <= state.LastRecordID) {
				continue
			}
			promptTokens := msg.Tokens.Input + msg.Tokens.Cached
			completionTokens := msg.Tokens.Output + msg.Tokens.Thoughts
			fileEdits := make(map[string]scanner.FileEdit)
			for _, call := range msg.ToolCalls {
				mergeFileEdits(fileEdits, fileEditsFromToolCall(call.Name, call.Args))
			}

			startedAt := parseRFC3339Any(session.StartTime)
			endedAt := parseRFC3339Any(session.LastUpdated)
			if startedAt.IsZero() {
				startedAt = timestamp
			}
			if endedAt.IsZero() {
				endedAt = timestamp
			}
			result := scanner.ScanResult{
				AgentType:              "gemini_cli",
				Type:                   "conversation",
				Entity:                 project.Path,
				Time:                   float64(endUnix),
				Timestamp:              timestamp,
				ConversationID:         session.SessionID,
				MessageID:              msg.ID,
				PromptTokens:           promptTokens,
				CompletionTokens:       completionTokens,
				TotalTokens:            promptTokens + completionTokens,
				Model:                  msg.Model,
				FileEdits:              flattenFileEdits(fileEdits),
				Project:                project.Name,
				SessionStartedAt:       timePtr(startedAt),
				SessionEndedAt:         timePtr(endedAt),
				SessionDurationSeconds: intPtr(durationSeconds(startedAt, endedAt)),
				Metadata: map[string]any{
					"parser": "gemini_current_chats",
				},
			}
			results = append(results, result)
			if endUnix > newState.LastScanTime || (endUnix == newState.LastScanTime && msg.ID > newState.LastRecordID) {
				newState.LastScanTime = endUnix
				newState.LastRecordID = msg.ID
			}
		}
	}
	return results, newState, nil
}

type geminiProjectRef struct {
	Path string
	Name string
}

type geminiCurrentSession struct {
	SessionID   string                 `json:"sessionId"`
	StartTime   string                 `json:"startTime"`
	LastUpdated string                 `json:"lastUpdated"`
	Messages    []geminiCurrentMessage `json:"messages"`
}

type geminiCurrentMessage struct {
	ID        string                  `json:"id"`
	Type      string                  `json:"type"`
	Content   any                     `json:"content"`
	Timestamp string                  `json:"timestamp"`
	Model     string                  `json:"model"`
	Tokens    geminiCurrentTokens     `json:"tokens"`
	ToolCalls []geminiCurrentToolCall `json:"toolCalls"`
}

type geminiCurrentTokens struct {
	Input    int `json:"input"`
	Output   int `json:"output"`
	Cached   int `json:"cached"`
	Thoughts int `json:"thoughts"`
}

type geminiCurrentToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func geminiCurrentChatFiles(ctx context.Context, root string) ([]string, error) {
	tmpRoot := filepath.Join(root, "tmp")
	hashDirs, err := os.ReadDir(tmpRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, hashDir := range hashDirs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if !hashDir.IsDir() {
			continue
		}
		chatsDir := filepath.Join(tmpRoot, hashDir.Name(), "chats")
		entries, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "session-") {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".json" || ext == ".jsonl" {
				paths = append(paths, filepath.Join(chatsDir, entry.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func parseGeminiCurrentSession(path string) (geminiCurrentSession, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return geminiCurrentSession{}, false
	}
	var session geminiCurrentSession
	if strings.EqualFold(filepath.Ext(path), ".json") {
		if err := json.Unmarshal(data, &session); err != nil {
			return geminiCurrentSession{}, false
		}
		return session, session.SessionID != ""
	}

	lineScanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	seen := make(map[string]int)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var record geminiCurrentMessage
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		var meta struct {
			SessionID   string `json:"sessionId"`
			StartTime   string `json:"startTime"`
			LastUpdated string `json:"lastUpdated"`
			Set         struct {
				LastUpdated string `json:"lastUpdated"`
			} `json:"$set"`
		}
		_ = json.Unmarshal([]byte(line), &meta)
		if session.SessionID == "" {
			session.SessionID = meta.SessionID
		}
		if session.StartTime == "" {
			session.StartTime = meta.StartTime
		}
		if meta.LastUpdated != "" {
			session.LastUpdated = meta.LastUpdated
		}
		if meta.Set.LastUpdated != "" {
			session.LastUpdated = meta.Set.LastUpdated
		}
		if record.Type != "user" && record.Type != "gemini" {
			continue
		}
		if record.ID != "" {
			if idx, ok := seen[record.ID]; ok {
				session.Messages[idx] = record
				continue
			}
			seen[record.ID] = len(session.Messages)
		}
		session.Messages = append(session.Messages, record)
	}
	return session, session.SessionID != ""
}

func geminiProjectMap(root string) map[string]geminiProjectRef {
	projects := make(map[string]geminiProjectRef)
	add := func(path, name string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if name = strings.TrimSpace(name); name == "" {
			name = projectNameFromPath(path)
		}
		projects[geminiProjectHash(path)] = geminiProjectRef{Path: path, Name: name}
	}

	var projectFile struct {
		Projects map[string]string `json:"projects"`
	}
	if data, err := os.ReadFile(filepath.Join(root, "projects.json")); err == nil {
		if err := json.Unmarshal(data, &projectFile); err == nil {
			for path, name := range projectFile.Projects {
				add(path, name)
			}
		}
	}
	var trustedFile struct {
		TrustedFolders []string `json:"trustedFolders"`
	}
	if data, err := os.ReadFile(filepath.Join(root, "trustedFolders.json")); err == nil {
		if err := json.Unmarshal(data, &trustedFile); err == nil {
			for _, path := range trustedFile.TrustedFolders {
				add(path, "")
			}
		}
	}
	return projects
}

func geminiProjectHash(path string) string {
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", sum)
}

func init() {
	scanner.Register(NewGeminiCLIDetector)
}
