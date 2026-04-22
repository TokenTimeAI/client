package scanner

import (
	"fmt"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/collector"
)

// formatTimePtr converts a *time.Time to a *string (ISO 8601 format)
func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// ToEvent converts a ScanResult to a collector.Event for ingestion
func (r ScanResult) ToEvent() collector.Event {
	return collector.Event{
		Entity:                 r.Entity,
		Type:                   r.Type,
		Project:                r.Project,
		Branch:                 r.Branch,
		Language:               r.Language,
		AgentType:              r.AgentType,
		Time:                   r.Time,
		Duration:               r.Duration,
		SessionStartedAt:       formatTimePtr(r.SessionStartedAt),
		SessionEndedAt:         formatTimePtr(r.SessionEndedAt),
		SessionDurationSeconds: r.SessionDurationSeconds,
		AgentActiveSeconds:     r.AgentActiveSeconds,
		HumanActiveSeconds:     r.HumanActiveSeconds,
		IdleSeconds:            r.IdleSeconds,
		IsWrite:                r.IsWrite,
		TokensUsed:             r.TotalTokens,
		LinesAdded:             r.LinesAdded,
		LinesDeleted:           r.LinesDeleted,
		CostUSD:                r.CostUSD,
		Metadata:               r.Metadata,
		ConversationID:         r.ConversationID,
		MessageID:              r.MessageID,
		PromptTokens:           r.PromptTokens,
		CompletionTokens:       r.CompletionTokens,
		TotalTokens:            r.TotalTokens,
		Model:                  r.Model,
		SourceFingerprint:      scanResultFingerprint(r),
		FileEdits:              toCollectorFileEdits(r.FileEdits),
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
	}
	if r.SessionStartedAt != nil {
		parts = append(parts, r.SessionStartedAt.UTC().Format(time.RFC3339))
	}
	if r.SessionEndedAt != nil {
		parts = append(parts, r.SessionEndedAt.UTC().Format(time.RFC3339))
	} else {
		parts = append(parts, fmt.Sprintf("%.0f", r.Time))
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, ":")
}
