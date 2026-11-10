package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand_RegistrationAndFlags(t *testing.T) {
	cmd := newInitCommand()
	if cmd.Name() != "init" {
		t.Errorf("expected command name 'init', got %s", cmd.Name())
	}

	flags := []string{
		flagMode, flagRegistry, flagModules, flagServices,
		flagWorkspace, flagPorts, flagVolumes, flagForwardPorts,
		flagSharedConfig, flagProfile, flagNoUp, flagBuild,
	}

	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be defined on init command", f)
		}
	}

	for _, removed := range []string{"with", "service", "no-build", "non-interactive", "force"} {
		if cmd.Flags().Lookup(removed) != nil {
			t.Errorf("expected flag --%s to be removed from init command", removed)
		}
	}
}

func TestInitCommand_Execution(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test-init"}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := newInitCommand()
	cmd.SetArgs([]string{dir, "--build=false", "--no-up"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() failed: %v", err)
	}

	cfgPath := filepath.Join(dir, "devcontainer.config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("expected devcontainer.config.json to exist at %s", cfgPath)
	}
}
