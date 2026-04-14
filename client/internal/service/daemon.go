package service

import (
	"context"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/collector"
	"github.com/ttime-ai/ttime/client/internal/normalize"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type Collector interface {
	Collect(context.Context) ([]collector.Event, error)
}

type Queue interface {
	ReadAll() ([]api.Heartbeat, error)
	Append([]api.Heartbeat) error
	Clear() error
}

type Sender interface {
	SendHeartbeats(context.Context, []api.Heartbeat) error
}

type AgentScanner interface {
	ScanOnce(context.Context) ([]scanner.ScanResult, error)
}

type Daemon struct {
	Collector    Collector
	Queue        Queue
	Sender       Sender
	MachineName  string
	PollInterval time.Duration
	Scanner      AgentScanner
}

type Result struct {
	QueuedPreviously int
	Collected        int
	Scanned          int
	Sent             int
}

func (d *Daemon) RunOnce(ctx context.Context) (Result, error) {
	queued, err := d.Queue.ReadAll()
	if err != nil {
		return Result{}, err
	}

	// Collect from JSONL inbox
	collectedRaw, err := d.Collector.Collect(ctx)
	if err != nil {
		return Result{}, err
	}

	// Normalize collected events
	events := make([]api.Heartbeat, 0, len(collectedRaw))
	for _, raw := range collectedRaw {
		events = append(events, normalize.Event(raw, normalize.Options{
			MachineName: d.MachineName,
		}))
	}

	// Scan agent databases if scanner is configured
	var scannedEvents []api.Heartbeat
	if d.Scanner != nil {
		scanResults, err := d.Scanner.ScanOnce(ctx)
		if err == nil {
			// Convert scan results to heartbeats
			for _, result := range scanResults {
				event := result.ToEvent()
				scannedEvents = append(scannedEvents, normalize.Event(event, normalize.Options{
					MachineName: d.MachineName,
				}))
			}
		}
	}

	// Combine all sources
	batch := append(append(append([]api.Heartbeat{}, queued...), events...), scannedEvents...)
	result := Result{
		QueuedPreviously: len(queued),
		Collected:        len(events),
		Scanned:          len(scannedEvents),
		Sent:             0,
	}

	if len(batch) == 0 {
		return result, nil
	}

	if err := d.Sender.SendHeartbeats(ctx, batch); err != nil {
		// Queue only the new events (not previously queued) for retry
		newEvents := append(events, scannedEvents...)
		if len(newEvents) > 0 {
			if appendErr := d.Queue.Append(newEvents); appendErr != nil {
				return result, appendErr
			}
		}
		return result, err
	}

	if err := d.Queue.Clear(); err != nil {
		return result, err
	}

	result.Sent = len(batch)
	return result, nil
}

func (d *Daemon) RunLoop(ctx context.Context) error {
	interval := d.PollInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	if _, err := d.RunOnce(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := d.RunOnce(ctx); err != nil {
				return err
			}
		}
	}
}
