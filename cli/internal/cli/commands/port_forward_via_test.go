package commands

import (
	"testing"
)

func TestForwardViaSubcommandRegistration(t *testing.T) {
	accessCmd := newPortForwardCommand()
	viaCmd, _, err := accessCmd.Find([]string{"via"})
	if err != nil || viaCmd == nil || viaCmd.Name() != "via" {
		t.Fatalf("access forward must register 'via' subcommand, got err=%v", err)
	}

	agentCmd := newAgentForwardCommand()
	agentViaCmd, _, err := agentCmd.Find([]string{"via"})
	if err != nil || agentViaCmd == nil || agentViaCmd.Name() != "via" {
		t.Fatalf("agent forward must register 'via' subcommand, got err=%v", err)
	}
}

func TestForwardViaArgValidation(t *testing.T) {
	accessVia := newPortForwardViaCommand()
	if err := accessVia.Args(accessVia, []string{}); err == nil {
		t.Error("access forward via: expected error with 0 args")
	}
	if err := accessVia.Args(accessVia, []string{"host"}); err == nil {
		t.Error("access forward via: expected error with 1 arg")
	}
	if err := accessVia.Args(accessVia, []string{"host", "container"}); err != nil {
		t.Errorf("access forward via: unexpected error with 2 args: %v", err)
	}
	if err := accessVia.Args(accessVia, []string{"host", "container", "3000"}); err != nil {
		t.Errorf("access forward via: unexpected error with 3 args: %v", err)
	}
	if err := accessVia.Args(accessVia, []string{"host", "container", "3000", "extra"}); err == nil {
		t.Error("access forward via: expected error with 4 args")
	}

	agentVia := newAgentForwardViaCommand()
	if err := agentVia.Args(agentVia, []string{"host", "container"}); err == nil {
		t.Error("agent forward via: expected error with 2 args")
	}
	if err := agentVia.Args(agentVia, []string{"host", "container", "3000"}); err != nil {
		t.Errorf("agent forward via: unexpected error with 3 args: %v", err)
	}
	if err := agentVia.Args(agentVia, []string{"host", "container", "3000", "extra"}); err == nil {
		t.Error("agent forward via: expected error with 4 args")
	}
}
