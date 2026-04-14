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
		Entity:          raw.Entity,
		Type:            eventType,
		Project:         raw.Project,
		Branch:          raw.Branch,
		Language:        raw.Language,
		AgentType:       raw.AgentType,
		Time:            raw.Time,
		IsWrite:         raw.IsWrite,
		TokensUsed:      raw.TokensUsed,
		LinesAdded:      raw.LinesAdded,
		LinesDeleted:    raw.LinesDeleted,
		CostUSD:         raw.CostUSD,
		Metadata:        raw.Metadata,
		Machine:         opts.MachineName,
		OperatingSystem: runtime.GOOS,
	}
}
