package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/commands"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Set the global context for infra/docker execution
	docker.SetContext(ctx)

	service.CleanupStaleUpdate()
	root := commands.NewRootCommand(version)
	if err := root.ExecuteContext(ctx); err != nil {
		handleError(err)
	}
}

func handleError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, ui.ErrCancelled) {
		fmt.Fprintln(os.Stderr, "\nCancelled.")
		os.Exit(130)
	}
	logger.Std().Error(err.Error())
	os.Exit(1)
}
