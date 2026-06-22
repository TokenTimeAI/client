package scanner

import (
	"fmt"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/collector"
)

// ToEvent converts a ScanResult to a collector.Event for ingestion
func (r ScanResult) ToEvent() collector.Event {
	return collector.Event{
		Entity:               r.Entity,
		Type:                 r.Type,
		Project:              r.Project,
		Branch:               r.Branch,
		Language:             r.Language,
		AgentType:            r.AgentType,
		Time:                 r.Time,
		Duration:             r.Duration,
		IsWrite:              r.IsWrite,
		TokensUsed:           r.TotalTokens,
		LinesAdded:           r.LinesAdded,
		LinesDeleted:         r.LinesDeleted,
		CostUSD:              r.CostUSD,
		Metadata:             metadataWithGitContext(r),
		ConversationID:       r.ConversationID,
		MessageID:            r.MessageID,
		PromptTokens:         r.PromptTokens,
		CompletionTokens:     r.CompletionTokens,
		CachedTokens:         r.CachedTokens,
		CacheCreationTokens:  r.CacheCreationTokens,
		ReasoningTokens:      r.ReasoningTokens,
		TotalTokens:          r.TotalTokens,
		Model:                r.Model,
		SourceFingerprint:    scanResultFingerprint(r),
		FileEdits:            toCollectorFileEdits(r.FileEdits),
	}
}

// ToEvents converts multiple ScanResults to collector.Events
func ToEvents(results []ScanResult) []collector.Event {
	events := make([]collector.Event, len(results))
	for i, r := range results {
		events[i] = r.ToEvent()
	}
	return events
}

func toCollectorFileEdits(raw []FileEdit) []collector.FileEdit {
	if len(raw) == 0 {
		return nil
	}
	edits := make([]collector.FileEdit, 0, len(raw))
	for _, edit := range raw {
		edits = append(edits, collector.FileEdit{
			Path:         edit.Path,
			EditCount:    edit.EditCount,
			LinesAdded:   edit.LinesAdded,
			LinesDeleted: edit.LinesDeleted,
		})
	}
	return edits
}

func scanResultFingerprint(r ScanResult) string {
	if strings.TrimSpace(r.SourceFingerprint) != "" {
		return strings.TrimSpace(r.SourceFingerprint)
	}
	parts := []string{
		strings.TrimSpace(r.AgentType),
		strings.TrimSpace(r.ConversationID),
		strings.TrimSpace(r.MessageID),
		strings.TrimSpace(r.Entity),
		fmt.Sprintf("%.0f", r.Time),
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, ":")
}
