package config_test

import (
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/config"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	cfg := config.Config{
		ServerURL:           "https://ttime.example",
		APIKey:              "tt_test",
		InboxDir:            filepath.Join(tempDir, "inbox"),
		PollIntervalSeconds: 15,
		MachineName:         "devbox",
		AuthenticatedEmail:  "dev@example.com",
	}

	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.ServerURL != cfg.ServerURL {
		t.Fatalf("server URL mismatch: got %q want %q", loaded.ServerURL, cfg.ServerURL)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Fatalf("API key mismatch: got %q want %q", loaded.APIKey, cfg.APIKey)
	}
	if loaded.InboxDir != cfg.InboxDir {
		t.Fatalf("inbox dir mismatch: got %q want %q", loaded.InboxDir, cfg.InboxDir)
	}
	if loaded.PollIntervalSeconds != cfg.PollIntervalSeconds {
		t.Fatalf("poll interval mismatch: got %d want %d", loaded.PollIntervalSeconds, cfg.PollIntervalSeconds)
	}
	if loaded.MachineName != cfg.MachineName {
		t.Fatalf("machine name mismatch: got %q want %q", loaded.MachineName, cfg.MachineName)
	}
}
