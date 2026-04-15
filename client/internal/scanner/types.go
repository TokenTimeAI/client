package scanner

import (
	"context"
	"time"
)

// ScanResult represents a conversation or interaction from an AI agent
type ScanResult struct {
	// Core identity
	AgentType string `json:"agent_type"`
	Entity    string `json:"entity"`     // e.g., conversation ID or project path
	Type      string `json:"type"`       // "conversation", "file", "command", etc.

	// Timing
	Time      float64   `json:"time"`       // Unix timestamp
	Duration  float64   `json:"duration"`   // Duration in seconds
	Timestamp time.Time `json:"timestamp"`  // Parsed time
	SessionStartedAt      *time.Time `json:"session_started_at,omitempty"`
	SessionEndedAt        *time.Time `json:"session_ended_at,omitempty"`
	SessionDurationSeconds *int      `json:"session_duration_seconds,omitempty"`
	AgentActiveSeconds    *int       `json:"agent_active_seconds,omitempty"`
	HumanActiveSeconds    *int       `json:"human_active_seconds,omitempty"`
	IdleSeconds           *int       `json:"idle_seconds,omitempty"`

	// Conversation metadata
	ConversationID   string `json:"conversation_id,omitempty"`
	MessageID        string `json:"message_id,omitempty"`
	ParentMessageID  string `json:"parent_message_id,omitempty"`

	// Token usage (what we really care about)
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`

	// Cost tracking
	CostUSD float64 `json:"cost_usd,omitempty"`

	// Content metrics
	LinesAdded   int    `json:"lines_added,omitempty"`
	LinesDeleted int    `json:"lines_deleted,omitempty"`
	IsWrite      bool   `json:"is_write,omitempty"`

	// Model information
	Model   string `json:"model,omitempty"`
	Version string `json:"version,omitempty"`

	// Project context
	Project  string `json:"project,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Language string `json:"language,omitempty"`

	// Additional metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// State represents the last scan position for each data source
type State struct {
	Sources map[string]SourceState `json:"sources"`
}

// SourceState tracks the last scan position for a single source
type SourceState struct {
	LastScanTime   int64  `json:"last_scan_time"`   // Unix timestamp of last successful scan
	LastRecordID   string `json:"last_record_id"`   // Last processed record ID
	LastOffset     int64  `json:"last_offset"`      // For file-based sources
	RowID          int64  `json:"row_id"`           // For SQLite sources
	Checksum       string `json:"checksum"`         // For change detection
}

// NewState creates an empty state
func NewState() State {
	return State{
		Sources: make(map[string]SourceState),
	}
}

// Detector is the interface for agent-specific scanners.
// Implement this interface to add support for a new AI agent.
// See CONTRIBUTING.md for detailed documentation.
type Detector interface {
	// Name returns the unique agent identifier (e.g., "cosine", "cline", "codex")
	// This is used as the agent_type in heartbeats and for state tracking.
	Name() string

	// Description returns a human-readable description of what this detector scans
	Description() string

	// Detect returns true if this agent's data exists on the system.
	// Called once during scanner initialization. Store the detected path
	// in your detector for use during Scan.
	Detect(ctx context.Context) (bool, error)

	// Scan retrieves new results since the provided state.
	// This should return ONLY new events since last scan (incremental).
	// The returned SourceState will be persisted and passed on next scan.
	//
	// Implementations should:
	// - Check LastScanTime/LastRecordID to determine where to resume
	// - Return results in chronological order
	// - Update the state with the latest processed record
	// - Handle errors gracefully (log and continue, don't fail entire scan)
	Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error)

	// DefaultPaths returns the expected paths where this agent stores data.
	// Used for documentation and health checks. Each path should use
	// filepath.Join for cross-platform compatibility.
	DefaultPaths() []string

	// Priority returns the scan priority (higher = scanned first).
	// Use for agents that lock their databases (scan them first).
	Priority() int
}

// BaseDetector provides common utilities for detector implementations.
// Embed this in your detector to get helper methods.
type BaseDetector struct {
	name        string
	description string
	paths       []string
	priority    int
	foundPath   string
}

// NewBaseDetector creates a base detector with common configuration
func NewBaseDetector(name, description string, paths []string, priority int) BaseDetector {
	return BaseDetector{
		name:        name,
		description: description,
		paths:       paths,
		priority:    priority,
	}
}

func (b *BaseDetector) Name() string        { return b.name }
func (b *BaseDetector) Description() string { return b.description }
func (b *BaseDetector) DefaultPaths() []string { return b.paths }
func (b *BaseDetector) Priority() int       { return b.priority }
func (b *BaseDetector) FoundPath() string   { return b.foundPath }
func (b *BaseDetector) SetFoundPath(path string) { b.foundPath = path }

// DetectorInfo provides metadata about a registered detector
type DetectorInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Detected    bool     `json:"detected"`
	FoundPath   string   `json:"found_path,omitempty"`
	Paths       []string `json:"default_paths"`
}

// DetectorConstructor is a function that creates a new detector instance
// This enables dynamic detector loading and testing
type DetectorConstructor func() Detector

// Registry manages detector registration and discovery
type Registry struct {
	constructors map[string]DetectorConstructor
	instances    map[string]Detector
	priorityList []string
}

// NewRegistry creates a new detector registry
func NewRegistry() *Registry {
	return &Registry{
		constructors: make(map[string]DetectorConstructor),
		instances:    make(map[string]Detector),
	}
}

// Register adds a detector constructor to the registry
func (r *Registry) Register(name string, ctor DetectorConstructor) {
	r.constructors[name] = ctor
}

// Get returns a detector instance by name (creates if needed)
func (r *Registry) Get(name string) (Detector, bool) {
	if inst, ok := r.instances[name]; ok {
		return inst, true
	}
	if ctor, ok := r.constructors[name]; ok {
		inst := ctor()
		r.instances[name] = inst
		return inst, true
	}
	return nil, false
}

// List returns all registered detector names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.constructors))
	for name := range r.constructors {
		names = append(names, name)
	}
	return names
}

// All returns all detector instances, sorted by priority
func (r *Registry) All() []Detector {
	detectors := make([]Detector, 0, len(r.instances))
	for _, inst := range r.instances {
		detectors = append(detectors, inst)
	}
	// Sort by priority descending
	for i := 0; i < len(detectors)-1; i++ {
		for j := i + 1; j < len(detectors); j++ {
			if detectors[j].Priority() > detectors[i].Priority() {
				detectors[i], detectors[j] = detectors[j], detectors[i]
			}
		}
	}
	return detectors
}

// globalRegistry is the default registry for all detectors
var globalRegistry = NewRegistry()

// Register adds a detector to the global registry
func Register(ctor DetectorConstructor) {
	inst := ctor()
	globalRegistry.Register(inst.Name(), ctor)
}

// GetDetector retrieves a detector from the global registry
func GetDetector(name string) (Detector, bool) {
	return globalRegistry.Get(name)
}

// ListDetectors returns all registered detector names
func ListDetectors() []string {
	return globalRegistry.List()
}

// AllDetectors returns all detector instances from global registry
func AllDetectors() []Detector {
	return globalRegistry.All()
}
