package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/normalize"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func runImport(ctx context.Context, paths config.Paths, args []string) int {
	if len(args) == 0 || args[0] != "replay" {
		fmt.Fprintf(os.Stderr, "usage: ttime import replay [--all] [--agent <name>]\n")
		return 1
	}

	flags := flag.NewFlagSet("import replay", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	replayAll := flags.Bool("all", false, "replay all detectable native-agent sessions")
	agentFilter := flags.String("agent", "", "replay only one agent")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import replay requires a configured client, run `ttime setup`: %v\n", err)
		return 1
	}

	client := api.NewClient(cfg.ServerURL, cfg.APIKey)
	importRun, err := client.CreateImportRun(ctx, api.ImportRun{
		Machine:      cfg.MachineName,
		TriggerKind:  "replay",
		Status:       "running",
		ReplayAll:    *replayAll || *agentFilter == "",
		AgentFilters: selectedAgents(*agentFilter),
		StartedAt:    time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create import run: %v\n", err)
		return 1
	}

	tempStatePath := filepath.Join(os.TempDir(), fmt.Sprintf("ttime-import-replay-%d.json", time.Now().UnixNano()))
	defer os.Remove(tempStatePath)

	scan := scanner.New(tempStatePath, 5*time.Minute)
	var results []scanner.ScanResult
	if *agentFilter != "" {
		results, err = scan.ScanAgent(ctx, *agentFilter)
	} else {
		results, err = scan.ScanOnce(ctx)
	}
	if err != nil {
		failImportRun(ctx, client, importRun, 0, err)
		fmt.Fprintf(os.Stderr, "replay scan failed: %v\n", err)
		return 1
	}

	heartbeats := make([]api.Heartbeat, 0, len(results))
	for _, result := range results {
		event := result.ToEvent()
		event.ImportRunID = importRun.ID
		heartbeat := normalize.Event(event, normalize.Options{MachineName: cfg.MachineName})
		heartbeat.ImportRunID = importRun.ID
		heartbeats = append(heartbeats, heartbeat)
	}

	sendResult, err := client.SendHeartbeatsDetailed(ctx, heartbeats)
	if err != nil {
		failImportRun(ctx, client, importRun, len(results), err)
		fmt.Fprintf(os.Stderr, "replay upload failed: %v\n", err)
		return 1
	}

	imported, updated := summarizeBulkResponses(sendResult)
	_, err = client.UpdateImportRun(ctx, api.ImportRun{
		ID:               importRun.ID,
		Status:           "completed",
		SessionsSeen:     len(results),
		SessionsImported: imported,
		SessionsUpdated:  updated,
		SessionsSkipped:  max(0, len(results)-imported-updated),
		CompletedAt:      timePtr(time.Now().UTC()),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: replay succeeded but import run update failed: %v\n", err)
	}

	if err := mergeScannerState(paths.ScannerStateFile, tempStatePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: replay uploaded but scanner state merge failed: %v\n", err)
	}

	fmt.Printf("replayed: scanned=%d imported=%d updated=%d skipped=%d import_run=%s\n",
		len(results), imported, updated, max(0, len(results)-imported-updated), importRun.ID)
	return 0
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

func failImportRun(ctx context.Context, client *api.Client, run api.ImportRun, seen int, err error) {
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
