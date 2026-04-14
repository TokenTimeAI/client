# Tinybird Schema for ttime.ai

This document describes the ClickHouse/Tinybird data schema used for real-time analytics.

## Data Source: `heartbeat_events`

```sql
SCHEMA >
    `id`               String,
    `user_id`          String,
    `project_id`       Nullable(String),
    `project_name`     Nullable(String),
    `agent_type`       String,
    `entity`           String,
    `entity_type`      String,
    `language`         Nullable(String),
    `branch`           Nullable(String),
    `operating_system` Nullable(String),
    `machine`          Nullable(String),
    `time`             Float64,
    `time_iso`         DateTime,
    `lines_added`      Nullable(Int32),
    `lines_deleted`    Nullable(Int32),
    `tokens_used`      Nullable(Int32),
    `cost_usd`         Nullable(Float64),
    `is_write`         UInt8,
    `metadata`         String

ENGINE "MergeTree"
ENGINE_PARTITION_KEY "toYYYYMM(time_iso)"
ENGINE_SORTING_KEY "user_id, time_iso"
```

## Pipes (API Endpoints)

### `daily_summary` — Coding time per day per user

```sql
%
SELECT
    user_id,
    toDate(time_iso) AS date,
    agent_type,
    language,
    project_name,
    count() AS events,
    sum(tokens_used) AS total_tokens,
    sum(cost_usd) AS total_cost,
    countIf(is_write = 1) AS write_events
FROM heartbeat_events
WHERE user_id = {{ String(user_id, '') }}
  AND date BETWEEN {{ Date(start_date, '2024-01-01') }} AND {{ Date(end_date, '2024-12-31') }}
GROUP BY user_id, date, agent_type, language, project_name
ORDER BY date DESC
```

### `language_breakdown` — Language usage by user

```sql
%
SELECT
    user_id,
    language,
    count() AS events,
    sum(tokens_used) AS total_tokens,
    sum(cost_usd) AS total_cost
FROM heartbeat_events
WHERE user_id = {{ String(user_id, '') }}
  AND toDate(time_iso) >= {{ Date(start_date, '2024-01-01') }}
GROUP BY user_id, language
ORDER BY events DESC
```

### `agent_breakdown` — Agent type usage

```sql
%
SELECT
    user_id,
    agent_type,
    count() AS events,
    sum(tokens_used) AS total_tokens,
    sum(cost_usd) AS total_cost,
    avg(tokens_used) AS avg_tokens_per_event
FROM heartbeat_events
WHERE user_id = {{ String(user_id, '') }}
  AND toDate(time_iso) >= {{ Date(start_date, '2024-01-01') }}
GROUP BY user_id, agent_type
ORDER BY events DESC
```

### `project_activity` — Project-level activity

```sql
%
SELECT
    user_id,
    project_name,
    toDate(time_iso) AS date,
    count() AS events,
    sum(tokens_used) AS total_tokens,
    sum(lines_added) AS lines_added,
    sum(lines_deleted) AS lines_deleted,
    countIf(is_write = 1) AS writes
FROM heartbeat_events
WHERE user_id = {{ String(user_id, '') }}
  AND project_name IS NOT NULL
  AND date >= {{ Date(start_date, '2024-01-01') }}
GROUP BY user_id, project_name, date
ORDER BY date DESC, events DESC
```

## Setup

1. Create a [Tinybird](https://tinybird.co) workspace
2. Create the `heartbeat_events` data source using the schema above
3. Deploy the pipes for analytics queries
4. Set your workspace token:
   - **Per-user**: Set `tinybird_token` on the User record for multi-tenant analytics
   - **Global**: Set the `TINYBIRD_TOKEN` environment variable
5. Optionally set `TINYBIRD_API_URL` if using EU region: `https://api.eu-central-1.aws.tinybird.co`

## Data Flow

```
Agent (Claude Code / Codex / Cursor / etc.)
    ↓ POST /api/v1/heartbeats
Rails (PostgreSQL — stores all events)
    ↓ TinybirdIngestJob (async via SolidQueue)
Tinybird → ClickHouse (real-time analytics)
    ↓ Tinybird Pipes (SQL queries)
Dashboard (charts, summaries, breakdowns)
```
