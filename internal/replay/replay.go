package replay

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/normalize"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type Options struct {
	ReplayAll   bool
	AgentFilter string
}

type Progress struct {
	Stage          string
	Message        string
	SessionsSeen   int
	SessionsImport int
	SessionsUpdate int
	SessionsSkip   int
	ImportRunID    string
}

type Summary struct {
	Scanned   int
	Imported  int
	Updated   int
	Skipped   int
	ImportRun string
}

const replayUploadBatchSize = 100

type Reporter func(Progress)

type scannerClient interface {
	ScanOnce(ctx context.Context) ([]scanner.ScanResult, error)
	ScanAgent(ctx context.Context, agentName string) ([]scanner.ScanResult, error)
}

type apiClient interface {
	CreateImportRun(ctx context.Context, run api.ImportRun) (api.ImportRun, error)
	UpdateImportRun(ctx context.Context, run api.ImportRun) (api.ImportRun, error)
	SendHeartbeatsDetailed(ctx context.Context, heartbeats []api.Heartbeat) (api.BulkSendResult, error)
}

type Runner struct {
	ClientFactory  func(config.Config) apiClient
	ScannerFactory func(statePath string) scannerClient
}

func NewRunner() Runner {
	return Runner{
		ClientFactory: func(cfg config.Config) apiClient {
			return api.NewClient(cfg.ServerURL, cfg.APIKey)
		},
		ScannerFactory: func(statePath string) scannerClient {
			return scanner.New(statePath, 5*time.Minute)
		},
	}
}

func Run(ctx context.Context, cfg config.Config, paths config.Paths, options Options, report Reporter) (Summary, error) {
	return NewRunner().Run(ctx, cfg, paths, options, report)
}

func (r Runner) Run(ctx context.Context, cfg config.Config, paths config.Paths, options Options, report Reporter) (Summary, error) {
	clientFactory := r.ClientFactory
	if clientFactory == nil {
		clientFactory = NewRunner().ClientFactory
	}
	scannerFactory := r.ScannerFactory
	if scannerFactory == nil {
		scannerFactory = NewRunner().ScannerFactory
	}

	client := clientFactory(cfg)

	emit(report, Progress{Stage: "create-run", Message: "Creating import run"})
	importRun, err := client.CreateImportRun(ctx, api.ImportRun{
		Machine:      cfg.MachineName,
		TriggerKind:  "replay",
		Status:       "running",
		ReplayAll:    options.ReplayAll || options.AgentFilter == "",
		AgentFilters: selectedAgents(options.AgentFilter),
		StartedAt:    time.Now().UTC(),
	})
	if err != nil {
		return Summary{}, err
	}

	tempStatePath := filepath.Join(os.TempDir(), fmt.Sprintf("ttime-import-replay-%d.json", time.Now().UnixNano()))
	defer os.Remove(tempStatePath)

	scan := scannerFactory(tempStatePath)
	emit(report, Progress{
		Stage:       "scan",
		Message:     scanMessage(options.AgentFilter),
		ImportRunID: importRun.ID,
	})

	var results []scanner.ScanResult
	if options.AgentFilter != "" {
		results, err = scan.ScanAgent(ctx, options.AgentFilter)
	} else {
		results, err = scan.ScanOnce(ctx)
	}
	if err != nil {
		failImportRun(ctx, client, importRun, 0, err)
		return Summary{}, err
	}

	emit(report, Progress{
		Stage:        "prepare",
		Message:      "Preparing heartbeats",
		SessionsSeen: len(results),
		ImportRunID:  importRun.ID,
	})

	heartbeats := make([]api.Heartbeat, 0, len(results))
	for _, result := range results {
		event := result.ToEvent()
		event.ImportRunID = importRun.ID
		heartbeat := normalize.Event(event, normalize.Options{MachineName: cfg.MachineName})
		heartbeat.ImportRunID = importRun.ID
		heartbeats = append(heartbeats, heartbeat)
	}

	emit(report, Progress{
		Stage:        "upload",
		Message:      "Uploading replayed sessions",
		SessionsSeen: len(results),
		ImportRunID:  importRun.ID,
	})

	sendResult, err := sendHeartbeatsInBatches(ctx, client, heartbeats, importRun.ID, report)
	if err != nil {
		failImportRun(ctx, client, importRun, len(results), err)
		return Summary{}, err
	}

	imported, updated := summarizeBulkResponses(sendResult)
	skipped := max(0, len(results)-imported-updated)

	emit(report, Progress{
		Stage:          "finalize",
		Message:        "Finalizing import run",
		SessionsSeen:   len(results),
		SessionsImport: imported,
		SessionsUpdate: updated,
		SessionsSkip:   skipped,
		ImportRunID:    importRun.ID,
	})

	_, err = client.UpdateImportRun(ctx, api.ImportRun{
		ID:               importRun.ID,
		Status:           "completed",
		SessionsSeen:     len(results),
		SessionsImported: imported,
		SessionsUpdated:  updated,
		SessionsSkipped:  skipped,
		CompletedAt:      timePtr(time.Now().UTC()),
	})
	if err != nil {
		return Summary{}, fmt.Errorf("replay succeeded but import run update failed: %w", err)
	}

	emit(report, Progress{
		Stage:          "merge-state",
		Message:        "Merging scanner state",
		SessionsSeen:   len(results),
		SessionsImport: imported,
		SessionsUpdate: updated,
		SessionsSkip:   skipped,
		ImportRunID:    importRun.ID,
	})

	if err := mergeScannerState(paths.ScannerStateFile, tempStatePath); err != nil {
		return Summary{}, fmt.Errorf("replay uploaded but scanner state merge failed: %w", err)
	}

	summary := Summary{
		Scanned:   len(results),
		Imported:  imported,
		Updated:   updated,
		Skipped:   skipped,
		ImportRun: importRun.ID,
	}
	emit(report, Progress{
		Stage:          "done",
		Message:        "Replay complete",
		SessionsSeen:   summary.Scanned,
		SessionsImport: summary.Imported,
		SessionsUpdate: summary.Updated,
		SessionsSkip:   summary.Skipped,
		ImportRunID:    summary.ImportRun,
	})
	return summary, nil
}

