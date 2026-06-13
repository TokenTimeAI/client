package detectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type claudeAIExportConversation struct {
	UUID      string                  `json:"uuid"`
	Name      string                  `json:"name"`
	CreatedAt string                  `json:"created_at"`
	UpdatedAt string                  `json:"updated_at"`
	Messages  []claudeAIExportMessage `json:"chat_messages"`
}

type claudeAIExportMessage struct {
	Sender    string `json:"sender"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type chatGPTExportConversation struct {
	ID          string                       `json:"conversation_id"`
	RawID       string                       `json:"id"`
	Title       string                       `json:"title"`
	CreateTime  *float64                     `json:"create_time"`
	UpdateTime  *float64                     `json:"update_time"`
	CurrentNode string                       `json:"current_node"`
	Mapping     map[string]chatGPTExportNode `json:"mapping"`
}

type chatGPTExportNode struct {
	ID      string                `json:"id"`
	Parent  *string               `json:"parent"`
	Message *chatGPTExportMessage `json:"message"`
}

type chatGPTExportMessage struct {
	CreateTime *float64 `json:"create_time"`
	Author     struct {
		Role string `json:"role"`
	} `json:"author"`
	Metadata struct {
		ModelSlug string `json:"model_slug"`
	} `json:"metadata"`
}

func collectClaudeAIExportSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	path := filepath.Join(root, "conversations.json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open claude.ai export: %w", err)
	}
	defer file.Close()

	var conversations []claudeAIExportConversation
	if err := json.NewDecoder(file).Decode(&conversations); err != nil {
		return nil, fmt.Errorf("decode claude.ai export: %w", err)
	}
	summaries := make([]agentsViewGenericSummary, 0, len(conversations))
	for _, conversation := range conversations {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if summary, ok := summarizeClaudeAIConversation(conversation); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectChatGPTExportSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := filepath.Glob(filepath.Join(root, "conversations-*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob chatgpt exports: %w", err)
	}
	single := filepath.Join(root, "conversations.json")
	if len(paths) == 0 && scanner.FileExists(single) {
		paths = []string{single}
	}
	sort.Strings(paths)

	var summaries []agentsViewGenericSummary
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		fileSummaries, err := readChatGPTExportFile(path)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, fileSummaries...)
	}
	return summaries, nil
}

func readChatGPTExportFile(path string) ([]agentsViewGenericSummary, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open chatgpt export: %w", err)
	}
	defer file.Close()

	var conversations []chatGPTExportConversation
	if err := json.NewDecoder(file).Decode(&conversations); err != nil {
		return nil, fmt.Errorf("decode chatgpt export: %w", err)
	}
	summaries := make([]agentsViewGenericSummary, 0, len(conversations))
	for _, conversation := range conversations {
		if summary, ok := summarizeChatGPTConversation(conversation); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func summarizeClaudeAIConversation(conversation claudeAIExportConversation) (agentsViewGenericSummary, bool) {
	if conversation.UUID == "" || len(conversation.Messages) == 0 {
		return agentsViewGenericSummary{}, false
	}
	startedAt := parseRFC3339Any(conversation.CreatedAt)
	endedAt := parseRFC3339Any(conversation.UpdatedAt)
	for _, message := range conversation.Messages {
		ts := parseRFC3339Any(message.CreatedAt)
		if ts.IsZero() {
			continue
		}
		if startedAt.IsZero() || ts.Before(startedAt) {
			startedAt = ts
		}
		if endedAt.IsZero() || ts.After(endedAt) {
			endedAt = ts
		}
	}
	if endedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return agentsViewGenericSummary{
		SessionID: conversation.UUID,
		Project:   "claude.ai",
		StartedAt: startedAt,
		EndedAt:   endedAt,
		FileEdits: make(map[string]scanner.FileEdit),
	}, true
}

func summarizeChatGPTConversation(conversation chatGPTExportConversation) (agentsViewGenericSummary, bool) {
	sessionID := conversation.ID
	if sessionID == "" {
		sessionID = conversation.RawID
	}
	if sessionID == "" || len(conversation.Mapping) == 0 {
		return agentsViewGenericSummary{}, false
	}

	nodes := chatGPTLinearizedNodes(conversation)
	if len(nodes) == 0 {
		return agentsViewGenericSummary{}, false
	}

	startedAt := unixFloatPtrToTime(conversation.CreateTime)
	endedAt := unixFloatPtrToTime(conversation.UpdateTime)
	var model string
	hasContent := false
	for _, node := range nodes {
		if node.Message == nil {
			continue
		}
		hasContent = true
		ts := unixFloatPtrToTime(node.Message.CreateTime)
		if !ts.IsZero() {
			if startedAt.IsZero() || ts.Before(startedAt) {
				startedAt = ts
			}
			if endedAt.IsZero() || ts.After(endedAt) {
				endedAt = ts
			}
		}
		if model == "" {
			model = node.Message.Metadata.ModelSlug
		}
	}
	if !hasContent || endedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return agentsViewGenericSummary{
		SessionID: sessionID,
		Project:   "chatgpt.com",
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Model:     model,
		FileEdits: make(map[string]scanner.FileEdit),
	}, true
}

func chatGPTLinearizedNodes(conversation chatGPTExportConversation) []chatGPTExportNode {
	if conversation.CurrentNode == "" {
		return nil
	}
	var nodes []chatGPTExportNode
	for nodeID := conversation.CurrentNode; nodeID != ""; {
		node, ok := conversation.Mapping[nodeID]
		if !ok {
			break
		}
		nodes = append(nodes, node)
		if node.Parent == nil {
			break
		}
		nodeID = *node.Parent
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return nodes
}

func unixFloatPtrToTime(value *float64) time.Time {
	if value == nil || *value <= 0 {
		return time.Time{}
	}
	return time.Unix(int64(*value), 0).UTC()
}
