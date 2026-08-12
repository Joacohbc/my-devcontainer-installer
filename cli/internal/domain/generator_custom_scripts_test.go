package domain_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func withScripts(scripts ...types.CustomScript) func(*types.DevcontainerConfig) {
	return func(c *types.DevcontainerConfig) { c.Dockerfile.Scripts = scripts }
}

// A `when: build` script is COPYed into a staging dir, run as devuser, and the
// staging dir removed in the same layer so nothing of it survives.
func TestGenerateDockerfile_CustomBuildScript(t *testing.T) {
	df := mustGenerateDockerfile(t, makeConfig(withScripts(types.CustomScript{File: "install-terraform.sh"})))

	assertContainsStr(t, df, "COPY custom-install-terraform.sh /tmp/devcontainer-custom-scripts/", "custom build script copy")
	assertContainsStr(t, df, "su - devuser -c '/tmp/devcontainer-custom-scripts/custom-install-terraform.sh'", "custom build script run")
	assertContainsStr(t, df, "rm -rf /tmp/devcontainer-custom-scripts", "custom build script cleanup")
}

// The block must come after every module: the leading layers have to stay
// byte-identical across variants for the devcontainer-base cache, and a user
// script is meant to build on the toolchain the modules installed.
func TestGenerateDockerfile_CustomBuildScriptRunsLast(t *testing.T) {
	cfg := makeConfig(withScripts(types.CustomScript{File: "after.sh"}))
	cfg.Dockerfile.Modules = []types.SelectedModule{{ID: types.ModuleGolang}}
	df := mustGenerateDockerfile(t, cfg)

	custom := strings.Index(df, "/tmp/devcontainer-custom-scripts")
	if custom < 0 {
		t.Fatal("expected the custom script block to be rendered")
	}
	for _, earlier := range []string{"FROM ubuntu:24.04", "COPY alias.sh", "COPY golang_utils.sh"} {
		if idx := strings.Index(df, earlier); idx < 0 || idx > custom {
			t.Errorf("expected %q (at %d) to render before the custom scripts (at %d)", earlier, idx, custom)
		}
	}
	// The labels close the file, so the custom block still precedes them.
	if labels := strings.Index(df, "LABEL "); labels >= 0 && labels < custom {
		t.Errorf("expected the custom scripts (at %d) to render before the labels (at %d)", custom, labels)
	}
}

// `when: start` reuses the auto-start machinery: the entrypoint's sorted glob
// over start.d/ runs it once per container.
func TestGenerateDockerfile_CustomStartScript(t *testing.T) {
	df := mustGenerateDockerfile(t, makeConfig(withScripts(
		types.CustomScript{File: "vpn-login.sh", When: types.ScriptWhenStart},
	)))

	assertContainsStr(t, df, "COPY custom-vpn-login.sh /home/devuser/post-script/start.d/90-custom-vpn-login.sh", "custom start script")
	// It is also reachable by hand under its plain name, like every other
	// auto-start script.
	assertContainsStr(t, df, "ln -sfn start.d/90-custom-vpn-login.sh /home/devuser/post-script/custom-vpn-login.sh", "custom start script symlink")
	assertNotContainsStr(t, df, "/tmp/devcontainer-custom-scripts", "a start script must not run at build time")
}

// The user's start scripts run after every module-provided installer, so they
// can build on what those set up.
func TestGenerateDockerfile_CustomStartScriptRunsAfterModules(t *testing.T) {
	cfg := makeConfig(withScripts(types.CustomScript{File: "last.sh", When: types.ScriptWhenStart}))
	cfg.Dockerfile.Modules = []types.SelectedModule{{ID: types.ModuleClaudeCode}}
	df := mustGenerateDockerfile(t, cfg)

	if types.CustomScriptStartOrder <= types.DefaultPostScriptStartOrder {
		t.Fatalf("custom start scripts must be ordered after module ones, got %d vs %d",
			types.CustomScriptStartOrder, types.DefaultPostScriptStartOrder)
	}
	assertContainsStr(t, df, "start.d/50-install-claude-code.sh", "module auto-start script")
	assertContainsStr(t, df, "start.d/90-custom-last.sh", "custom auto-start script")
}

// `when: manual` only copies the script; nothing ever runs it on its own.
func TestGenerateDockerfile_CustomManualScript(t *testing.T) {
	df := mustGenerateDockerfile(t, makeConfig(withScripts(
		types.CustomScript{File: "reset-db.sh", When: types.ScriptWhenManual},
	)))

	// The manual scripts share one COPY line with the module-provided ones.
	assertContainsStr(t, df, "custom-reset-db.sh /home/devuser/post-script/", "custom manual script")
	assertNotContainsStr(t, df, "/tmp/devcontainer-custom-scripts", "a manual script must not run at build time")
	assertNotContainsStr(t, df, "start.d/90-custom-reset-db.sh", "a manual script must not auto-start")
}

func TestGenerateDockerfile_NoCustomScriptsEmitsNoBlock(t *testing.T) {
	df := mustGenerateDockerfile(t, makeConfig())
	assertNotContainsStr(t, df, "CUSTOM SCRIPTS", "a config with no scripts")
	assertNotContainsStr(t, df, "/tmp/devcontainer-custom-scripts", "a config with no scripts")
}

// An invalid entry must fail generation rather than emit a Dockerfile that
// interpolates it.
func TestGenerateDockerfile_RejectsInvalidCustomScript(t *testing.T) {
	cfg := makeConfig(withScripts(types.CustomScript{File: "../escape.sh"}))
	if _, err := domain.GenerateDockerfile(cfg); err == nil {
		t.Fatal("expected an error for a script that escapes the profile directory")
	}
}

// Remote mode ships a prebuilt image, so it generates no Dockerfile at all —
// custom scripts included.
func TestGenerateDockerfile_RemoteModeIgnoresCustomScripts(t *testing.T) {
	cfg := makeConfig(withScripts(types.CustomScript{File: "x.sh"}))
	cfg.Mode = types.BuildModeRemote
	df, err := domain.GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if df != "" {
		t.Errorf("expected no Dockerfile in remote mode, got %d bytes", len(df))
	}
}
