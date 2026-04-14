package bootstrap

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/tui"
)

func RunSetup(ctx context.Context, cfg config.Config, paths config.Paths) (config.Config, error) {
	model := tui.NewSetupModel(cfg, paths)
	program := tea.NewProgram(model)

	go func() {
		<-ctx.Done()
		program.Quit()
	}()

	finalModel, err := program.Run()
	if err != nil {
		return config.Config{}, err
	}

	setupModel, ok := finalModel.(tui.SetupModel)
	if !ok {
		return config.Config{}, nil
	}

	return setupModel.Result()
}
