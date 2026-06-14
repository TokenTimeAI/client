package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ttime-ai/ttime/client/internal/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli.SetVersion(version)
	os.Exit(cli.Run(ctx, os.Args[1:]))
}
