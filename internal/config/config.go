package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultServerURL          = "https://ttime.ai"
	defaultPollIntervalSecond = 10
)

type Config struct {
	ServerURL           string `json:"server_url"`
	APIKey              string `json:"api_key"`
	InboxDir            string `json:"inbox_dir"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	MachineName         string `json:"machine_name"`
	AuthenticatedEmail  string `json:"authenticated_email,omitempty"`
	AuthenticatedName   string `json:"authenticated_name,omitempty"`
}

type Paths struct {
	RootDir            string
	ConfigFile         string
	QueueFile          string
	CollectorStateFile string
	ScannerStateFile   string
}

func DefaultPaths() (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}

	root := filepath.Join(configDir, "ttime")
	return Paths{
		RootDir:            root,
		ConfigFile:         filepath.Join(root, "config.json"),
		QueueFile:          filepath.Join(root, "queue.jsonl"),
		CollectorStateFile: filepath.Join(root, "collector-state.json"),
		ScannerStateFile:   filepath.Join(root, "scanner-state.json"),
	}, nil
}

func Default() (Config, error) {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown-machine"
	}

	paths, err := DefaultPaths()
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServerURL:           defaultServerURL,
		InboxDir:            filepath.Join(paths.RootDir, "inbox"),
		PollIntervalSeconds: defaultPollIntervalSecond,
		MachineName:         hostname,
	}, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg, err := Default()
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.ApplyDefaults()
	return cfg, nil
}

func LoadOrDefault(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	return Default()
}

func Save(path string, cfg Config) error {
	cfg.ApplyDefaults()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.InboxDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o600)
}

func (c *Config) ApplyDefaults() {
	def, err := Default()
	if err != nil {
		return
	}

	if strings.TrimSpace(c.ServerURL) == "" {
		c.ServerURL = def.ServerURL
	}
	if strings.TrimSpace(c.InboxDir) == "" {
		c.InboxDir = def.InboxDir
	}
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = def.PollIntervalSeconds
	}
	if strings.TrimSpace(c.MachineName) == "" {
		c.MachineName = def.MachineName
	}
}