func emit(report Reporter, progress Progress) {
	if report != nil {
		report(progress)
	}
}

func scanMessage(agentFilter string) string {
	if agentFilter == "" {
		return "Scanning all detected agents"
	}
	return fmt.Sprintf("Scanning %s sessions", agentFilter)
}

func selectedAgents(agentFilter string) []string {
	if agentFilter == "" {
		return nil
	}
	return []string{agentFilter}
}

func summarizeBulkResponses(result api.BulkSendResult) (imported int, updated int) {
	for _, response := range result.Responses {
		switch response.StatusCode {
		case 201:
			imported++
		case 200:
			updated++
		}
	}
	return imported, updated
}

func sendHeartbeatsInBatches(ctx context.Context, client apiClient, heartbeats []api.Heartbeat, importRunID string, report Reporter) (api.BulkSendResult, error) {
	if len(heartbeats) == 0 {
		return api.BulkSendResult{}, nil
	}

	totalBatches := (len(heartbeats) + replayUploadBatchSize - 1) / replayUploadBatchSize
	aggregate := api.BulkSendResult{Responses: make([]api.BulkResponse, 0, len(heartbeats))}

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * replayUploadBatchSize
		end := min(start+replayUploadBatchSize, len(heartbeats))

		emit(report, Progress{
			Stage:        "upload",
			Message:      fmt.Sprintf("Uploading replayed sessions (%d/%d)", batchIndex+1, totalBatches),
			SessionsSeen: len(heartbeats),
			ImportRunID:  importRunID,
		})

		result, err := client.SendHeartbeatsDetailed(ctx, heartbeats[start:end])
		if err != nil {
			return api.BulkSendResult{}, err
		}
		aggregate.Responses = append(aggregate.Responses, result.Responses...)

		imported, updated := summarizeBulkResponses(aggregate)
		emit(report, Progress{
			Stage:          "upload",
			Message:        fmt.Sprintf("Uploaded %d/%d replayed sessions", end, len(heartbeats)),
			SessionsSeen:   len(heartbeats),
			SessionsImport: imported,
			SessionsUpdate: updated,
			SessionsSkip:   max(0, end-imported-updated),
			ImportRunID:    importRunID,
		})
	}

	return aggregate, nil
}

func failImportRun(ctx context.Context, client apiClient, run api.ImportRun, seen int, err error) {
	_, _ = client.UpdateImportRun(ctx, api.ImportRun{
		ID:           run.ID,
		Status:       "failed",
		SessionsSeen: seen,
		CompletedAt:  timePtr(time.Now().UTC()),
		ErrorSummary: err.Error(),
	})
}

func mergeScannerState(livePath, tempPath string) error {
	liveManager := scanner.NewStateManager(livePath)
	tempManager := scanner.NewStateManager(tempPath)
	liveState, err := liveManager.Load()
	if err != nil {
		return err
	}
	tempState, err := tempManager.Load()
	if err != nil {
		return err
	}
	for source, candidate := range tempState.Sources {
		current := liveState.Sources[source]
		if shouldReplaceState(current, candidate) {
			liveState.Sources[source] = candidate
		}
	}
	return liveManager.Save(liveState)
}

func shouldReplaceState(current, candidate scanner.SourceState) bool {
	if candidate.LastScanTime > current.LastScanTime {
		return true
	}
	if candidate.LastScanTime == current.LastScanTime && candidate.LastRecordID > current.LastRecordID {
		return true
	}
	return false
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
