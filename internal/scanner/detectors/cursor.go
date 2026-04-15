package detectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"

	_ "github.com/mattn/go-sqlite3"
)

type CursorDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewCursorDetector() scanner.Detector {
	return &CursorDetector{
		BaseDetector: scanner.NewBaseDetector("cursor", "Cursor IDE conversations", []string{
			"~/Library/Application Support/Cursor",
			"~/.config/Cursor",
			"~/AppData/Roaming/Cursor",
			"~/.cursor",
		}, 50),
	}
}

func (d *CursorDetector) Detect(ctx context.Context) (bool, error) {
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

type cursorComposerHeaders struct {
	AllComposers []struct {
		ComposerID          string         `json:"composerId"`
		Name                string         `json:"name"`
		Subtitle            string         `json:"subtitle"`
		CreatedAt           int64          `json:"createdAt"`
		LastUpdatedAt       int64          `json:"lastUpdatedAt"`
		CheckpointAt        int64          `json:"conversationCheckpointLastUpdatedAt"`
		UnifiedMode         string         `json:"unifiedMode"`
		ForceMode           string         `json:"forceMode"`
		TotalLinesAdded     int            `json:"totalLinesAdded"`
		TotalLinesRemoved   int            `json:"totalLinesRemoved"`
		WorkspaceIdentifier map[string]any `json:"workspaceIdentifier"`
	} `json:"allComposers"`
}

func (d *CursorDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	db, err := sql.Open("sqlite3", filepath.Join(d.dataDir, "User", "globalStorage", "state.vscdb"))
	if err != nil {
		return nil, state, fmt.Errorf("open cursor global storage db: %w", err)
	}
	defer db.Close()

	var raw string
	if err := db.QueryRowContext(ctx, "SELECT value FROM ItemTable WHERE key = 'composer.composerHeaders'").Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, state, nil
		}
		return nil, state, fmt.Errorf("query cursor composer headers: %w", err)
	}

	var headers cursorComposerHeaders
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, state, fmt.Errorf("decode cursor composer headers: %w", err)
	}

	sort.Slice(headers.AllComposers, func(i, j int) bool {
		if headers.AllComposers[i].LastUpdatedAt != headers.AllComposers[j].LastUpdatedAt {
			return headers.AllComposers[i].LastUpdatedAt < headers.AllComposers[j].LastUpdatedAt
		}
		return headers.AllComposers[i].ComposerID < headers.AllComposers[j].ComposerID
	})

	results := make([]scanner.ScanResult, 0, len(headers.AllComposers))
	newState := state

	for _, composer := range headers.AllComposers {
		select {
		case <-ctx.Done():
			return results, newState, ctx.Err()
		default:
		}

		sessionID := strings.TrimSpace(composer.ComposerID)
		if sessionID == "" || composer.LastUpdatedAt <= 0 {
			continue
		}

		endUnix := composer.LastUpdatedAt / 1000
		if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && sessionID <= state.LastRecordID) {
			continue
		}

		startedAt := time.UnixMilli(composer.CreatedAt).UTC()
		endedAt := time.UnixMilli(composer.LastUpdatedAt).UTC()
		sessionSeconds := durationSeconds(startedAt, endedAt)

		title := strings.TrimSpace(composer.Name)
		if title == "" {
			title = strings.TrimSpace(composer.Subtitle)
		}
		entity := title
		if entity == "" {
			entity = sessionID
		}

		workspaceID := strings.TrimSpace(stringValue(composer.WorkspaceIdentifier["id"]))
		project := projectNameFromPath(workspaceID)
		if project == "" {
			project = "cursor"
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "cursor",
			Type:                   "conversation",
			Entity:                 entity,
			Time:                   float64(endUnix),
			Timestamp:              endedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(startedAt),
			SessionEndedAt:         timePtr(endedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			ConversationID:         sessionID,
			MessageID:              sessionID,
			Project:                project,
			LinesAdded:             composer.TotalLinesAdded,
			LinesDeleted:           composer.TotalLinesRemoved,
			Metadata: map[string]any{
				"title":                 title,
				"workspace_id":          workspaceID,
				"unified_mode":          composer.UnifiedMode,
				"force_mode":            composer.ForceMode,
				"checkpoint_updated_at": composer.CheckpointAt,
			},
		})

		newState.LastScanTime = endUnix
		newState.LastRecordID = sessionID
	}

	return results, newState, nil
}

func init() {
	scanner.Register(NewCursorDetector)
}
