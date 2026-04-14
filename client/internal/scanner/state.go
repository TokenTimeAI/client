package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StateManager persists scan state to disk
type StateManager struct {
	statePath string
}

// NewStateManager creates a state manager for the given path
func NewStateManager(statePath string) *StateManager {
	return &StateManager{statePath: statePath}
}

// Load reads the state from disk
func (sm *StateManager) Load() (State, error) {
	state := NewState()

	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return State{}, fmt.Errorf("read state file: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}

	if state.Sources == nil {
		state.Sources = make(map[string]SourceState)
	}

	return state, nil
}

// Save writes the state to disk
func (sm *StateManager) Save(state State) error {
	if err := os.MkdirAll(filepath.Dir(sm.statePath), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(sm.statePath, data, 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

// UpdateSource updates the state for a single source
func (sm *StateManager) UpdateSource(state *State, sourceName string, newState SourceState) {
	state.Sources[sourceName] = newState
}

// GetSource retrieves the state for a single source
func (sm *StateManager) GetSource(state State, sourceName string) SourceState {
	if s, ok := state.Sources[sourceName]; ok {
		return s
	}
	return SourceState{}
}