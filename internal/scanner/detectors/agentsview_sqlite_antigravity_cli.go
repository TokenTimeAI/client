package detectors

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

const antigravityCLIMaxPlausibleTokens = 2_000_000

type antigravityCLIHistoryEntry struct {
	Workspace string
	Display   string
	Timestamp time.Time
}

type antigravityCLIProtoField struct {
	Number int
	Wire   int
	Varint uint64
	Bytes  []byte
	Nested []antigravityCLIProtoField
}

func collectAntigravityCLISQLiteSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	dbs, err := collectAntigravitySQLiteFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	history := readAntigravityCLIHistory(filepath.Join(root, "history.jsonl"))

	summaries := make([]agentsViewGenericSummary, 0, len(dbs))
	for _, dbPath := range dbs {
		parsed, err := summarizeAntigravityCLISQLite(dbPath, history)
		if err != nil {
			continue
		}
		summaries = append(summaries, parsed...)
	}
	return summaries, nil
}

func summarizeAntigravityCLISQLite(dbPath string, history map[string]antigravityCLIHistoryEntry) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	sessionID := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	entry := history[sessionID]
	summary := agentsViewGenericSummary{
		SessionID: sessionID,
		CWD:       entry.Workspace,
		Project:   projectNameFromPath(entry.Workspace),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	if !entry.Timestamp.IsZero() {
		summary.StartedAt = entry.Timestamp
		summary.EndedAt = entry.Timestamp
	}

	readableStrings, err := visitAntigravityCLISteps(db, &summary)
	if err != nil {
		return nil, err
	}
	visitAntigravityCLIGenerationMetadata(db, &summary)
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" || readableStrings == 0 {
		return nil, nil
	}

	if info, err := os.Stat(dbPath); err == nil {
		if summary.StartedAt.IsZero() {
			summary.StartedAt = info.ModTime().UTC()
		}
		if summary.EndedAt.IsZero() || info.ModTime().After(summary.EndedAt) {
			summary.EndedAt = info.ModTime().UTC()
		}
	}
	if summary.EndedAt.IsZero() {
		return nil, nil
	}
	return []agentsViewGenericSummary{summary}, nil
}

func visitAntigravityCLISteps(db *sql.DB, summary *agentsViewGenericSummary) (int, error) {
	rows, err := db.Query(`
		SELECT idx,
		       step_type,
		       step_payload
		FROM steps
		ORDER BY idx
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	readableStrings := 0
	for rows.Next() {
		var idx, stepType int
		var payload []byte
		if err := rows.Scan(&idx, &stepType, &payload); err != nil {
			return 0, err
		}
		for _, text := range extractAntigravityProtoStrings(payload) {
			readableStrings++
			if summary.CWD == "" {
				summary.CWD = workingDirectoryFromGenericContent(text)
				if summary.Project == "" {
					summary.Project = projectNameFromPath(summary.CWD)
				}
			}
		}
	}
	return readableStrings, rows.Err()
}

func visitAntigravityCLIGenerationMetadata(db *sql.DB, summary *agentsViewGenericSummary) {
	rows, err := db.Query(`SELECT data FROM gen_metadata ORDER BY idx`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return
		}
		fields, ok := parseAntigravityCLIProtoFields(data, 0)
		if !ok {
			continue
		}
		if summary.Model == "" {
			summary.Model = antigravityCLIModelName(fields)
		}
		input, output, reasoning, ok := antigravityCLITokenUsage(fields)
		if ok {
			summary.PromptTokens += input
			summary.CompletionTokens += output + reasoning
		}
	}
}

func readAntigravityCLIHistory(path string) map[string]antigravityCLIHistoryEntry {
	out := make(map[string]antigravityCLIHistoryEntry)
	file, err := os.Open(path)
	if err != nil {
		return out
	}
	defer file.Close()

	lineScanner := bufio.NewScanner(file)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var record struct {
			ConversationID string `json:"conversationId"`
			Workspace      string `json:"workspace"`
			Display        string `json:"display"`
			Timestamp      int64  `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil || record.ConversationID == "" {
			continue
		}
		out[record.ConversationID] = antigravityCLIHistoryEntry{
			Workspace: strings.TrimSpace(record.Workspace),
			Display:   strings.TrimSpace(record.Display),
			Timestamp: unixFlexible(record.Timestamp),
		}
	}
	return out
}

