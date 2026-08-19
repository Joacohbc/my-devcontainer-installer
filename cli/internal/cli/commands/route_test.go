package commands

import (
	"testing"
)

func TestNewRouteCommand_Structure(t *testing.T) {
	cmd := newRouteCommand()
	if cmd.Name() != "route" {
		t.Errorf("command name = %q, want route", cmd.Name())
	}

	wantSubcommands := []string{"add", "ls", "rm", "status", "stop"}
	have := map[string]bool{}
	for _, sub := range cmd.Commands() {
		have[sub.Name()] = true
	}

	for _, name := range wantSubcommands {
		if !have[name] {
			t.Errorf("expected subcommand %q to be registered under route", name)
		}
	}
}

func TestRouteAddCommand_Flags(t *testing.T) {
	cmd := newRouteAddCommand()
	if cmd.Flags().Lookup("domain") == nil {
		t.Error("expected --domain flag on route add")
	}
	if cmd.Flags().Lookup("container") == nil {
		t.Error("expected --container flag on route add")
	}
}
