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
	registry     *Registry
}

// New creates a new scanner with the given state path and interval
func New(statePath string, interval time.Duration) *Scanner {
	if interval <= 0 {
		interval = 5 * time.Minute // Default scan every 5 minutes
	}
	return &Scanner{
		stateManager: NewStateManager(statePath),
		interval:     interval,
		registry:     globalRegistry,
	}
}

// NewWithRegistry creates a scanner with a custom registry (for testing)
func NewWithRegistry(statePath string, interval time.Duration, registry *Registry) *Scanner {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Scanner{
		stateManager: NewStateManager(statePath),
		interval:     interval,
		registry:     registry,
	}
}

// ScanOnce performs a single scan of all detected agents
func (s *Scanner) ScanOnce(ctx context.Context) ([]ScanResult, error) {
	state, err := s.stateManager.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	detectors, err := s.findDetectors(ctx)
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

// ScanAgent scans a specific agent by name
func (s *Scanner) ScanAgent(ctx context.Context, agentName string) ([]ScanResult, error) {
	detector, ok := s.registry.Get(agentName)
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentName)
	}

	// Check if detected
	detected, err := detector.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("detect agent %s: %w", agentName, err)
	}
	if !detected {
		return nil, fmt.Errorf("agent %s not detected on this system", agentName)
	}

	state, err := s.stateManager.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	sourceState := s.stateManager.GetSource(state, detector.Name())
	results, newState, err := detector.Scan(ctx, sourceState)
	if err != nil {
		return nil, fmt.Errorf("scan agent %s: %w", agentName, err)
	}

	s.stateManager.UpdateSource(&state, detector.Name(), newState)
	if err := s.stateManager.Save(state); err != nil {
		return results, fmt.Errorf("save state: %w", err)
	}

	return results, nil
}

// findDetectors returns detected detectors from the scanner's registry
func (s *Scanner) findDetectors(ctx context.Context) ([]Detector, error) {
	var detected []Detector
	for _, name := range s.registry.List() {
		d, ok := s.registry.Get(name)
		if !ok {
			continue
		}
		ok, err := d.Detect(ctx)
		if err != nil {
			continue
		}
		if ok {
			detected = append(detected, d)
		}
	}
	
	// Sort by priority descending
	for i := 0; i < len(detected)-1; i++ {
		for j := i + 1; j < len(detected); j++ {
			if detected[j].Priority() > detected[i].Priority() {
				detected[i], detected[j] = detected[j], detected[i]
			}
		}
	}
	
	return detected, nil
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

// GetDetectorInfo returns info about all detectors
func GetDetectorInfo(ctx context.Context) []DetectorInfo {
	return GetDetectedInfos(ctx)
}

// Detected returns which detectors found their agents
func (s *Scanner) Detected(ctx context.Context) ([]string, error) {
	detectors, err := s.findDetectors(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(detectors))
	for i, d := range detectors {
		names[i] = d.Name()
	}
	return names, nil
}