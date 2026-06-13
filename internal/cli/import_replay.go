package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ttime-ai/ttime/client/internal/bootstrap"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/replay"
)

func runImport(ctx context.Context, paths config.Paths, args []string) int {
	if len(args) == 0 || args[0] != "replay" {
		fmt.Fprintf(os.Stderr, "usage: ttime import replay [--all] [--agent <name>]\n")
		return 1
	}

	flags := flag.NewFlagSet("import replay", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	replayAll := flags.Bool("all", false, "replay all detectable native-agent sessions")
	agentFilter := flags.String("agent", "", "replay only one agent")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import replay requires a configured client, run `ttime setup`: %v\n", err)
		return 1
	}

	summary, err := bootstrap.RunImportReplay(ctx, cfg, paths, replay.Options{
		ReplayAll:   *replayAll,
		AgentFilter: *agentFilter,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay failed: %v\n", err)
		return 1
	}

	fmt.Printf("replayed: scanned=%d imported=%d updated=%d skipped=%d import_run=%s\n",
		summary.Scanned, summary.Imported, summary.Updated, summary.Skipped, summary.ImportRun)
	return 0
}
