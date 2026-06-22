package normalize

import (
	"runtime"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/collector"
)

type Options struct {
	MachineName string
}

func Event(raw collector.Event, opts Options) api.Heartbeat {
	eventType := raw.Type
	if eventType == "" {
		eventType = "file"
	}

	return api.Heartbeat{
		Entity:              raw.Entity,
		Type:                eventType,
		Project:             raw.Project,
		Branch:              raw.Branch,
		Language:            raw.Language,
		AgentType:           raw.AgentType,
		Time:                raw.Time,
		Duration:            raw.Duration,
		IsWrite:             raw.IsWrite,
		TokensUsed:          raw.TokensUsed,
		LinesAdded:          raw.LinesAdded,
		LinesDeleted:        raw.LinesDeleted,
		CostUSD:             raw.CostUSD,
		Metadata:            raw.Metadata,
		Machine:             opts.MachineName,
		OperatingSystem:     runtime.GOOS,
		ConversationID:      raw.ConversationID,
		MessageID:           raw.MessageID,
		PromptTokens:        raw.PromptTokens,
		CompletionTokens:    raw.CompletionTokens,
		CachedTokens:        raw.CachedTokens,
		CacheCreationTokens: raw.CacheCreationTokens,
		ReasoningTokens:     raw.ReasoningTokens,
		TotalTokens:         raw.TotalTokens,
		Model:               raw.Model,
		ImportRunID:         raw.ImportRunID,
		SourceFingerprint:   raw.SourceFingerprint,
		FileEdits:           normalizeFileEdits(raw.FileEdits),
	}
}

func normalizeFileEdits(raw []collector.FileEdit) []api.FileEdit {
	if len(raw) == 0 {
		return nil
	}
	edits := make([]api.FileEdit, 0, len(raw))
	for _, edit := range raw {
		edits = append(edits, api.FileEdit{
			Path:         edit.Path,
			EditCount:    edit.EditCount,
			LinesAdded:   edit.LinesAdded,
			LinesDeleted: edit.LinesDeleted,
		})
	}
	return edits
}
