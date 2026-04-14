package scanner

import (
	"context"
	"fmt"
	"time"
)

// Scanner orchestrates detection and scanning of all AI agents
type Scanner struct {
	stateManager *StateManager
	interval     time.Duration
}

// New creates a new scanner with the given state path and interval
func New(statePath string, interval time.Duration) *Scanner {
	if interval <= 0 {
		interval = 5 * time.Minute // Default scan every 5 minutes
	}
	return &Scanner{
		stateManager: NewStateManager(statePath),
		interval:     interval,
	}
}

// ScanOnce performs a single scan of all detected agents
func (s *Scanner) ScanOnce(ctx context.Context) ([]ScanResult, error) {
	state, err := s.stateManager.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	detectors, err := FindDetectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("find detectors: %w", err)
	}

	var allResults []ScanResult
	
	for _, detector := range detectors {
		sourceState := s.stateManager.GetSource(state, detector.Name())
		
		results, newState, err := detector.Scan(ctx, sourceState)
		if err != nil {
			// Log error but continue with other detectors
			continue
		}

		allResults = append(allResults, results...)
		s.stateManager.UpdateSource(&state, detector.Name(), newState)
	}

	// Save updated state
	if err := s.stateManager.Save(state); err != nil {
		return allResults, fmt.Errorf("save state: %w", err)
	}

	return allResults, nil
}

// RunLoop continuously scans at the configured interval
func (s *Scanner) RunLoop(ctx context.Context, callback func([]ScanResult)) error {
	// Initial scan
	results, err := s.ScanOnce(ctx)
	if err != nil {
		return err
	}
	if len(results) > 0 && callback != nil {
		callback(results)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			results, err := s.ScanOnce(ctx)
			if err != nil {
				// Log error but continue
				continue
			}
			if len(results) > 0 && callback != nil {
				callback(results)
			}
		}
	}
}

// ListDetectors returns all registered detector names
func ListDetectors() []string {
	names := make([]string, len(Registry))
	for i, d := range Registry {
		names[i] = d.Name()
	}
	return names
}

// Detected returns which detectors found their agents
func (s *Scanner) Detected(ctx context.Context) ([]string, error) {
	detectors, err := FindDetectors(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(detectors))
	for i, d := range detectors {
		names[i] = d.Name()
	}
	return names, nil
}