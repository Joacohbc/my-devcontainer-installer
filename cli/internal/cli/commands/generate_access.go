package commands

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

const (
	accessSSHEphemeral = "ssh-ephemeral"
	accessSSHSetup     = "ssh-setup"
	accessShell        = "shell"
	accessNone         = "none"
)

const (
	upHint     = "Start it with 'devcontainer-cli workspace up', then get in with 'devcontainer-cli access shell', 'access ssh' or 'access ssh --ephemeral'."
	accessHint = "Get in with 'devcontainer-cli access shell', 'access ssh' or 'access ssh --ephemeral'."
)

func accessOptions() []service.Option {
	return []service.Option{
		{Value: accessSSHEphemeral, Label: "SSH, ephemeral (no changes to ~/.ssh/config)"},
		{Value: accessSSHSetup, Label: "SSH, create key and register host"},
		{Value: accessShell, Label: "Shell (docker exec)"},
		{Value: accessNone, Label: "None"},
	}
}

type accessInvocation struct {
	newCommand func() *cobra.Command
	flags      []string
}

var accessInvocations = map[string]accessInvocation{
	accessSSHEphemeral: {newSshCommand, []string{"--ephemeral"}},
	accessSSHSetup:     {newSshCommand, []string{"--setup"}},
	accessShell:        {newShellCommand, nil},
}

type nextStepHooks struct {
	confirmUp  func() (bool, error)
	up         func() error
	pickAccess func() (string, error)
	runAccess  func(choice string) error
}

func offerUpAndAccess(h nextStepHooks) error {
	ok, err := h.confirmUp()
	if err != nil {
		return err
	}
	if !ok {
		console.Info(upHint)
		return nil
	}
	if err := h.up(); err != nil {
		return err
	}
	choice, err := h.pickAccess()
	if err != nil {
		return err
	}
	if _, runnable := accessInvocations[choice]; !runnable {
		console.Info(accessHint)
		return nil
	}
	if err := h.runAccess(choice); err != nil {
		console.Warn("Access session ended with an error: %v", err)
	}
	return nil
}

func interactiveNextSteps(composeFile, workspace string) nextStepHooks {
	return nextStepHooks{
		confirmUp: func() (bool, error) {
			return console.ConfirmDefault("Run 'docker compose up -d' now?", true)
		},
		up: func() error {
			return service.LifecycleService{Report: ui.Console{}}.Up(composeFile, workspace, false)
		},
		pickAccess: func() (string, error) {
			opts := accessOptions()
			picked, err := console.Select("Open a session now?", opts, opts[0])
			return picked.Value, err
		},
		runAccess: runAccessCommand,
	}
}

func runAccessCommand(choice string) error {
	inv, ok := accessInvocations[choice]
	if !ok {
		return nil
	}
	cmd := inv.newCommand()
	if err := cmd.ParseFlags(inv.flags); err != nil {
		return err
	}
	return cmd.RunE(cmd, cmd.Flags().Args())
}
