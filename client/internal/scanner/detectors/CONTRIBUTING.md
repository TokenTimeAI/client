# Contributing a New Agent Detector

This guide explains how to add support for a new AI agent to ttime's native scanner.

## Overview

Detectors are modular plugins that scan agent-specific data stores (SQLite databases, JSON files, etc.) to extract conversation metrics including token counts, costs, and model information.

## Quick Start

1. Copy the template: `cp template.go your_agent.go`
2. Implement the `Detector` interface
3. Register your detector in `init()`
4. Run tests: `go test ./internal/scanner/...`
5. Submit a PR

## The Detector Interface

```go
type Detector interface {
    // Name returns the unique agent identifier (e.g., "cosine", "cline")
    Name() string

    // Description returns a human-readable description
    Description() string

    // Detect returns true if this agent's data exists on the system
    Detect(ctx context.Context) (bool, error)

    // Scan retrieves new results since the provided state
    Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error)

    // DefaultPaths returns expected paths for this agent's data
    DefaultPaths() []string

    // Priority returns scan priority (higher = scanned first)
    Priority() int
}
```

## Key Concepts

### Incremental Scanning

Detectors must support incremental scanning - only returning new data since the last scan. Use `state.LastScanTime` and `state.LastRecordID` to track position:

```go
func (d *YourDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
    // Only process records newer than LastScanTime
    // Update state with latest processed record
}
```

### BaseDetector Helper

Embed `BaseDetector` to get common functionality:

```go
type YourDetector struct {
    scanner.BaseDetector
    foundPath string  // Store detected path here
}

func NewYourDetector() scanner.Detector {
    return &YourDetector{
        BaseDetector: scanner.NewBaseDetector(
            "your_agent",           // unique name
            "Description here",      // human-readable
            []string{"~/.youragent"}, // default paths
            50,                      // priority
        ),
    }
}
```

### Cross-Platform Paths

Use `~` for home directory and let the scanner expand it:

```go
paths := []string{
    "~/Library/Application Support/YourAgent",  // macOS
    "~/.config/youragent",                       // Linux
    "~/AppData/Roaming/YourAgent",              // Windows
}
```

### Error Handling

Be resilient - don't fail the entire scan if your detector has issues:

```go
if err != nil {
    // Log and continue, don't return error
    return results, state, nil
}
```

## File Structure

Place your detector in `internal/scanner/detectors/`:

```
detectors/
├── your_agent.go      # Your detector implementation
├── cosine.go          # Example: JSON file scanner
├── cline.go           # Example: SQLite + JSON scanner
└── CONTRIBUTING.md    # This file
```

## Data Format

Return `ScanResult` structs with these key fields:

```go
result := scanner.ScanResult{
    AgentType:        "your_agent",     // Your detector name
    Type:             "conversation",   // "conversation", "command", etc.
    Entity:           projectPath,      // Project/file being worked on
    Time:             float64(timestamp),
    Timestamp:        time.Unix(timestamp, 0),
    ConversationID:   convID,           // Unique conversation ID
    MessageID:        msgID,            // Unique message ID
    PromptTokens:     msg.InputTokens,
    CompletionTokens: msg.OutputTokens,
    TotalTokens:      msg.TotalTokens,
    Model:            msg.Model,
    CostUSD:          msg.CostUSD,
    Project:          projectName,
}
```

## Testing

Test your detector manually:

```go
func TestYourDetector(t *testing.T) {
    d := NewYourDetector()
    
    ctx := context.Background()
    detected, err := d.Detect(ctx)
    if err != nil {
        t.Fatalf("detect failed: %v", err)
    }
    
    if detected {
        results, _, err := d.Scan(ctx, scanner.SourceState{})
        if err != nil {
            t.Fatalf("scan failed: %v", err)
        }
        t.Logf("found %d results", len(results))
    }
}
```

## Checklist

Before submitting your PR:

- [ ] Detector implements all interface methods
- [ ] `init()` function registers the detector
- [ ] Incremental scanning works correctly
- [ ] Cross-platform paths included
- [ ] Handles missing files gracefully
- [ ] Returns assistant/model messages only (not user prompts)
- [ ] Builds without errors: `go build ./...`
- [ ] Tests pass: `go test ./internal/scanner/...`

## Example Detectors

- **Simple JSON**: `cosine.go` - Scans JSON session files
- **SQLite**: `cline.go` - Queries SQLite database with fallback to JSON
- **Multiple formats**: `claude.go` - Handles various file layouts

## Questions?

Open an issue or discussion on GitHub. We're happy to help!
