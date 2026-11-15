package commands

import (
	"strings"
	"testing"
)

func TestAccessSsh_DefaultsToEphemeral(t *testing.T) {
	cmd := newSshCommand()
	eph, err := cmd.Flags().GetBool("ephemeral")
	if err != nil {
		t.Fatalf("flag --ephemeral: %v", err)
	}
	if !eph {
		t.Error("access ssh must default --ephemeral to true")
	}

	if err := cmd.Flags().Parse([]string{"--ephemeral=false"}); err != nil {
		t.Fatalf("parse --ephemeral=false: %v", err)
	}
	eph, _ = cmd.Flags().GetBool("ephemeral")
	if eph {
		t.Error("access ssh explicit --ephemeral=false must survive")
	}
}

func TestAccessForward_DefaultsToEphemeral(t *testing.T) {
	cmd := newPortForwardCommand()
	eph, err := cmd.Flags().GetBool("ephemeral")
	if err != nil {
		t.Fatalf("flag --ephemeral: %v", err)
	}
	if !eph {
		t.Error("access forward must default --ephemeral to true")
	}

	if err := cmd.Flags().Parse([]string{"--ephemeral=false"}); err != nil {
		t.Fatalf("parse --ephemeral=false: %v", err)
	}
	eph, _ = cmd.Flags().GetBool("ephemeral")
	if eph {
		t.Error("access forward explicit --ephemeral=false must survive")
	}
}

func TestAccessForwardVia_RegisteredAndArgsValidated(t *testing.T) {
	root := NewRootCommand("test")
	accessCmd := findCommand(t, root, "access")
	agentCmd := findCommand(t, root, "agent")

	t.Run("access forward via", func(t *testing.T) {
		fwdVia := findCommand(t, findCommand(t, accessCmd, "forward"), "via")
		if fwdVia.Flags().Lookup("container") != nil {
			t.Error("access forward via must not register --container")
		}
		if fwdVia.Flags().Lookup("alias") != nil {
			t.Error("access forward via must not register --alias")
		}
		if fwdVia.ValidArgsFunction == nil {
			t.Error("access forward via must register ValidArgsFunction")
		}
		eph, err := fwdVia.Flags().GetBool("ephemeral")
		if err != nil || !eph {
			t.Errorf("access forward via --ephemeral = %v (err=%v), want true", eph, err)
		}
		if err := fwdVia.Args(fwdVia, []string{"me@host"}); err == nil {
			t.Error("access forward via must reject < 2 args")
		}
		if err := fwdVia.Args(fwdVia, []string{"me@host", "   "}); err == nil {
			t.Error("access forward via must reject blank container")
		}
		if err := fwdVia.Args(fwdVia, []string{"me@host", "ctr"}); err != nil {
			t.Errorf("access forward via rejected 2 valid args: %v", err)
		}
		if err := fwdVia.Args(fwdVia, []string{"me@host", "ctr", "3000"}); err != nil {
			t.Errorf("access forward via rejected 3 valid args: %v", err)
		}
		if err := fwdVia.Args(fwdVia, []string{"me@host", "ctr", "3000", "extra"}); err == nil {
			t.Error("access forward via must reject > 3 args")
		}
	})

	t.Run("agent forward via", func(t *testing.T) {
		agentFwdVia := findCommand(t, findCommand(t, agentCmd, "forward"), "via")
		if agentFwdVia.Flags().Lookup("container") != nil {
			t.Error("agent forward via must not register --container")
		}
		if agentFwdVia.Flags().Lookup("alias") != nil {
			t.Error("agent forward via must not register --alias")
		}
		if agentFwdVia.ValidArgsFunction == nil {
			t.Error("agent forward via must register ValidArgsFunction")
		}
		if err := agentFwdVia.Args(agentFwdVia, []string{"me@host", "ctr"}); err == nil {
			t.Error("agent forward via must require <port-mapping> as 3rd arg")
		}
		if err := agentFwdVia.Args(agentFwdVia, []string{"me@host", "ctr", "   "}); err == nil {
			t.Error("agent forward via must reject blank port mapping")
		}
		if err := agentFwdVia.Args(agentFwdVia, []string{"me@host", "ctr", "3000"}); err != nil {
			t.Errorf("agent forward via rejected valid args: %v", err)
		}
	})
}

