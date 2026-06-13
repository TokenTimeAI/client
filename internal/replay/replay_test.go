package replay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type fakeClient struct {
	importRun     api.ImportRun
	updateCalls   []api.ImportRun
	sendResult    api.BulkSendResult
	sendResults   []api.BulkSendResult
	sendCalls     int
	createErr     error
	sendErr       error
	updateErr     error
	createdRunReq api.ImportRun
}

func (f *fakeClient) CreateImportRun(_ context.Context, run api.ImportRun) (api.ImportRun, error) {
	f.createdRunReq = run
	if f.createErr != nil {
		return api.ImportRun{}, f.createErr
	}
	if f.importRun.ID == "" {
		f.importRun = api.ImportRun{ID: "run_123"}
	}
	return f.importRun, nil
}

func (f *fakeClient) UpdateImportRun(_ context.Context, run api.ImportRun) (api.ImportRun, error) {
	f.updateCalls = append(f.updateCalls, run)
	if f.updateErr != nil {
		return api.ImportRun{}, f.updateErr
	}
	return run, nil
}

func (f *fakeClient) SendHeartbeatsDetailed(_ context.Context, _ []api.Heartbeat) (api.BulkSendResult, error) {
	f.sendCalls++
	if f.sendErr != nil {
		return api.BulkSendResult{}, f.sendErr
	}
	if len(f.sendResults) >= f.sendCalls {
		return f.sendResults[f.sendCalls-1], nil
	}
	return f.sendResult, nil
}

type fakeScanner struct {
	results   []scanner.ScanResult
	err       error
	agentName string
}

func (f *fakeScanner) ScanOnce(context.Context) ([]scanner.ScanResult, error) {
	return f.results, f.err
}

func (f *fakeScanner) ScanAgent(_ context.Context, agentName string) ([]scanner.ScanResult, error) {
	f.agentName = agentName
	return f.results, f.err
}

func TestRunCompletesAndReportsSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "scanner-state.json")
	liveManager := scanner.NewStateManager(livePath)
	initialState := scanner.NewState()
	initialState.Sources["codex"] = scanner.SourceState{LastScanTime: 10, LastRecordID: "old"}
	if err := liveManager.Save(initialState); err != nil {
		t.Fatalf("save live state: %v", err)
	}

	endedAt := time.Now().UTC()
	duration := 60
	result := scanner.ScanResult{
		AgentType:              "codex",
		Entity:                 "conversation-1",
		Type:                   "conversation",
		Time:                   float64(time.Now().Unix()),
		Timestamp:              time.Now().UTC(),
		SessionEndedAt:         &endedAt,
		SessionDurationSeconds: &duration,
		SourceFingerprint:      "fp-1",
	}

	client := &fakeClient{
		importRun:  api.ImportRun{ID: "run_123"},
		sendResult: api.BulkSendResult{Responses: []api.BulkResponse{{StatusCode: 201}, {StatusCode: 200}}},
	}
	scan := &fakeScanner{results: []scanner.ScanResult{result, result}}
	var progress []Progress

	summary, err := Runner{
		ClientFactory: func(config.Config) apiClient { return client },
		ScannerFactory: func(string) scannerClient {
			return scan
		},
	}.Run(context.Background(), config.Config{
		ServerURL:   "https://ttime.ai",
		APIKey:      "key",
		MachineName: "laptop",
	}, config.Paths{
		ScannerStateFile: livePath,
	}, Options{ReplayAll: true}, func(update Progress) {
		progress = append(progress, update)
	})
	if err != nil {
		t.Fatalf("run replay: %v", err)
	}

	if summary.Scanned != 2 || summary.Imported != 1 || summary.Updated != 1 || summary.Skipped != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.ImportRun != "run_123" {
		t.Fatalf("unexpected import run id: %#v", summary)
	}
	if client.createdRunReq.TriggerKind != "replay" || client.createdRunReq.Status != "running" {
		t.Fatalf("unexpected import run create payload: %#v", client.createdRunReq)
	}
	if len(client.updateCalls) != 1 || client.updateCalls[0].Status != "completed" {
		t.Fatalf("expected completed import run update, got %#v", client.updateCalls)
	}
	if got := progress[len(progress)-1]; got.Stage != "done" || got.SessionsImport != 1 || got.SessionsUpdate != 1 {
		t.Fatalf("unexpected final progress: %#v", got)
	}
}

func TestRunMarksImportRunFailedOnUploadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "scanner-state.json")
	client := &fakeClient{
		importRun: api.ImportRun{ID: "run_456"},
		sendErr:   errors.New("upload failed"),
	}
	scan := &fakeScanner{results: []scanner.ScanResult{{AgentType: "codex", Entity: "conv", Type: "conversation", Time: 1, Timestamp: time.Now().UTC()}}}

	_, err := Runner{
		ClientFactory: func(config.Config) apiClient { return client },
		ScannerFactory: func(string) scannerClient {
			return scan
		},
	}.Run(context.Background(), config.Config{
		ServerURL:   "https://ttime.ai",
		APIKey:      "key",
		MachineName: "laptop",
	}, config.Paths{
		ScannerStateFile: livePath,
	}, Options{}, nil)
	if err == nil {
		t.Fatal("expected upload error")
	}

	if len(client.updateCalls) != 1 {
		t.Fatalf("expected failed update call, got %#v", client.updateCalls)
	}
	if client.updateCalls[0].Status != "failed" || client.updateCalls[0].SessionsSeen != 1 {
		t.Fatalf("unexpected failure update payload: %#v", client.updateCalls[0])
	}
}

func TestRunUploadsReplayInBatches(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "scanner-state.json")
	liveManager := scanner.NewStateManager(livePath)
	if err := liveManager.Save(scanner.NewState()); err != nil {
		t.Fatalf("save live state: %v", err)
	}

	results := make([]scanner.ScanResult, 205)
	for i := range results {
		results[i] = scanner.ScanResult{
			AgentType: "codex",
			Entity:    fmt.Sprintf("conversation-%d", i),
			Type:      "conversation",
			Time:      float64(i + 1),
			Timestamp: time.Now().UTC(),
		}
	}

	client := &fakeClient{
		importRun: api.ImportRun{ID: "run_batch"},
		sendResults: []api.BulkSendResult{
			{Responses: makeResponses(100, 201)},
			{Responses: makeResponses(100, 200)},
			{Responses: makeResponses(5, 201)},
		},
	}
	scan := &fakeScanner{results: results}

	summary, err := Runner{
		ClientFactory: func(config.Config) apiClient { return client },
		ScannerFactory: func(string) scannerClient {
			return scan
		},
	}.Run(context.Background(), config.Config{
		ServerURL:   "https://ttime.ai",
		APIKey:      "key",
		MachineName: "laptop",
	}, config.Paths{
		ScannerStateFile: livePath,
	}, Options{ReplayAll: true}, nil)
	if err != nil {
		t.Fatalf("run replay: %v", err)
	}

	if client.sendCalls != 3 {
		t.Fatalf("expected 3 upload calls, got %d", client.sendCalls)
	}
	if summary.Scanned != 205 || summary.Imported != 105 || summary.Updated != 100 || summary.Skipped != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestMergeScannerStateKeepsNewestCheckpoint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.json")
	tempPath := filepath.Join(dir, "temp.json")

	liveManager := scanner.NewStateManager(livePath)
	tempManager := scanner.NewStateManager(tempPath)

	liveState := scanner.NewState()
	liveState.Sources["codex"] = scanner.SourceState{LastScanTime: 10, LastRecordID: "a"}
	if err := liveManager.Save(liveState); err != nil {
		t.Fatalf("save live state: %v", err)
	}

	tempState := scanner.NewState()
	tempState.Sources["codex"] = scanner.SourceState{LastScanTime: 20, LastRecordID: "b"}
	if err := tempManager.Save(tempState); err != nil {
		t.Fatalf("save temp state: %v", err)
	}

	if err := mergeScannerState(livePath, tempPath); err != nil {
		t.Fatalf("merge scanner state: %v", err)
	}

	merged, err := liveManager.Load()
	if err != nil {
		t.Fatalf("load merged state: %v", err)
	}
	if merged.Sources["codex"].LastScanTime != 20 {
		t.Fatalf("expected latest scan time to win, got %#v", merged.Sources["codex"])
	}
}

func TestMergeScannerStateCreatesLiveStateIfMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "missing-live.json")
	tempPath := filepath.Join(dir, "temp.json")

	tempManager := scanner.NewStateManager(tempPath)
	tempState := scanner.NewState()
	tempState.Sources["claude_code"] = scanner.SourceState{LastScanTime: 30, LastRecordID: "session-1"}
	if err := tempManager.Save(tempState); err != nil {
		t.Fatalf("save temp state: %v", err)
	}

	if err := mergeScannerState(livePath, tempPath); err != nil {
		t.Fatalf("merge scanner state: %v", err)
	}
	if _, err := os.Stat(livePath); err != nil {
		t.Fatalf("expected live state file to be created: %v", err)
	}
}

func makeResponses(count int, status int) []api.BulkResponse {
	responses := make([]api.BulkResponse, count)
	for i := range responses {
		responses[i] = api.BulkResponse{StatusCode: status}
	}
	return responses
}
