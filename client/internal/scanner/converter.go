package scanner

import (
	"github.com/ttime-ai/ttime/client/internal/collector"
)

// ToEvent converts a ScanResult to a collector.Event for ingestion
func (r ScanResult) ToEvent() collector.Event {
	return collector.Event{
		Entity:           r.Entity,
		Type:             r.Type,
		Project:          r.Project,
		Branch:           r.Branch,
		Language:         r.Language,
		AgentType:        r.AgentType,
		Time:             r.Time,
		Duration:         r.Duration,
		IsWrite:          r.IsWrite,
		TokensUsed:       r.TotalTokens,
		LinesAdded:       r.LinesAdded,
		LinesDeleted:     r.LinesDeleted,
		CostUSD:          r.CostUSD,
		Metadata:         r.Metadata,
		ConversationID:   r.ConversationID,
		MessageID:        r.MessageID,
		PromptTokens:     r.PromptTokens,
		CompletionTokens: r.CompletionTokens,
		TotalTokens:      r.TotalTokens,
		Model:            r.Model,
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