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

## OAuth Sign-In

GitHub and Google sign-in are available through Devise OmniAuth. Set these environment variables before booting the app to enable the providers:

```bash
GITHUB_CLIENT_ID=your-github-oauth-app-client-id
GITHUB_CLIENT_SECRET=your-github-oauth-app-client-secret
GOOGLE_CLIENT_ID=your-google-oauth-client-id
GOOGLE_CLIENT_SECRET=your-google-oauth-client-secret
```

Callback URLs:

- GitHub: `http://localhost:3000/users/auth/github/callback`
- Google: `http://localhost:3000/users/auth/google_oauth2/callback`

Production uses the same paths on your deployed host, for example:

- `https://your-app.example.com/users/auth/github/callback`
- `https://your-app.example.com/users/auth/google_oauth2/callback`

Account-linking behavior:

- Existing local accounts are linked automatically only when the provider returns a verified email that matches an existing user.
- Google sign-in requires `info.email_verified == true`.
- GitHub sign-in only trusts verified addresses returned from `extra.all_emails`.
- If the provider does not return a verified email, the app does not create or link an account silently; the user is sent back to the sign-in page with an actionable error.

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

## Local daemon client

The repo also includes a Go client under [`client/`](client/) for local forwarding workflows where multiple agent hooks write newline-delimited JSON into a local inbox.

### Install

Install directly with Go:

```bash
go install github.com/ttime-ai/ttime/client/cmd/ttime@latest
```

Or with Homebrew from the project tap:

```bash
brew tap ttime-ai/homebrew-tap
brew install ttime
brew upgrade ttime
```

If you installed via Homebrew, you can run the daemon with `brew services`:

```bash
brew services start ttime
brew services stop ttime
brew services restart ttime
```

If you installed via `go install`, a release archive, or another non-Homebrew path, `ttime install` and `ttime uninstall` remain available to manage the per-user launchd/systemd service directly.

### Usage

```bash
ttime setup
ttime status
ttime daemon --once
```

`ttime setup` launches a small terminal UI that:

1. Prompts for the server URL
2. Starts the device authorization flow
3. Displays the verification URL and user code
4. Polls until the server returns an API key
5. Saves config in the user config directory

For long-running use, install the daemon as a per-user service:

```bash
ttime install
ttime uninstall
```

See [docs/local_daemon.md](docs/local_daemon.md) for the inbox schema, retry queue behavior, and service installation details.
That guide also includes concrete Claude Code and Cursor hook examples for writing heartbeats into the shared inbox.

## Tinybird Analytics

See [docs/tinybird_schema.md](docs/tinybird_schema.md) for the ClickHouse schema and Pipe definitions.

Configure via environment variables:

```bash
TINYBIRD_TOKEN=p.eyJ...   # Workspace token
TINYBIRD_API_URL=https://api.tinybird.co  # Default
```

## Billing / Stripe setup

The app supports exactly one hosted Stripe subscription for a **personal plan at $20/month**. Team billing is not self-serve; the in-app billing page sends team buyers to contact us instead.

### Required Stripe env vars

| Variable | Required | Description |
|---|---|---|
| `STRIPE_SECRET_KEY` | Yes | Secret API key used by the Stripe Ruby SDK for checkout + billing portal. |
| `STRIPE_PERSONAL_MONTHLY_PRICE_ID` | Yes | Stripe Price ID for the $20/month recurring personal plan. |
| `STRIPE_WEBHOOK_SECRET` | Yes | Signing secret for the `/stripe/webhooks` endpoint. |
| `STRIPE_PUBLISHABLE_KEY` | Optional | Not required for the current hosted checkout/portal flow, but available if you later add client-side Stripe.js. |
| `STRIPE_PERSONAL_MONTHLY_PRICE_CENTS` | Optional | Display-only fallback for the plan amount on the billing page. Defaults to `2000`. |
| `STRIPE_PERSONAL_MONTHLY_PRICE_LABEL` | Optional | Display-only label shown on the billing page. Defaults to `$20.00/month`. |

### Stripe setup steps

1. Create a recurring monthly product/price in Stripe for the personal plan at **$20/month**.
2. Save the resulting Price ID into `STRIPE_PERSONAL_MONTHLY_PRICE_ID`.
3. Set `STRIPE_SECRET_KEY` in your environment.
4. Point a Stripe webhook endpoint at `POST /stripe/webhooks`. Subscribe at minimum to:
   - `checkout.session.completed`
   - `customer.subscription.created`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
5. Save the Stripe webhook signing secret as `STRIPE_WEBHOOK_SECRET`.
6. Signed-in users can then visit `/billing` to start checkout or manage an existing subscription in the Stripe Billing Portal.
7. Team customers should use the contact link on `/billing`; there is no self-serve team checkout flow.

### Local webhook development

For local testing, run Stripe's webhook forwarder and point it at the Rails app:

```bash
stripe listen --forward-to localhost:3000/stripe/webhooks
```

Copy the reported signing secret into `STRIPE_WEBHOOK_SECRET`.

Create the personal plan in Stripe first, then export the matching app configuration:

```bash
export STRIPE_SECRET_KEY=sk_test_...
export STRIPE_WEBHOOK_SECRET=whsec_...
export STRIPE_PERSONAL_MONTHLY_PRICE_ID=price_...
export STRIPE_PERSONAL_MONTHLY_PRICE_LABEL='$20.00/month'
```

In development you can then:

```bash
bin/rails server
stripe listen --forward-to localhost:3000/stripe/webhooks
```

Sign in, open `http://localhost:3000/billing`, and start checkout from the billing page. After completing a test checkout, use the Stripe CLI to replay or trigger subscription lifecycle events against the webhook endpoint as needed.

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
| `GITHUB_CLIENT_ID`    | GitHub OAuth app client ID for Devise OmniAuth  | —                            |
| `GITHUB_CLIENT_SECRET`| GitHub OAuth app client secret                  | —                            |
| `GOOGLE_CLIENT_ID`    | Google OAuth client ID for Devise OmniAuth      | —                            |
| `GOOGLE_CLIENT_SECRET`| Google OAuth client secret                      | —                            |

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
