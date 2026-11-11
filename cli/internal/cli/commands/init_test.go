package commands

import "testing"

func TestInitCommand_IsTheWizard(t *testing.T) {
	cmd := newInitCommand()
	if cmd.Name() != "init" {
		t.Fatalf("expected command name init, got %s", cmd.Name())
	}
	if err := cmd.Args(cmd, []string{"./dir"}); err == nil {
		t.Fatal("init must reject positional arguments")
	}
	if !cmd.Flags().Lookup(flagVersion).Hidden {
		t.Error("--version is a root concern and must stay hidden on init")
	}
	for _, f := range []string{flagMode, flagModules, flagServices, flagProfile, flagSkills, flagBuild} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected wizard flag --%s on init", f)
		}
	}
	if cmd.Flags().Lookup(flagNoUp) != nil {
		t.Error("init must not expose the auto-init only flag --no-up")
	}
}

func TestWorkspace_RegistersInitAndAutoInit(t *testing.T) {
	have := map[string]bool{}
	for _, c := range newWorkspaceCommand().Commands() {
		have[c.Name()] = true
	}
	for _, name := range []string{"init", "auto-init"} {
		if !have[name] {
			t.Errorf("workspace must register %q", name)
		}
	}
}
