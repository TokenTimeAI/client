# Local heartbeat daemon (`ttime`)

The Go client under `client/` provides a local daemon that ingests JSONL events from a user inbox directory and forwards them to the Rails API in bulk.

## Install

Install from source with Go:

```bash
go install github.com/ttime-ai/ttime/client/cmd/ttime@latest
```

Or install from Homebrew:

```bash
brew tap ttime-ai/homebrew-tap
brew install ttime
brew upgrade ttime
```

If you use Homebrew, manage the long-running daemon with:

```bash
brew services start ttime
brew services stop ttime
brew services restart ttime
```

If you do not use Homebrew, `ttime install` and `ttime uninstall` write and remove the per-user launchd/systemd unit files directly.

## Commands

```bash
ttime setup
ttime status
ttime daemon [--once] [--no-scan]
ttime agents          # list detected AI agents
ttime scan [--agent <name>]  # scan agent databases
ttime install
ttime uninstall
```

## Setup flow

`ttime setup` uses a small Bubble Tea TUI:

1. Prompt for the server URL
2. Create a device authorization
3. Show the user code and verification URL
4. Poll until approval
5. Persist config to the user config directory

The config file is stored at:

- macOS: `~/Library/Application Support/ttime/config.json`
- Linux: `~/.config/ttime/config.json`

The daemon also stores:

- collector offsets: `collector-state.json`
- retry spool: `queue.jsonl`
- inbox directory: `inbox/`

## Inbox format

The daemon reads newline-delimited JSON objects from files in the configured inbox directory. Each line should match the Rails heartbeat schema:

```json
{
  "entity": "main.go",
  "type": "file",
  "project": "demo",
  "branch": "main",
  "language": "Go",
  "agent_type": "codex",
  "time": 1700000000.0,
  "is_write": true,
  "tokens_used": 500,
  "lines_added": 20,
  "lines_deleted": 4,
  "cost_usd": 0.01,
  "metadata": { "tool": "write_file" }
}
```

## Wiring agent hooks into the inbox

The daemon’s integration boundary is the inbox directory. The recommended pattern is:

1. each local agent writes JSONL heartbeats into its own file under `inbox/`
2. `ttime daemon` tails those files by byte offset
3. the daemon normalizes and uploads the combined stream to Rails

Separate files per agent make it easy to debug and avoid write contention:

- `inbox/claude_code.jsonl`
- `inbox/cosine.jsonl` or `inbox/cos.jsonl`
- `inbox/cline.jsonl`
- `inbox/cursor.jsonl`
- `inbox/codex.jsonl`
- `inbox/copilot.jsonl` (for GitHub Copilot CLI)
- `inbox/opencode.jsonl`
- `inbox/continue.jsonl`
- `inbox/aider.jsonl`

### Claude Code hook example

Claude Code supports command hooks. A practical pattern is to append one JSON line after tool use:

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": ".*",
      "hooks": [{
        "type": "command",
        "command": "mkdir -p \"$HOME/.config/ttime/inbox\" && printf '%s\n' \"{\\\"entity\\\":\\\"$CLAUDE_PROJECT_DIR\\\",\\\"type\\\":\\\"app\\\",\\\"agent_type\\\":\\\"claude_code\\\",\\\"time\\\":$(date +%s),\\\"metadata\\\":{\\\"session_id\\\":\\\"$CLAUDE_SESSION_ID\\\"}}\" >> \"$HOME/.config/ttime/inbox/claude_code.jsonl\""
      }]
    }]
  }
}
```

If your hook environment provides a transcript path, current working directory, tool name, or session ID, include those values in `metadata`.

### Cursor hook example

Cursor’s hooks can use the same append-to-JSONL pattern:

```json
{
  "hooks": {
    "postToolUse": [{
      "command": "mkdir -p \"$HOME/.config/ttime/inbox\" && printf '%s\n' '{\"entity\":\"'$PWD'\",\"type\":\"app\",\"agent_type\":\"cursor\",\"time\":'$(date +%s)',\"metadata\":{\"source\":\"cursor-hook\"}}' >> \"$HOME/.config/ttime/inbox/cursor.jsonl\""
    }]
  }
}
```

### Cosine/COS hook example

Cosine CLI supports hooks via `.cosine.toml` or `cosine.toml` configuration. Configure post-action hooks to emit events:

```toml
[[hooks.PostAction]]
matcher = ".*"
command = "mkdir -p \"$HOME/.config/ttime/inbox\" && printf '%s\\n' \"{\\\"entity\\\":\\\"$PWD\\\",\\\"type\\\":\\\"app\\\",\\\"agent_type\\\":\\\"cosine\\\",\\\"time\\\":$(date +%s),\\\"metadata\\\":{\\\"source\\\":\\\"cosine-hook\\\"}}\" >> \"$HOME/.config/ttime/inbox/cosine.jsonl\""
```

Or via MCP, configure a server to write events:

```toml
[mcp.ttime]
command = "sh"
args = ["-c", "mkdir -p ~/.config/ttime/inbox && cat >> ~/.config/ttime/inbox/cosine.jsonl"]
```

### Cline hook example

Cline (VS Code extension) supports custom settings for hooks. Add to your VS Code settings:

```json
{
  "cline.postToolUseHooks": [
    "mkdir -p \"$HOME/.config/ttime/inbox\" && printf '%s\\n' '{\"entity\":\"'$PWD'\",\"type\":\"app\",\"agent_type\":\"cline\",\"time\":'$(date +%s)',\"metadata\":{\"source\":\"cline-hook\"}}' >> \"$HOME/.config/ttime/inbox/cline.jsonl\""
  ]
}
```

### OpenCode hook example

OpenCode supports hooks in its configuration file (`.opencode/config.toml`):

```toml
[hooks]
post_action = [
  "mkdir -p \"$HOME/.config/ttime/inbox\" && printf '%s\\n' '{\"entity\":\"'$PWD'\",\"type\":\"app\",\"agent_type\":\"opencode\",\"time\":'$(date +%s)',\"metadata\":{\"source\":\"opencode-hook\"}}' >> \"$HOME/.config/ttime/inbox/opencode.jsonl\""
]
```

### GitHub Copilot CLI hook example

For the `gh copilot` CLI, create a wrapper script to capture usage:

```bash
#!/bin/bash
# copilot-wrapper.sh - Wrap gh copilot suggest and emit stats

