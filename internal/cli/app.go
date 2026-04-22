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
	"github.com/ttime-ai/ttime/client/internal/scanner"
	_ "github.com/ttime-ai/ttime/client/internal/scanner/detectors" // Register all detectors
	"github.com/ttime-ai/ttime/client/internal/service"
	"github.com/ttime-ai/ttime/client/internal/updater"
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
	case "agents":
		return runAgents(ctx, paths, args[1:])
	case "scan":
		return runScan(ctx, paths, args[1:])
	case "import":
		return runImport(ctx, paths, args[1:])
	case "update":
		return runUpdate(ctx, paths, args[1:])
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
	fmt.Printf("Scanner state:    %s\n", paths.ScannerStateFile)
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
	noScan := flags.Bool("no-scan", false, "disable agent database scanning")
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

	// Add scanner if not disabled
	if !*noScan {
		daemon.Scanner = scanner.New(paths.ScannerStateFile, 5*time.Minute)
	}

	if *once {
		result, err := daemon.RunOnce(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon run failed: %v\n", err)
			return 1
		}
		fmt.Printf("processed: queued=%d collected=%d scanned=%d sent=%d\n",
			result.QueuedPreviously, result.Collected, result.Scanned, result.Sent)
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

func runAgents(ctx context.Context, paths config.Paths, args []string) int {
	fmt.Println("Available agent detectors:")
	for _, name := range scanner.ListDetectors() {
		fmt.Printf("  - %s\n", name)
	}

	fmt.Println("\nDetected agents on this system:")
	detected, err := scanner.FindDetectors(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error detecting agents: %v\n", err)
		return 1
	}

	if len(detected) == 0 {
		fmt.Println("  (none detected)")
		return 0
	}

	for _, d := range detected {
		fmt.Printf("  + %s\n", d.Name())
		for _, path := range d.DefaultPaths() {
			if scanner.DirExists(path) {
				fmt.Printf("      path: %s\n", path)
				break
			}
		}
	}

	return 0
}

func runScan(ctx context.Context, paths config.Paths, args []string) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	agentFilter := flags.String("agent", "", "scan only specific agent (e.g., 'cosine', 'cline')")
	scanAll := flags.Bool("all", false, "ignore saved scanner state and scan all detectable conversations")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	scannerStatePath := paths.ScannerStateFile
	if *scanAll {
		scannerStatePath = filepath.Join(os.TempDir(), fmt.Sprintf("ttime-scan-all-%d.json", time.Now().UnixNano()))
		defer os.Remove(scannerStatePath)
	}
	s := scanner.New(scannerStatePath, 5*time.Minute)

	if *agentFilter != "" {
		fmt.Printf("Scanning agent: %s\n", *agentFilter)
		results, err := s.ScanAgent(ctx, *agentFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			return 1
		}
		return printScanResults(results)
	}

	results, err := s.ScanOnce(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		return 1
	}
	return printScanResults(results)
}

func runUpdate(ctx context.Context, paths config.Paths, args []string) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	checkOnly := flags.Bool("check", false, "only check for updates, don't install")
	autoYes := flags.Bool("yes", false, "automatically confirm update installation")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	cfg, err := config.LoadOrDefault(paths.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		return 1
	}

	// Get current version from ldflags or default to "dev"
	version := os.Getenv("TTIME_VERSION")
	if version == "" {
		version = "dev"
	}

	u := updater.New(version, cfg.ServerURL)
	result, err := u.CheckForUpdate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check for updates: %v\n", err)
		return 1
	}

	if !result.UpdateAvailable {
		fmt.Printf("✓ ttime is up to date (version %s)\n", result.CurrentVersion)
		return 0
	}

	fmt.Printf("Update available: %s → %s\n", result.CurrentVersion, result.LatestVersion)
	fmt.Printf("Download: %s\n", result.ReleaseURL)

	if *checkOnly {
		return 0
	}

	// Prompt for confirmation unless --yes
	if !*autoYes {
		fmt.Print("Install update? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Update cancelled.")
			return 0
		}
	}

	fmt.Printf("Downloading ttime %s...\n", result.LatestVersion)
	if err := u.PerformUpdate(result.Asset); err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Successfully updated to ttime %s\n", result.LatestVersion)
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `ttime - local heartbeat daemon client

Usage:
  ttime setup
  ttime status
  ttime daemon [--once] [--no-scan]
  ttime agents          # list detected AI agents
  ttime scan [--agent <name>]  # scan agent databases
  ttime import replay [--all] [--agent <name>]
  ttime update [--check] [--yes]  # check for or install updates
  ttime install
  ttime uninstall
`)
}

func printScanResults(results []scanner.ScanResult) int {
	fmt.Printf("Found %d conversation events:\n\n", len(results))

	for _, r := range results {
		fmt.Printf("Agent:     %s\n", r.AgentType)
		fmt.Printf("Project:   %s\n", r.Project)
		fmt.Printf("Time:      %s\n", r.Timestamp.Format(time.RFC3339))
		if !r.TokenUsageKnown && r.PromptTokens == 0 && r.CompletionTokens == 0 && r.TotalTokens == 0 {
			fmt.Printf("Tokens:    unknown\n")
		} else {
			fmt.Printf("Tokens:    %d prompt + %d completion = %d total\n",
				r.PromptTokens, r.CompletionTokens, r.TotalTokens)
		}
		if r.Model != "" {
			fmt.Printf("Model:     %s\n", r.Model)
		}
		if r.CostUSD > 0 {
			fmt.Printf("Cost:      $%.6f\n", r.CostUSD)
		}
		fmt.Printf("Conv ID:   %s\n", r.ConversationID)
		fmt.Println()
	}

	return 0
}
