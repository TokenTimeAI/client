package detectors

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func collectAntigravitySQLiteSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	dbs, err := collectAntigravitySQLiteFiles(ctx, root)
	if err != nil {
		return nil, err
	}

	summaries := make([]agentsViewGenericSummary, 0, len(dbs))
	for _, dbPath := range dbs {
		parsed, err := summarizeAntigravitySQLite(dbPath)
		if err != nil {
			continue
		}
		summaries = append(summaries, parsed...)
	}
	return summaries, nil
}

func collectAntigravitySQLiteFiles(ctx context.Context, root string) ([]string, error) {
	conversationsRoot := filepath.Join(root, "conversations")
	if !scanner.DirExists(conversationsRoot) {
		return nil, nil
	}

	paths := make([]string, 0, 16)
	err := filepath.WalkDir(conversationsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".db") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk antigravity conversations: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeAntigravitySQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT idx,
		       step_type,
		       step_payload
		FROM steps
		ORDER BY idx
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := agentsViewGenericSummary{
		SessionID: strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath)),
		Project:   "antigravity",
		FileEdits: make(map[string]scanner.FileEdit),
	}
	readableStrings := 0
	for rows.Next() {
		var idx, stepType int
		var payload []byte
		if err := rows.Scan(&idx, &stepType, &payload); err != nil {
			return nil, err
		}
		for _, text := range extractAntigravityProtoStrings(payload) {
			readableStrings++
			if summary.CWD == "" {
				summary.CWD = workingDirectoryFromGenericContent(text)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if summary.SessionID == "" || readableStrings == 0 {
		return nil, nil
	}

	if info, err := os.Stat(dbPath); err == nil {
		summary.StartedAt = info.ModTime().UTC()
		summary.EndedAt = summary.StartedAt
	}
	if summary.EndedAt.IsZero() {
		return nil, nil
	}
	return []agentsViewGenericSummary{summary}, nil
}

func extractAntigravityProtoStrings(payload []byte) []string {
	seen := make(map[string]bool)
	var strings []string
	collectAntigravityProtoStrings(payload, 0, seen, &strings)
	return strings
}

func collectAntigravityProtoStrings(payload []byte, depth int, seen map[string]bool, stringsOut *[]string) {
	if depth > 4 || len(payload) == 0 {
		return
	}

	for offset := 0; offset < len(payload); {
		key, next, ok := readAntigravityProtoVarint(payload, offset)
		if !ok {
			return
		}
		offset = next

		wireType := int(key & 0x7)
		switch wireType {
		case 0:
			_, next, ok := readAntigravityProtoVarint(payload, offset)
			if !ok {
				return
			}
			offset = next
		case 1:
			if offset+8 > len(payload) {
				return
			}
			offset += 8
		case 2:
			length, next, ok := readAntigravityProtoVarint(payload, offset)
			if !ok || length > uint64(len(payload)-next) {
				return
			}
			offset = next
			chunk := payload[offset : offset+int(length)]
			offset += int(length)

			if text, ok := antigravityReadableString(chunk); ok && !seen[text] {
				seen[text] = true
				*stringsOut = append(*stringsOut, text)
			}
			collectAntigravityProtoStrings(chunk, depth+1, seen, stringsOut)
		case 5:
			if offset+4 > len(payload) {
				return
			}
			offset += 4
		default:
			return
		}
	}
}

func readAntigravityProtoVarint(payload []byte, offset int) (uint64, int, bool) {
	var value uint64
	for shift := 0; shift < 64 && offset < len(payload); shift += 7 {
		b := payload[offset]
		offset++
		value |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return value, offset, true
		}
	}
	return 0, offset, false
}

func antigravityReadableString(payload []byte) (string, bool) {
	if !utf8.Valid(payload) {
		return "", false
	}
	text := strings.TrimSpace(string(payload))
	if len([]rune(text)) < 4 || strings.ContainsRune(text, '\x00') {
		return "", false
	}

	printable := 0
	total := 0
	for _, r := range text {
		total++
		if unicode.IsPrint(r) || r == '\n' || r == '\t' || r == '\r' {
			printable++
		}
	}
	if total == 0 || printable*100/total < 85 {
		return "", false
	}
	return text, true
}