mkdir -p "$HOME/.config/ttime/inbox"
START_TIME=$(date +%s)

# Run the actual copilot command
gh copilot "$@"
EXIT_CODE=$?

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Emit heartbeat
printf '%s\\n' "{\"entity\":\"$PWD\",\"type\":\"app\",\"agent_type\":\"copilot_cli\",\"time\":$END_TIME,\"metadata\":{\"duration_seconds\":$DURATION,\"command\":\"$*\"}}" >> "$HOME/.config/ttime/inbox/copilot.jsonl"

exit $EXIT_CODE
```

Add to your shell profile (e.g., `~/.bashrc`, `~/.zshrc`):

```bash
alias copilot='/path/to/copilot-wrapper.sh'
```

### Other supported sources

- **Cosine/COS**: configure hooks to write JSONL events into `inbox/cosine.jsonl` or `inbox/cos.jsonl`
- **Cline**: add post-tool hooks to append events to `inbox/cline.jsonl`
- **Codex**: write hook or session-derived JSONL lines into `inbox/codex.jsonl`
- **GitHub Copilot CLI**: use `gh copilot` with wrapper scripts that export session events into `inbox/copilot.jsonl`
- **OpenCode**: configure hooks to write events to `inbox/opencode.jsonl`
- **Continue**: point development-data output or a small wrapper into `inbox/continue.jsonl`
- **Aider**: append derived events from `.aider.chat.history.md` or notifications into `inbox/aider.jsonl`

The daemon intentionally does not scrape undocumented IDE databases. The stable contract is: if the local tool can emit JSONL heartbeat records, `ttime daemon` can ingest them.

## Retry behavior

If `POST /api/v1/heartbeats/bulk` fails, newly collected events are appended to `queue.jsonl` and retried on the next daemon run. The collector maintains per-file byte offsets so the daemon does not resend the same inbox lines once they have been queued.

## Per-user service install

- macOS installs a LaunchAgent at `~/Library/LaunchAgents/ai.ttime.daemon.plist`
- Linux installs a systemd user unit at `~/.config/systemd/user/ttime.service`

Both services execute:

```bash
ttime daemon
```

---

## Native Agent Scanning

The daemon includes **native agent scanning** that automatically detects and reads conversation data from installed AI agents. This captures complete conversation history including total token usage, model information, cost tracking, and conversation IDs.

### How It Works

1. **Detection**: On startup, the daemon scans for known agent configuration directories
2. **Scanning**: Every 5 minutes, it reads conversation databases (SQLite, JSON)
3. **Incremental**: Only new conversations since last scan are reported
4. **Deduplication**: Events tracked by message ID to avoid duplicates

### Supported Agents

| Agent | Data Location | Format |
|-------|--------------|--------|
| **Cosine/COS** | `~/.cosine/sessions/*.json` | JSON |
| **Cline** | VS Code extension storage | SQLite/JSON |
| **Cursor** | `~/Library/Application Support/Cursor` | JSON |
| **Claude Code** | `~/.claude/projects/*/chat_history.json` | JSON |
| **OpenCode** | `~/.opencode/conversations/*.json` | JSON |
| **GitHub Copilot CLI** | `~/.config/github-copilot/usage.json` | JSON |

### New CLI Commands

```bash
# List available and detected agents
ttime agents

# Scan agents once and display results
ttime scan

# Scan only specific agent
ttime scan --agent cosine

# Run daemon without scanning
ttime daemon --no-scan
```

### New Heartbeat Fields

Agent scanning adds these fields to heartbeats:

```json
{
  "agent_type": "cosine",
  "type": "conversation",
  "conversation_id": "conv-123",
  "message_id": "msg-456",
  "prompt_tokens": 1500,
  "completion_tokens": 800,
  "total_tokens": 2300,
  "model": "claude-3-5-sonnet",
  "cost_usd": 0.0075
}
```
