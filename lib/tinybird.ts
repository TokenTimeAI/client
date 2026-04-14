/**
 * Tinybird Definitions
 *
 * Define your datasources, endpoints, and client here.
 */

import {
  defineDatasource,
  defineEndpoint,
  Tinybird,
  node,
  t,
  p,
  engine,
  type InferRow,
  type InferParams,
  type InferOutputRow,
} from "@tinybirdco/sdk";

// ============================================================================
// Datasources
// ============================================================================

/**
 * Heartbeat events datasource - tracks coding activity from AI agents
 */
export const heartbeatEvents = defineDatasource("heartbeat_events", {
  description: "Heartbeat events from AI coding agents (Claude Code, Codex, Cursor, etc.)",
  schema: {
    id: t.string(),
    user_id: t.string(),
    project_id: t.string().nullable(),
    project_name: t.string().nullable(),
    agent_type: t.string().lowCardinality(),
    entity: t.string(),
    entity_type: t.string().lowCardinality(),
    language: t.string().lowCardinality().nullable(),
    branch: t.string().nullable(),
    operating_system: t.string().lowCardinality().nullable(),
    machine: t.string().nullable(),
    time: t.float64(),
    time_iso: t.dateTime(),
    lines_added: t.int32().nullable(),
    lines_deleted: t.int32().nullable(),
    tokens_used: t.int32().nullable(),
    cost_usd: t.float64().nullable(),
    is_write: t.uint8(),
    metadata: t.string(),
  },
  engine: engine.mergeTree({
    partitionBy: "toYYYYMM(time_iso)",
    sortingKey: ["user_id", "time_iso"],
  }),
});

export type HeartbeatEventRow = InferRow<typeof heartbeatEvents>;

// ============================================================================
// Endpoints
// ============================================================================

/**
 * Daily summary endpoint - coding time per day per user
 */
export const dailySummary = defineEndpoint("daily_summary", {
  description: "Get coding time per day per user",
  params: {
    user_id: p.string().required(),
    start_date: p.date().optional("2024-01-01"),
    end_date: p.date().optional("2026-12-31"),
  },
  nodes: [
    node({
      name: "endpoint",
      sql: `
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
        WHERE user_id = {{String(user_id)}}
          AND date BETWEEN {{Date(start_date, '2024-01-01')}} AND {{Date(end_date, '2026-12-31')}}
        GROUP BY user_id, date, agent_type, language, project_name
        ORDER BY date DESC
      `,
    }),
  ],
  output: {
    user_id: t.string(),
    date: t.date(),
    agent_type: t.string(),
    language: t.string().nullable(),
    project_name: t.string().nullable(),
    events: t.uint64(),
    total_tokens: t.int64().nullable(),
    total_cost: t.float64().nullable(),
    write_events: t.uint64(),
  },
});

export type DailySummaryParams = InferParams<typeof dailySummary>;
export type DailySummaryOutput = InferOutputRow<typeof dailySummary>;

/**
 * Language breakdown endpoint - language usage by user
 */
export const languageBreakdown = defineEndpoint("language_breakdown", {
  description: "Get language usage breakdown by user",
  params: {
    user_id: p.string().required(),
    start_date: p.date().optional("2024-01-01"),
  },
  nodes: [
    node({
      name: "endpoint",
      sql: `
        SELECT
          user_id,
          language,
          count() AS events,
          sum(tokens_used) AS total_tokens,
          sum(cost_usd) AS total_cost
        FROM heartbeat_events
        WHERE user_id = {{String(user_id)}}
          AND toDate(time_iso) >= {{Date(start_date, '2024-01-01')}}
          AND language IS NOT NULL
        GROUP BY user_id, language
        ORDER BY events DESC
      `,
    }),
  ],
  output: {
    user_id: t.string(),
    language: t.string(),
    events: t.uint64(),
    total_tokens: t.int64().nullable(),
    total_cost: t.float64().nullable(),
  },
});

export type LanguageBreakdownParams = InferParams<typeof languageBreakdown>;
export type LanguageBreakdownOutput = InferOutputRow<typeof languageBreakdown>;

/**
 * Agent breakdown endpoint - agent type usage
 */
export const agentBreakdown = defineEndpoint("agent_breakdown", {
  description: "Get agent type usage breakdown by user",
  params: {
    user_id: p.string().required(),
    start_date: p.date().optional("2024-01-01"),
  },
  nodes: [
    node({
      name: "endpoint",
      sql: `
        SELECT
          user_id,
          agent_type,
          count() AS events,
          sum(tokens_used) AS total_tokens,
          sum(cost_usd) AS total_cost,
          avg(tokens_used) AS avg_tokens_per_event
        FROM heartbeat_events
        WHERE user_id = {{String(user_id)}}
          AND toDate(time_iso) >= {{Date(start_date, '2024-01-01')}}
        GROUP BY user_id, agent_type
        ORDER BY events DESC
      `,
    }),
  ],
  output: {
    user_id: t.string(),
    agent_type: t.string(),
    events: t.uint64(),
    total_tokens: t.int64().nullable(),
    total_cost: t.float64().nullable(),
    avg_tokens_per_event: t.float64().nullable(),
  },
});

export type AgentBreakdownParams = InferParams<typeof agentBreakdown>;
export type AgentBreakdownOutput = InferOutputRow<typeof agentBreakdown>;

/**
 * Project activity endpoint - project-level activity
 */
export const projectActivity = defineEndpoint("project_activity", {
  description: "Get project-level activity by user",
  params: {
    user_id: p.string().required(),
    start_date: p.date().optional("2024-01-01"),
  },
  nodes: [
    node({
      name: "endpoint",
      sql: `
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
        WHERE user_id = {{String(user_id)}}
          AND project_name IS NOT NULL
          AND date >= {{Date(start_date, '2024-01-01')}}
        GROUP BY user_id, project_name, date
        ORDER BY date DESC, events DESC
      `,
    }),
  ],
  output: {
    user_id: t.string(),
    project_name: t.string(),
    date: t.date(),
    events: t.uint64(),
    total_tokens: t.int64().nullable(),
    lines_added: t.int64().nullable(),
    lines_deleted: t.int64().nullable(),
    writes: t.uint64(),
  },
});

export type ProjectActivityParams = InferParams<typeof projectActivity>;
export type ProjectActivityOutput = InferOutputRow<typeof projectActivity>;

// ============================================================================
// Client
// ============================================================================

export const tinybird = new Tinybird({
  datasources: { heartbeatEvents },
  pipes: { dailySummary, languageBreakdown, agentBreakdown, projectActivity },
});
