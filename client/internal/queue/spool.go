package queue

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/api"
)

type Spool struct {
	Path string
}

func New(path string) *Spool {
	return &Spool{Path: path}
}

func (s *Spool) ReadAll() ([]api.Heartbeat, error) {
	file, err := os.Open(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var heartbeats []api.Heartbeat
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var heartbeat api.Heartbeat
		if err := json.Unmarshal([]byte(line), &heartbeat); err != nil {
			return nil, err
		}
		heartbeats = append(heartbeats, heartbeat)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return heartbeats, nil
}

func (s *Spool) Append(heartbeats []api.Heartbeat) error {
	if len(heartbeats) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(s.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, heartbeat := range heartbeats {
		if err := encoder.Encode(heartbeat); err != nil {
			return err
		}
	}
	return nil
}

func (s *Spool) Clear() error {
	if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