func TestAccessSsh_SetupSwitchesToPersistentAndRejectsConflict(t *testing.T) {
	t.Run("setup without explicit ephemeral switches to persistent", func(t *testing.T) {
		cmd := newSshCommand()
		if err := cmd.Flags().Parse([]string{"--setup"}); err != nil {
			t.Fatalf("parse --setup: %v", err)
		}
		ephemeral, forceSetup, err := resolveSshEphemeral(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !forceSetup {
			t.Error("expected forceSetup=true")
		}
		if ephemeral {
			t.Error("expected --setup to switch to persistent mode (ephemeral=false)")
		}
	})

	t.Run("setup with explicit ephemeral=false stays persistent", func(t *testing.T) {
		cmd := newSshCommand()
		if err := cmd.Flags().Parse([]string{"--setup", "--ephemeral=false"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}
		ephemeral, forceSetup, err := resolveSshEphemeral(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !forceSetup || ephemeral {
			t.Errorf("got ephemeral=%v, forceSetup=%v; want false, true", ephemeral, forceSetup)
		}
	})

	t.Run("setup with explicit ephemeral=true returns conflict error", func(t *testing.T) {
		cmd := newSshCommand()
		if err := cmd.Flags().Parse([]string{"--setup", "--ephemeral=true"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}
		err := runSsh(cmd, nil)
		if err == nil {
			t.Fatal("expected conflict error for --setup --ephemeral=true")
		}
		if !strings.Contains(err.Error(), "--setup") || !strings.Contains(err.Error(), "--ephemeral") {
			t.Errorf("error = %q, want mention of --setup and --ephemeral conflict", err.Error())
		}
	})
}

func TestAccessForward_AliasSwitchesToPersistentAndRejectsConflict(t *testing.T) {
	t.Run("alias without explicit ephemeral switches to persistent", func(t *testing.T) {
		cmd := newPortForwardCommand()
		if err := cmd.Flags().Parse([]string{"--alias", "foo"}); err != nil {
			t.Fatalf("parse --alias: %v", err)
		}
		ephemeral, alias, err := resolveForwardEphemeral(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alias != "foo" {
			t.Errorf("alias = %q, want %q", alias, "foo")
		}
		if ephemeral {
			t.Error("expected --alias to switch to persistent mode (ephemeral=false)")
		}
	})

	t.Run("alias with explicit ephemeral=false stays persistent", func(t *testing.T) {
		cmd := newPortForwardCommand()
		if err := cmd.Flags().Parse([]string{"--alias", "foo", "--ephemeral=false"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}
		ephemeral, alias, err := resolveForwardEphemeral(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alias != "foo" || ephemeral {
			t.Errorf("got ephemeral=%v, alias=%q; want false, %q", ephemeral, alias, "foo")
		}
	})

	t.Run("alias with explicit ephemeral=true returns conflict error", func(t *testing.T) {
		cmd := newPortForwardCommand()
		if err := cmd.Flags().Parse([]string{"--alias", "foo", "--ephemeral=true"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}
		err := runPortForward(cmd, nil)
		if err == nil {
			t.Fatal("expected conflict error for --alias foo --ephemeral=true")
		}
		if !strings.Contains(err.Error(), "--alias") || !strings.Contains(err.Error(), "--ephemeral") {
			t.Errorf("error = %q, want mention of --alias and --ephemeral conflict", err.Error())
		}
	})
}

func TestRunPortForward_NonInteractiveWithoutArgsOrConfigErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	root := NewRootCommand("test")
	cmd := findCommand(t, findCommand(t, root, "access"), "forward")
	if err := cmd.Flags().Parse([]string{"--no-interactive"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := runPortForward(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "port mapping is required in non-interactive mode") {
		t.Errorf("error = %v, want 'port mapping is required in non-interactive mode'", err)
	}
}
