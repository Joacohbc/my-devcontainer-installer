package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joacohbc/my-devcontainer-installer/cli-go/internal/infra/prompt"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	installSignalHandlers()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
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

var rootCmd = &cobra.Command{
	Use:          "devcontainer-cli",
	Short:        "Generate and manage devcontainer environments",
	Version:      version,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("devcontainer-cli %s\n", version)
	},
}

func handleError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, prompt.ErrCancelled) {
		os.Exit(130)
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	os.Exit(1)
}
