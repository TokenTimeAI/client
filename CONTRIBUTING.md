# Contributing to ttime Client

Thank you for your interest in contributing to the ttime client! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Adding New Agent Detectors](#adding-new-agent-detectors)
- [Release Process](#release-process)

## Code of Conduct

Be respectful, constructive, and professional. All contributors are expected to follow standard open-source etiquette.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally
3. Create a branch for your changes
4. Make your changes following the guidelines below
5. Push and open a pull request

## Development Setup

### Prerequisites

- Go 1.23 or later
- Make (for build automation)
- Git

### Clone and Build

```bash
git clone https://github.com/tokentimeai/client.git
cd client
make build
```

### Verify Setup

```bash
# Run tests
make test

# Build and run
make run ARGS="status"
```

## Making Changes

### Code Style

- Follow standard Go conventions (enforced by `go fmt`)
- Run `make fmt` before committing
- Keep functions focused and reasonably sized
- Add comments for exported functions and types
- Use meaningful variable names

### Project Structure

When adding new code, respect the existing structure:

- `cmd/ttime/` - Main entry point only
- `internal/api/` - HTTP client and API types
- `internal/scanner/detectors/` - Agent-specific detectors
- `internal/config/` - Configuration management
- `internal/service/` - Daemon service logic

### Commit Messages

Follow conventional commit format:

```
<type>: <description>

[optional body]

[optional footer]
```

Types:
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `refactor` - Code refactoring
- `test` - Test changes
- `chore` - Build/tooling changes

Examples:
```
feat: add detector for new AI agent

fix: handle nil pointer in scanner state
docs: update installation instructions
```

## Testing

### Running Tests

```bash
# All tests
make test

# Short tests only
make test-short

# With race detector
make test-race

# Coverage report
make test-coverage
```

### Writing Tests

- Add tests for new functionality
- Place tests in `_test.go` files alongside source
- Use table-driven tests where appropriate
- Mock external dependencies (HTTP, filesystem)

Example:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid", "input", "output"},
        {"empty", "", ""},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

## Submitting Changes

### Before Submitting

1. Run all checks: `make check`
2. Ensure tests pass: `make test`
3. Update documentation if needed
4. Review your changes with `git diff`

### Pull Request Process

1. Push your branch to your fork
2. Open a PR against `master`
3. Fill out the PR template
4. Link any related issues
5. Wait for review and address feedback

### PR Guidelines

- Keep changes focused on a single concern
- Include tests for new functionality
- Update README if adding user-facing changes
- Respond to review comments promptly

## Adding New Agent Detectors

To add support for a new AI agent:

1. Create a new file in `internal/scanner/detectors/`
2. Implement the `scanner.Detector` interface
3. Register the detector in `init()`

### Detector Interface

```go
type Detector interface {
    Name() string                    // Unique identifier (e.g., "myagent")
    Description() string             // Human-readable description
    DefaultPaths() []string          // Common config/data directories
    Priority() int                   // Scan priority (lower = earlier)
    Detect(ctx context.Context) (bool, error)  // Check if agent is installed
    Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error)
}
```

### Example Detector

```go
package detectors

import (
    "context"
    "github.com/ttime-ai/ttime/client/internal/scanner"
)

type MyAgentDetector struct {
    scanner.BaseDetector
    dataDir string
}

func NewMyAgentDetector() scanner.Detector {
    return &MyAgentDetector{
        BaseDetector: scanner.NewBaseDetector(
            "myagent",
            "MyAgent AI conversations",
            []string{"~/.myagent", "~/.config/myagent"},
            50,
        ),
    }
}

func (d *MyAgentDetector) Detect(ctx context.Context) (bool, error) {
    for _, path := range d.DefaultPaths() {
        expanded, err := scanner.ExpandHome(path)
        if err != nil {
            continue
        }
        if scanner.DirExists(expanded) {
            d.dataDir = expanded
            d.SetFoundPath(expanded)
            return true, nil
        }
    }
    return false, nil
}

func (d *MyAgentDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
    // Read agent data files and return results
    // ... implementation ...
}

func init() {
    scanner.Register(NewMyAgentDetector)
}
```

### Testing Detectors

Add a test file with sample data:

```go
// myagent_test.go
func TestMyAgentDetector_Scan(t *testing.T) {
    d := NewMyAgentDetector().(*MyAgentDetector)
    d.dataDir = "testdata/myagent"
    
    results, _, err := d.Scan(context.Background(), scanner.SourceState{})
    if err != nil {
        t.Fatal(err)
    }
    
    if len(results) == 0 {
        t.Error("expected results")
    }
}
```

## Release Process

Releases are automated via GoReleaser when tags are pushed:

```bash
# Create a new version tag
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

The CI pipeline will:
1. Run tests
2. Build binaries for all platforms
3. Create GitHub release
4. Update Homebrew formula

### Versioning

Follow semantic versioning:
- MAJOR - Breaking changes
- MINOR - New features (backwards compatible)
- PATCH - Bug fixes

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for questions or ideas
- Email: support@ttime.ai

Thank you for contributing!
