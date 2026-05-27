package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/commands"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/prompt"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	installSignalHandlers()
	commands.CleanupStaleUpdate()
	root := commands.NewRootCommand(version)
	if err := root.Execute(); err != nil {
		handleError(err)
	}
}

func installSignalHandlers() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		os.Exit(130)
	}()
}

func handleError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, prompt.ErrCancelled) {
		fmt.Fprintln(os.Stderr, "\nCancelled.")
		os.Exit(130)
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	os.Exit(1)
}
