# ttime Client

A local heartbeat daemon client for [ttime.ai](https://ttime.ai) — track your AI coding agent usage across multiple tools and sync it to the cloud.

## Overview

The ttime client is a lightweight Go application that runs on your local machine, monitoring your usage of various AI coding assistants (like Claude Code, Cline, Cursor, Copilot, and more). It collects usage metrics (tokens, duration, cost) and syncs them to your ttime.ai account.

## Features

- **Multi-Agent Support**: Tracks conversations from 15+ AI coding assistants including:
  - Claude Code
  - Cline / Cline CLI
  - Cursor
  - GitHub Copilot
  - Cosine (COS CLI)
  - Gemini
  - Windsurf
  - Factory.ai
  - OpenClaw
  - OpenCode
  - Hermes
  - Codex
  - And more...

- **Local-First**: All data is collected locally first; no data leaves your machine without explicit sync
- **Background Daemon**: Runs continuously in the background with minimal resource usage
- **Automatic Updates**: Built-in update mechanism to stay current
- **Cross-Platform**: Supports macOS, Linux, and Windows (AMD64 & ARM64)
- **Secure**: Uses device authorization flow (OAuth2) for secure API authentication

## Installation

### Homebrew (macOS/Linux)

```bash
brew install tokentimeai/tap/ttime
```

### Install Script

```bash
curl -sSL https://ttime.ai/install.sh | sh
```

Or on Windows with PowerShell:

```powershell
iwr -useb https://ttime.ai/install.ps1 | iex
```

### Manual Download

Download the latest release for your platform from the [releases page](https://github.com/tokentimeai/client/releases).

### Build from Source

```bash
git clone https://github.com/tokentimeai/client.git
cd client
make build
```

## Quick Start

### 1. Initial Setup

Run the interactive setup wizard to configure the client:

```bash
ttime setup
```

This will:
- Connect to your ttime.ai account via device authorization
- Generate an API key for your machine
- Configure the local daemon

### 2. Start the Daemon

```bash
# Run once (for testing)
ttime daemon --once

# Or install as a system service
ttime install

# Start the service (platform-specific)
# macOS: launchctl start ttime
# Linux: systemctl --user start ttime
```

### 3. Verify Status

```bash
ttime status
```

## Commands

| Command | Description |
|---------|-------------|
| `ttime setup` | Interactive setup wizard (TUI) |
| `ttime status` | Show current configuration and daemon status |
| `ttime daemon` | Run the background daemon |
| `ttime daemon --once` | Process queued events once and exit |
| `ttime daemon --no-scan` | Run without agent database scanning |
| `ttime agents` | List detected AI agents on this system |
| `ttime scan` | Manually scan agent databases |
| `ttime scan --agent <name>` | Scan only specific agent |
| `ttime scan --all` | Scan all conversations (ignore state) |
| `ttime install` | Install as system service |
| `ttime uninstall` | Remove system service |
| `ttime update` | Check for and install updates |
| `ttime update --check` | Check for updates only |

## Configuration

Configuration is stored in your platform's config directory:

- **macOS**: `~/Library/Application Support/ttime/config.json`
- **Linux**: `~/.config/ttime/config.json`
- **Windows**: `%AppData%/ttime/config.json`

### Config File Structure

```json
{
  "server_url": "https://ttime.ai",
  "api_key": "your-api-key",
  "inbox_dir": "/path/to/inbox",
  "poll_interval_seconds": 10,
  "machine_name": "my-machine",
  "authenticated_email": "user@example.com"
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        ttime client                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Scanner   │  │  Collector  │  │      Queue          │  │
│  │  (Agents)   │  │   (Inbox)   │  │   (Persistence)     │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                    │             │
│         └────────────────┼────────────────────┘             │
│                          ▼                                  │
│                   ┌─────────────┐                           │
│                   │   Sender    │◄──── ttime.ai API        │
│                   └─────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

### Components

- **Scanner**: Reads conversation data from AI agent databases
- **Collector**: Processes `.jsonl` files from the inbox directory
- **Queue**: Persistent SQLite-backed queue for events pending sync
- **Sender**: HTTP client for sending heartbeats to ttime.ai API

## Development

### Prerequisites

- Go 1.23+
- Make

### Build

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Install to $GOPATH/bin
make install
```

### Test

```bash
# Run all tests
make test

# Run with race detector
make test-race

# Generate coverage report
make test-coverage
```

### Lint & Format

```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter
make lint

# Run all checks
make check
```

### Release

Uses [GoReleaser](https://goreleaser.com/) for automated releases:

```bash
# Check config
make release-check

# Build snapshot (no release)
make snapshot

# Full release (requires GITHUB_TOKEN)
goreleaser release
```

## Project Structure

```
client/
├── cmd/ttime/              # Main entry point
├── internal/
│   ├── api/               # HTTP client for ttime.ai API
│   ├── bootstrap/         # Setup TUI and device auth
│   ├── cli/               # Command-line interface
│   ├── collector/         # JSONL file collector
│   ├── config/            # Configuration management
│   ├── normalize/         # Heartbeat normalization
│   ├── platform/          # Platform-specific service management
│   ├── queue/             # Persistent event queue
│   ├── scanner/           # Agent database scanners
│   │   └── detectors/     # Individual agent detectors
│   ├── service/           # Daemon service logic
│   ├── tui/               # Terminal UI components
│   └── updater/           # Self-update mechanism
├── testdata/              # Test fixtures
├── Makefile               # Build automation
└── .goreleaser.yml        # Release configuration
```

## Supported Agents

The client can detect and scan the following AI coding assistants:

| Agent | ID | Default Paths |
|-------|-----|---------------|
| Claude Code | `claude_code` | `~/.claude`, `~/.config/claude`, `~/Library/Application Support/Claude` |
| Cline | `cline` | `~/.vscode/globalStorage/saoudrizwan.claude-dev`, `~/.cursor/extensions/saoudrizwan.claude-dev` |
| Cline CLI | `cline_cli` | `~/.cline`, `~/.config/cline` |
| Cursor | `cursor` | `~/.cursor`, `~/.config/Cursor` |
| GitHub Copilot | `copilot` | `~/.config/github-copilot` |
| Cosine | `cosine` | `~/.cosine`, `~/.config/cosine` |
| Gemini | `gemini` | `~/.gemini`, `~/.config/gemini` |
| Windsurf | `windsurf` | `~/.windsurf`, `~/.config/windsurf` |
| Factory.ai | `factory` | `~/.factory`, `~/.config/factory` |
| OpenClaw | `openclaw` | `~/.openclaw`, `~/.config/openclaw` |
| OpenCode | `opencode` | `~/.opencode`, `~/.config/opencode` |
| Hermes | `hermes` | `~/.hermes`, `~/.config/hermes` |
| Codex | `codex` | `~/.codex`, `~/.config/codex` |

## Support

- **Website**: https://ttime.ai
- **Issues**: https://github.com/tokentimeai/client/issues
- **Email**: support@ttime.ai
