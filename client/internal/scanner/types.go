package scanner

import (
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