package bootstrap

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/replay"
	"github.com/ttime-ai/ttime/client/internal/tui"
)

func RunImportReplay(ctx context.Context, cfg config.Config, paths config.Paths, options replay.Options) (replay.Summary, error) {
	model := tui.NewImportReplayModel(ctx, cfg, paths, options)
	program := tea.NewProgram(model)

	go func() {
		<-ctx.Done()
		program.Quit()
	}()

	finalModel, err := program.Run()
	if err != nil {
		return replay.Summary{}, err
	}

	replayModel, ok := finalModel.(tui.ImportReplayModel)
	if !ok {
		return replay.Summary{}, nil
	}

	return replayModel.Result()
}
