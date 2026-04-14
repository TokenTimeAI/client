package service

import (
	"context"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/collector"
	"github.com/ttime-ai/ttime/client/internal/normalize"
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

type Daemon struct {
	Collector    Collector
	Queue        Queue
	Sender       Sender
	MachineName  string
	PollInterval time.Duration
}

type Result struct {
	QueuedPreviously int
	Collected        int
	Sent             int
}

func (d *Daemon) RunOnce(ctx context.Context) (Result, error) {
	queued, err := d.Queue.ReadAll()
	if err != nil {
		return Result{}, err
	}

	collectedRaw, err := d.Collector.Collect(ctx)
	if err != nil {
		return Result{}, err
	}

	collected := make([]api.Heartbeat, 0, len(collectedRaw))
	for _, raw := range collectedRaw {
		collected = append(collected, normalize.Event(raw, normalize.Options{
			MachineName: d.MachineName,
		}))
	}

	batch := append(append([]api.Heartbeat{}, queued...), collected...)
	result := Result{
		QueuedPreviously: len(queued),
		Collected:        len(collected),
		Sent:             0,
	}

	if len(batch) == 0 {
		return result, nil
	}

	if err := d.Sender.SendHeartbeats(ctx, batch); err != nil {
		if len(collected) > 0 {
			if appendErr := d.Queue.Append(collected); appendErr != nil {
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
