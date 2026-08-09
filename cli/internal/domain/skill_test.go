package domain_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestHostSkillAgents_ResolveAndPaths(t *testing.T) {
	ids := domain.HostSkillAgentIDs()
	if len(ids) == 0 {
		t.Fatal("expected at least one host skill agent")
	}
	for _, id := range ids {
		agent, ok := domain.HostSkillAgentByID(id)
		if !ok {
			t.Fatalf("HostSkillAgentByID(%q) did not resolve", id)
		}
		path := domain.HostSkillPath("/home/u", agent)
		want := filepath.Join("/home/u", agent.Dir, domain.HostSkillName, domain.HostSkillFile)
		if path != want {
			t.Errorf("HostSkillPath = %q, want %q", path, want)
		}
		if dir := domain.HostSkillDir("/home/u", agent); dir != filepath.Dir(want) {
			t.Errorf("HostSkillDir = %q, want %q", dir, filepath.Dir(want))
		}
	}
	// The two directories the container entrypoint links the in-image skill
	// into are the same two the host skill is installed into; keep them.
	for _, id := range []string{"claude", "agents"} {
		if _, ok := domain.HostSkillAgentByID(id); !ok {
			t.Errorf("expected agent %q to be a host skill target", id)
		}
	}
}

func TestResolveHostSkillAgents(t *testing.T) {
	all, err := domain.ResolveHostSkillAgents(nil)
	if err != nil {
		t.Fatalf("empty selection: %v", err)
	}
	if len(all) != len(domain.HostSkillAgents) {
		t.Errorf("empty selection must mean every agent, got %d of %d", len(all), len(domain.HostSkillAgents))
	}

	one, err := domain.ResolveHostSkillAgents([]string{"claude"})
	if err != nil {
		t.Fatalf("claude: %v", err)
	}
	if len(one) != 1 || one[0].ID != "claude" {
		t.Errorf("expected only claude, got %v", one)
	}

	if _, err := domain.ResolveHostSkillAgents([]string{"claude", "nope"}); err == nil {
		t.Error("expected an unknown agent id to be rejected")
	}
}

func TestHostSkillNpxCommand(t *testing.T) {
	got := domain.HostSkillNpxCommand()
	for _, frag := range []string{"npx skills add", domain.HostSkillRepo, domain.HostSkillName, "-g"} {
		if !strings.Contains(got, frag) {
			t.Errorf("the Skills CLI command must contain %q, got %q", frag, got)
		}
	}
	// The Skills CLI discovers a skill as <dir>/SKILL.md, so the published
	// path must keep that shape or `npx skills add` finds nothing.
	if !strings.HasPrefix(domain.HostSkillRepoPath, "skills/") ||
		!strings.HasSuffix(domain.HostSkillRepoPath, "/"+domain.HostSkillFile) {
		t.Errorf("HostSkillRepoPath = %q, want skills/<name>/%s", domain.HostSkillRepoPath, domain.HostSkillFile)
	}
}

func TestHostSkillBaseDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}
	got, err := domain.HostSkillBaseDir(domain.SkillScopeGlobal, "/somewhere")
	if err != nil {
		t.Fatalf("global scope: %v", err)
	}
	if got != home {
		t.Errorf("global scope = %q, want the home dir %q", got, home)
	}

	got, err = domain.HostSkillBaseDir("", "/somewhere")
	if err != nil {
		t.Fatalf("empty scope: %v", err)
	}
	if got != home {
		t.Errorf("empty scope must default to global, got %q", got)
	}

	got, err = domain.HostSkillBaseDir(domain.SkillScopeProject, "/somewhere")
	if err != nil {
		t.Fatalf("project scope: %v", err)
	}
	if got != "/somewhere" {
		t.Errorf("project scope = %q, want the cwd", got)
	}

	if _, err := domain.HostSkillBaseDir("elsewhere", "/somewhere"); err == nil {
		t.Error("expected an unknown scope to be rejected")
	}
}

func TestClassifyHostSkill(t *testing.T) {
	dir := t.TempDir()
	want := []byte("---\nname: devcontainer-cli\n---\nbody\n<!-- " + domain.HostSkillMarker + " -->\n")

	path := filepath.Join(dir, "SKILL.md")
	if got := domain.ClassifyHostSkill(path, want); got != domain.SkillAbsent {
		t.Errorf("missing file = %q, want %q", got, domain.SkillAbsent)
	}

	writeFile(t, path, want)
	if got := domain.ClassifyHostSkill(path, want); got != domain.SkillCurrent {
		t.Errorf("identical file = %q, want %q", got, domain.SkillCurrent)
	}

	// An older version of our own document: different bytes, same marker.
	writeFile(t, path, []byte("older body\n<!-- "+domain.HostSkillMarker+" -->\n"))
	if got := domain.ClassifyHostSkill(path, want); got != domain.SkillOutdated {
		t.Errorf("older managed file = %q, want %q", got, domain.SkillOutdated)
	}

	// A skill the user wrote under the same name must never be classified as
	// ours — that is what keeps install from silently replacing it.
	writeFile(t, path, []byte("my own skill\n"))
	if got := domain.ClassifyHostSkill(path, want); got != domain.SkillForeign {
		t.Errorf("unmanaged file = %q, want %q", got, domain.SkillForeign)
	}
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
