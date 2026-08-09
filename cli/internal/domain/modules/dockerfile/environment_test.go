package dockerfile_test

import (
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
)

func TestEnvValueExpansion(t *testing.T) {
	value := dockerfile.EnvValue("$HOME/.cargo/bin")
	if got := value.ForShell(); got != "$HOME/.cargo/bin" {
		t.Errorf("ForShell must leave $HOME for the shell, got %q", got)
	}
	if got := value.Expanded(); got != "/home/devuser/.cargo/bin" {
		t.Errorf("Expanded must resolve $HOME, got %q", got)
	}
	absolute := dockerfile.EnvValue("/usr/local/go/bin")
	if got := absolute.Expanded(); got != "/usr/local/go/bin" {
		t.Errorf("a value without $HOME must survive expansion unchanged, got %q", got)
	}
}

func TestMergeContainerEnv(t *testing.T) {
	merged := dockerfile.MergeContainerEnv(
		dockerfile.ContainerEnv{PathEntries: []dockerfile.PathEntry{"$HOME/.local/bin"}},
		dockerfile.ContainerEnv{
			Assignments: []dockerfile.EnvVar{{Name: "PNPM_HOME", Value: "$HOME/.local/share/pnpm"}},
			PathEntries: []dockerfile.PathEntry{"$HOME/.local/share/pnpm", "$HOME/.local/bin"},
		},
		dockerfile.ContainerEnv{
			Assignments: []dockerfile.EnvVar{{Name: "PNPM_HOME", Value: "/opt/pnpm"}},
		},
	)

	wantPath := []dockerfile.PathEntry{"$HOME/.local/bin", "$HOME/.local/share/pnpm"}
	if len(merged.PathEntries) != len(wantPath) {
		t.Fatalf("a PATH entry declared twice must appear once, got %v", merged.PathEntries)
	}
	for i, entry := range wantPath {
		if merged.PathEntries[i] != entry {
			t.Errorf("PATH entry %d = %q, want %q (declaration order)", i, merged.PathEntries[i], entry)
		}
	}

	if len(merged.Assignments) != 1 {
		t.Fatalf("a variable assigned twice must appear once, got %v", merged.Assignments)
	}
	if merged.Assignments[0].Value != "/opt/pnpm" {
		t.Errorf("the last assignment must win, got %q", merged.Assignments[0].Value)
	}
}

func TestMergeContainerEnvEmpty(t *testing.T) {
	merged := dockerfile.MergeContainerEnv()
	if !merged.IsEmpty() {
		t.Errorf("merging nothing must yield an empty environment, got %+v", merged)
	}
	if dockerfile.RenderDockerfileEnv(merged) != "" || dockerfile.RenderEnvScript(merged) != "" {
		t.Error("an empty environment must render no Dockerfile fragment at all")
	}
}

func TestRenderDockerfileEnv(t *testing.T) {
	out := dockerfile.RenderDockerfileEnv(dockerfile.ContainerEnv{
		Assignments: []dockerfile.EnvVar{{Name: "BUN_INSTALL", Value: "$HOME/.bun"}},
		PathEntries: []dockerfile.PathEntry{"$HOME/.bun/bin", "/usr/local/go/bin"},
	})

	// This is the target a process started without a shell inherits, so no
	// $HOME may survive: nothing would expand it.
	if strings.Contains(out, "$HOME") {
		t.Errorf("Dockerfile ENV must not reference $HOME:\n%s", out)
	}
	for _, frag := range []string{
		`ENV BUN_INSTALL="/home/devuser/.bun"`,
		`ENV PATH="/home/devuser/.bun/bin:/usr/local/go/bin:$PATH"`,
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("expected %q in:\n%s", frag, out)
		}
	}
}

func TestRenderEnvScript(t *testing.T) {
	out := dockerfile.RenderEnvScript(dockerfile.ContainerEnv{
		Assignments: []dockerfile.EnvVar{{Name: "UV_SYSTEM_PYTHON", Value: "1"}},
		PathEntries: []dockerfile.PathEntry{"$HOME/.local/bin", "$HOME/.cargo/bin"},
	})

	if !strings.Contains(out, dockerfile.EnvScriptFile) {
		t.Errorf("the script must be written to %s:\n%s", dockerfile.EnvScriptFile, out)
	}
	// .zshenv is the load-bearing one: it is the only zsh startup file read by
	// a non-interactive, non-login shell, which is what `ssh <host> <command>`
	// runs.
	for _, rcFile := range []string{".zshenv", ".profile", ".bashrc"} {
		if !strings.Contains(out, rcFile) {
			t.Errorf("the env script must be sourced from %s:\n%s", rcFile, out)
		}
	}
	if !strings.Contains(out, `export UV_SYSTEM_PYTHON=\"1\"`) {
		t.Errorf("expected the assignment to be exported:\n%s", out)
	}
	// Sourcing twice must not grow PATH: a login bash reads .profile, which on
	// Ubuntu sources .bashrc, and both carry the source line.
	if !strings.Contains(out, `case \":\$PATH:\" in`) {
		t.Errorf("every PATH entry must be prepended through a guard:\n%s", out)
	}
	if !strings.Contains(out, `export PATH`) {
		t.Errorf("PATH must end up exported:\n%s", out)
	}
	// The shell expands $HOME itself, so it must survive into the script.
	if !strings.Contains(out, `\$HOME/.cargo/bin`) {
		t.Errorf("the script must keep $HOME unexpanded:\n%s", out)
	}
}

func TestRenderEnvScriptPrependsInDeclarationOrder(t *testing.T) {
	out := dockerfile.RenderEnvScript(dockerfile.ContainerEnv{
		PathEntries: []dockerfile.PathEntry{"/first", "/second"},
	})
	// Prepending one entry at a time reverses them, so the renderer emits them
	// backwards to leave PATH in the order RenderDockerfileEnv writes.
	firstAt := strings.Index(out, "/first")
	secondAt := strings.Index(out, "/second")
	if secondAt > firstAt {
		t.Errorf("entries must be prepended last-to-first so PATH keeps declaration order:\n%s", out)
	}
}
