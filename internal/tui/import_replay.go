package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/replay"
)

type replayProgressMsg struct {
	progress replay.Progress
}

type replayFinishedMsg struct {
	summary replay.Summary
	err     error
}

type replayTickMsg time.Time

type replayProgressClosedMsg struct{}

type ImportReplayModel struct {
	ctx       context.Context
	cfg       config.Config
	paths     config.Paths
	options   replay.Options
	progressC chan replay.Progress

	frame    int
	progress replay.Progress
	summary  replay.Summary
	err      error
	done     bool
}

func NewImportReplayModel(ctx context.Context, cfg config.Config, paths config.Paths, options replay.Options) ImportReplayModel {
	return ImportReplayModel{
		ctx:       ctx,
		cfg:       cfg,
		paths:     paths,
		options:   options,
		progressC: make(chan replay.Progress, 16),
		progress: replay.Progress{
			Stage:   "init",
			Message: "Starting replay",
		},
	}
}

func (m ImportReplayModel) Init() tea.Cmd {
	return tea.Batch(
		replayTickCmd(),
		listenReplayProgressCmd(m.progressC),
		runImportReplayCmd(m.ctx, m.cfg, m.paths, m.options, m.progressC),
	)
}

func (m ImportReplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		switch typed.String() {
		case "ctrl+c", "esc":
			m.err = context.Canceled
			m.done = true
			return m, tea.Quit
		case "enter", "q":
			if m.done {
				return m, tea.Quit
			}
		}
	case replayTickMsg:
		if m.done {
			return m, nil
		}
		m.frame = (m.frame + 1) % len(replayFrames)
		return m, replayTickCmd()
	case replayProgressMsg:
		m.progress = typed.progress
		if typed.progress.Stage == "done" {
			m.done = true
		}
		if m.done {
			return m, nil
		}
		return m, listenReplayProgressCmd(m.progressC)
	case replayProgressClosedMsg:
		return m, nil
	case replayFinishedMsg:
		if typed.err != nil {
			m.err = typed.err
			m.done = true
			return m, nil
		}
		m.summary = typed.summary
		m.done = true
		return m, nil
	}

	return m, nil
}

func (m ImportReplayModel) View() string {
	if m.err != nil {
		return fmt.Sprintf(
			"ttime import replay\n\n%s Replay failed\n\n%s\n\nPress Enter to exit.\n",
			statusGlyph("error"),
			m.err,
		)
	}

	if m.done {
		return fmt.Sprintf(
			"ttime import replay\n\n%s Replay complete\n\nScanned:   %d\nImported:  %d\nUpdated:   %d\nSkipped:   %d\nImport run: %s\n\nPress Enter to exit.\n",
			statusGlyph("done"),
			m.summary.Scanned,
			m.summary.Imported,
			m.summary.Updated,
			m.summary.Skipped,
			m.summary.ImportRun,
		)
	}

	return fmt.Sprintf(
		"ttime import replay\n\n%s %s\n\n%s\n\nScanned:   %d\nImported:  %d\nUpdated:   %d\nSkipped:   %d\nImport run: %s\n\nCtrl+C cancels.\n",
		replayFrames[m.frame],
		stageLabel(m.progress.Stage),
		m.progress.Message,
		m.progress.SessionsSeen,
		m.progress.SessionsImport,
		m.progress.SessionsUpdate,
		m.progress.SessionsSkip,
		displayImportRun(m.progress.ImportRunID),
	)
}

func (m ImportReplayModel) Result() (replay.Summary, error) {
	if m.err != nil {
		return replay.Summary{}, m.err
	}
	return m.summary, nil
}

var replayFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func replayTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return replayTickMsg(t)
	})
}

func listenReplayProgressCmd(progressC <-chan replay.Progress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-progressC
		if !ok {
			return replayProgressClosedMsg{}
		}
		return replayProgressMsg{progress: progress}
	}
}

func runImportReplayCmd(ctx context.Context, cfg config.Config, paths config.Paths, options replay.Options, progressC chan<- replay.Progress) tea.Cmd {
	return func() tea.Msg {
		defer close(progressC)

		summary, err := replay.Run(ctx, cfg, paths, options, func(progress replay.Progress) {
			select {
			case progressC <- progress:
			case <-ctx.Done():
			}
		})
		return replayFinishedMsg{
			summary: summary,
			err:     err,
		}
	}
}

func stageLabel(stage string) string {
	switch stage {
	case "create-run":
		return "Creating import run"
	case "scan":
		return "Scanning"
	case "prepare":
		return "Preparing"
	case "upload":
		return "Uploading"
	case "finalize":
		return "Finalizing"
	case "merge-state":
		return "Saving state"
	case "done":
		return "Done"
	default:
		return "Starting"
	}
}

func displayImportRun(id string) string {
	if strings.TrimSpace(id) == "" {
		return "pending"
	}
	return id
}

func statusGlyph(kind string) string {
	switch kind {
	case "done":
		return "✓"
	case "error":
		return "✗"
	default:
		return "•"
	}
}