func parseAntigravityCLIProtoFields(payload []byte, depth int) ([]antigravityCLIProtoField, bool) {
	if depth > 4 {
		return nil, false
	}
	fields := make([]antigravityCLIProtoField, 0, 8)
	for offset := 0; offset < len(payload); {
		key, next, ok := readAntigravityProtoVarint(payload, offset)
		if !ok {
			return fields, false
		}
		offset = next
		field := antigravityCLIProtoField{Number: int(key >> 3), Wire: int(key & 0x7)}
		switch field.Wire {
		case 0:
			value, next, ok := readAntigravityProtoVarint(payload, offset)
			if !ok {
				return fields, false
			}
			field.Varint = value
			offset = next
		case 1:
			if offset+8 > len(payload) {
				return fields, false
			}
			offset += 8
		case 2:
			length, next, ok := readAntigravityProtoVarint(payload, offset)
			if !ok || length > uint64(len(payload)-next) {
				return fields, false
			}
			offset = next
			field.Bytes = payload[offset : offset+int(length)]
			offset += int(length)
			if nested, ok := parseAntigravityCLIProtoFields(field.Bytes, depth+1); ok && len(nested) > 0 {
				field.Nested = nested
			}
		case 5:
			if offset+4 > len(payload) {
				return fields, false
			}
			offset += 4
		default:
			return fields, false
		}
		fields = append(fields, field)
	}
	return fields, true
}

func antigravityCLIModelName(fields []antigravityCLIProtoField) string {
	for _, field := range fields {
		if field.Number == 21 || field.Number == 19 {
			if model, ok := antigravityReadableString(field.Bytes); ok && strings.Contains(model, "-") {
				return model
			}
		}
		if model := antigravityCLIModelName(field.Nested); model != "" {
			return model
		}
	}
	return ""
}

func antigravityCLITokenUsage(fields []antigravityCLIProtoField) (input, output, reasoning int, ok bool) {
	if input, output, reasoning, ok := antigravityCLITokenBlock(fields); ok {
		return input, output, reasoning, true
	}
	for _, field := range fields {
		if input, output, reasoning, ok := antigravityCLITokenUsage(field.Nested); ok {
			return input, output, reasoning, true
		}
	}
	return 0, 0, 0, false
}

func antigravityCLITokenBlock(fields []antigravityCLIProtoField) (input, output, reasoning int, ok bool) {
	kind, hasKind := antigravityCLIField(fields, 1)
	out, hasOutput := antigravityCLIField(fields, 2)
	in, hasInput := antigravityCLIField(fields, 5)
	if !hasKind || !hasOutput || !hasInput || kind.Wire != 0 || out.Wire != 0 || in.Wire != 0 {
		return 0, 0, 0, false
	}
	if kind.Varint < 1000 || kind.Varint >= 5000 || out.Varint > antigravityCLIMaxPlausibleTokens ||
		in.Varint > antigravityCLIMaxPlausibleTokens {
		return 0, 0, 0, false
	}
	if reas, hasReasoning := antigravityCLIField(fields, 3); hasReasoning {
		if reas.Wire != 0 || reas.Varint > antigravityCLIMaxPlausibleTokens ||
			out.Varint+reas.Varint > antigravityCLIMaxPlausibleTokens {
			return 0, 0, 0, false
		}
		reasoning = int(reas.Varint)
	}
	return int(in.Varint), int(out.Varint), reasoning, true
}

func antigravityCLIField(fields []antigravityCLIProtoField, number int) (antigravityCLIProtoField, bool) {
	for _, field := range fields {
		if field.Number == number {
			return field, true
		}
	}
	return antigravityCLIProtoField{}, false
}
