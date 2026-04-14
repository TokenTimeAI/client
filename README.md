# ttime.ai

**WakaTime for AI coding agents.** Track tokens, sessions, languages, and edits from Claude Code, Codex, Cursor, Copilot, and more.

## Features

- 🤖 **Agent-first API** — WakaTime-compatible heartbeat endpoints
- 📊 **Real-time dashboard** — Activity charts, language & agent breakdowns
- ⚡ **Tinybird analytics** — ClickHouse-powered via Tinybird for real-time analytics at scale
- 🔑 **API key management** — Create and revoke tokens per integration
- 📁 **Project tracking** — Auto-created from heartbeat data
- 🪪 **ULID primary keys** — Sortable, unique identifiers for all records
- 🔥 **Hotwire + Stimulus** — Modern, reactive UI without heavy JavaScript

## Tech Stack

- **Rails 8** monolith (PostgreSQL, Hotwire, Stimulus, Tailwind CSS v4)
- **Tinybird** (managed ClickHouse) for analytics data layer
- **SolidQueue** for background jobs (Tinybird ingestion)
- **Devise** for authentication
- **ULID** primary keys on all models

## Quick Start

```bash
bin/setup
bin/rails server
```

Visit `http://localhost:3000`, create an account, generate an API key.

## API Reference

### Authentication

All API endpoints require a Bearer token or WakaTime-style Basic auth:

```bash
# Bearer token
Authorization: Bearer tt_your_api_key

# Basic auth (WakaTime-compatible — use API key as username)
Authorization: Basic base64(tt_your_api_key:)
```

### Heartbeat Endpoints

#### Single heartbeat
```bash
POST /api/v1/heartbeats
POST /api/v1/users/current/heartbeats

{
  "entity":       "app/models/user.rb",   # file, domain, or app name
  "type":         "file",                 # file | app | domain | url
  "language":     "Ruby",
  "project":      "my-project",
  "agent_type":   "claude_code",          # see Agent Types below
  "time":         1700000000.123,         # Unix timestamp (float)
  "is_write":     true,
  "branch":       "main",
  "tokens_used":  1500,
  "lines_added":  42,
  "lines_deleted": 5,
  "cost_usd":     0.002
}
```

#### Bulk heartbeats
```bash
POST /api/v1/heartbeats/bulk
POST /api/v1/users/current/heartbeats/bulk

[
  { "entity": "main.py", "type": "file", "agent_type": "codex", "time": 1700000000 },
  { "entity": "utils.py", "type": "file", "agent_type": "codex", "time": 1700000060 }
]
```

#### Current user info
```bash
GET /api/v1/users/current
```

#### Daily summaries
```bash
GET /api/v1/summaries?start=2024-01-01&end=2024-01-07
GET /api/v1/users/current/summaries?start=2024-01-01&end=2024-01-07
```

#### Status bar (today's time)
```bash
GET /api/v1/users/current/statusbar/today
```

### Supported Agent Types

| `agent_type`    | Description                          |
|----------------|--------------------------------------|
| `claude_code`  | Anthropic Claude Code                |
| `codex`        | OpenAI Codex / ChatGPT coding tools  |
| `cursor`       | Cursor IDE                           |
| `copilot`      | GitHub Copilot                       |
| `codeium`      | Codeium                              |
| `continue`     | Continue.dev                         |
| `aider`        | Aider                                |
| `devin`        | Cognition Devin                      |
| `custom`       | Custom / unknown agents              |

## Agent Integration Examples

### Claude Code (`~/.claude/settings.json` hook)

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": ".*",
      "hooks": [{
        "type": "command",
        "command": "curl -s -X POST https://ttime.ai/api/v1/heartbeats -H 'Authorization: Bearer $TTIME_API_KEY' -H 'Content-Type: application/json' -d '{\"entity\":\".\",\"type\":\"app\",\"agent_type\":\"claude_code\",\"time\":'$(date +%s.%N)'}'"
      }]
    }]
  }
}
```

### OpenAI Codex

Configure via the `WAKATIME_API_KEY` environment variable (ttime is WakaTime-compatible):

```bash
export WAKATIME_API_KEY="tt_your_api_key"
export WAKATIME_API_URL="https://ttime.ai/api/v1/"
```

## Tinybird Analytics

See [docs/tinybird_schema.md](docs/tinybird_schema.md) for the ClickHouse schema and Pipe definitions.

Configure via environment variables:

```bash
TINYBIRD_TOKEN=p.eyJ...   # Workspace token
TINYBIRD_API_URL=https://api.tinybird.co  # Default
```

## Environment Variables

| Variable              | Description                                     | Default                      |
|-----------------------|-------------------------------------------------|------------------------------|
| `DATABASE_URL`        | PostgreSQL connection URL                       | —                            |
| `DB_USERNAME`         | PostgreSQL username                             | `ttime`                      |
| `DB_PASSWORD`         | PostgreSQL password                             | `ttime`                      |
| `DB_HOST`             | PostgreSQL host                                 | `localhost`                  |
| `TINYBIRD_TOKEN`      | Tinybird workspace token (global fallback)      | —                            |
| `TINYBIRD_API_URL`    | Tinybird API base URL                           | `https://api.tinybird.co`    |
| `RAILS_MASTER_KEY`    | Rails credentials decryption key                | —                            |

## Development

```bash
bundle install
bin/rails db:create db:migrate
bin/dev  # starts Rails + Tailwind watcher
```

## Tests

```bash
bin/rails test
```
