package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/bootstrap"
	"github.com/ttime-ai/ttime/client/internal/collector"
	"github.com/ttime-ai/ttime/client/internal/config"
	"github.com/ttime-ai/ttime/client/internal/platform"
	"github.com/ttime-ai/ttime/client/internal/queue"
	"github.com/ttime-ai/ttime/client/internal/service"
)

func Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve config paths: %v\n", err)
		return 1
	}

	switch args[0] {
	case "setup":
		return runSetup(ctx, paths)
	case "status":
		return runStatus(ctx, paths)
	case "daemon":
		return runDaemon(ctx, paths, args[1:])
	case "install":
		return runInstall(paths)
	case "uninstall":
		return runUninstall()
	case "-h", "--help", "help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 1
	}
}

func runSetup(ctx context.Context, paths config.Paths) int {
	cfg, err := config.LoadOrDefault(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		return 1
	}

	result, err := bootstrap.RunSetup(ctx, cfg, paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
		return 1
	}

	fmt.Printf("Configured %s for %s\n", result.ServerURL, result.MachineName)
	return 0
}

func runStatus(ctx context.Context, paths config.Paths) int {
	cfg, err := config.LoadOrDefault(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		return 1
	}

	status, err := platform.NewUserServiceManager().Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to inspect daemon service: %v\n", err)
		return 1
	}

	authenticatedUser := "not configured"
	if cfg.APIKey != "" && cfg.ServerURL != "" {
		user, currentUserErr := api.NewClient(cfg.ServerURL, cfg.APIKey).CurrentUser(ctx)
		switch {
		case currentUserErr == nil && user.Email != "":
			authenticatedUser = user.Email
			if user.Name != "" {
				authenticatedUser = fmt.Sprintf("%s (%s)", user.Name, user.Email)
			}
		case cfg.AuthenticatedEmail != "":
			authenticatedUser = cfg.AuthenticatedEmail + " (cached)"
		default:
			authenticatedUser = "configured, validation failed"
		}
	}

	fmt.Printf("Server:           %s\n", cfg.ServerURL)
	fmt.Printf("Machine:          %s\n", cfg.MachineName)
	fmt.Printf("Inbox dir:        %s\n", cfg.InboxDir)
	fmt.Printf("Poll interval:    %ds\n", cfg.PollIntervalSeconds)
	fmt.Printf("Config file:      %s\n", paths.ConfigFile)
	fmt.Printf("Queue file:       %s\n", paths.QueueFile)
	fmt.Printf("Collector state:  %s\n", paths.CollectorStateFile)
	fmt.Printf("Daemon manager:   %s\n", status.Manager)
	fmt.Printf("Daemon installed: %t\n", status.Installed)
	if status.UnitPath != "" {
		fmt.Printf("Daemon unit path: %s\n", status.UnitPath)
	}
	fmt.Printf("Authenticated:    %s\n", authenticatedUser)

	return 0
}

func runDaemon(ctx context.Context, paths config.Paths, args []string) int {
	flags := flag.NewFlagSet("daemon", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	once := flags.Bool("once", false, "process queued and inbox events once, then exit")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon requires a configured client, run `ttime setup`: %v\n", err)
		return 1
	}

	daemon := service.Daemon{
		Collector:    collector.NewJSONLCollector(cfg.InboxDir, paths.CollectorStateFile),
		Queue:        queue.New(paths.QueueFile),
		Sender:       api.NewClient(cfg.ServerURL, cfg.APIKey),
		MachineName:  cfg.MachineName,
		PollInterval: time.Duration(cfg.PollIntervalSeconds) * time.Second,
	}

	if *once {
		result, err := daemon.RunOnce(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon run failed: %v\n", err)
			return 1
		}
		fmt.Printf("processed: queued=%d collected=%d sent=%d\n", result.QueuedPreviously, result.Collected, result.Sent)
		return 0
	}

	if err := daemon.RunLoop(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "daemon exited with error: %v\n", err)
		return 1
	}
	return 0
}

func runInstall(paths config.Paths) int {
	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install requires a configured client, run `ttime setup`: %v\n", err)
		return 1
	}
	_ = cfg

	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve executable path: %v\n", err)
		return 1
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		binaryPath = filepath.Clean(binaryPath)
	}

	if err := platform.NewUserServiceManager().Install(binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		return 1
	}

	fmt.Printf("Installed daemon service for %s\n", binaryPath)
	return 0
}

func runUninstall() int {
	if err := platform.NewUserServiceManager().Uninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "uninstall failed: %v\n", err)
		return 1
	}

	fmt.Println("Removed daemon service")
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `ttime - local heartbeat daemon client

Usage:
  ttime setup
  ttime status
  ttime daemon [--once]
  ttime install
  ttime uninstall
`)
}
