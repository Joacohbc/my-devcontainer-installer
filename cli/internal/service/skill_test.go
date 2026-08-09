package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestSkillInstallWritesEveryAgent(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}
	want, err := svc.SkillDocument()
	if err != nil {
		t.Fatalf("SkillDocument: %v", err)
	}

	if err := svc.Install(base, nil, false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, agent := range domain.HostSkillAgents {
		got, err := os.ReadFile(domain.HostSkillPath(base, agent))
		if err != nil {
			t.Fatalf("reading installed skill for %s: %v", agent.ID, err)
		}
		if string(got) != string(want) {
			t.Errorf("installed skill for %s differs from the shipped document", agent.ID)
		}
	}

	// Installing again is a no-op upgrade, never an error.
	if err := svc.Install(base, nil, false); err != nil {
		t.Fatalf("second Install: %v", err)
	}
}

func TestSkillInstallSelectedAgentOnly(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}
	agents, err := domain.ResolveHostSkillAgents([]string{"claude"})
	if err != nil {
		t.Fatalf("ResolveHostSkillAgents: %v", err)
	}
	if err := svc.Install(base, agents, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, agent := range domain.HostSkillAgents {
		_, err := os.Stat(domain.HostSkillPath(base, agent))
		if agent.ID == "claude" && err != nil {
			t.Errorf("expected claude skill to be installed: %v", err)
		}
		if agent.ID != "claude" && err == nil {
			t.Errorf("agent %s must not be touched when only claude was selected", agent.ID)
		}
	}
}

func TestSkillInstallKeepsForeignFileUnlessForced(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}
	agents, err := domain.ResolveHostSkillAgents([]string{"claude"})
	if err != nil {
		t.Fatalf("ResolveHostSkillAgents: %v", err)
	}
	path := domain.HostSkillPath(base, agents[0])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mine := []byte("a skill the user wrote\n")
	if err := os.WriteFile(path, mine, 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// Nothing else to install, so the run reports that it did nothing rather
	// than pretending it succeeded.
	if err := svc.Install(base, agents, false); err == nil {
		t.Error("expected Install to fail when the only target is foreign and --force is not set")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(mine) {
		t.Error("a foreign skill must not be replaced without --force")
	}

	if err := svc.Install(base, agents, true); err != nil {
		t.Fatalf("forced Install: %v", err)
	}
	want, _ := svc.SkillDocument()
	got, _ = os.ReadFile(path)
	if string(got) != string(want) {
		t.Error("--force must replace the foreign skill")
	}
}

func TestSkillStatusReportsEachTarget(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}

	targets, err := svc.Status(base, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(targets) != len(domain.HostSkillAgents) {
		t.Fatalf("Status returned %d targets, want %d", len(targets), len(domain.HostSkillAgents))
	}
	for _, target := range targets {
		if target.State != domain.SkillAbsent {
			t.Errorf("%s: fresh dir must be %q, got %q", target.Agent.ID, domain.SkillAbsent, target.State)
		}
		if target.Installed() {
			t.Errorf("%s: absent target must not report as installed", target.Agent.ID)
		}
	}

	if err := svc.Install(base, nil, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	targets, err = svc.Status(base, nil)
	if err != nil {
		t.Fatalf("Status after install: %v", err)
	}
	for _, target := range targets {
		if target.State != domain.SkillCurrent || !target.Installed() {
			t.Errorf("%s: after install want %q, got %q", target.Agent.ID, domain.SkillCurrent, target.State)
		}
	}
}

func TestSkillRemove(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}
	if err := svc.Install(base, nil, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := svc.Remove(base, nil, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, agent := range domain.HostSkillAgents {
		if _, err := os.Stat(domain.HostSkillPath(base, agent)); !os.IsNotExist(err) {
			t.Errorf("%s: skill still present after Remove", agent.ID)
		}
		if _, err := os.Stat(domain.HostSkillDir(base, agent)); !os.IsNotExist(err) {
			t.Errorf("%s: the emptied skill dir must be cleaned up too", agent.ID)
		}
	}
	// Removing what is not installed is reported, not an error.
	if err := svc.Remove(base, nil, false); err != nil {
		t.Fatalf("Remove on empty base: %v", err)
	}
}

func TestSkillRemoveKeepsForeignFileUnlessForced(t *testing.T) {
	base := t.TempDir()
	svc := SkillService{Report: nopReporter{}}
	agents, err := domain.ResolveHostSkillAgents([]string{"agents"})
	if err != nil {
		t.Fatalf("ResolveHostSkillAgents: %v", err)
	}
	path := domain.HostSkillPath(base, agents[0])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	if err := svc.Remove(base, agents, false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("a foreign skill must survive Remove without --force")
	}

	if err := svc.Remove(base, agents, true); err != nil {
		t.Fatalf("forced Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("--force must remove the foreign skill")
	}
}

// The host skill teaches an agent which modules, services and variants exist,
// so a value added to the catalog that never reaches the document leaves the
// agent guessing. This is the only place both sides can be compared: service
// may import domain/catalog and infra/assets, neither of which may import the
// other.
func TestHostSkillDocumentCoversTheCatalog(t *testing.T) {
	doc, err := SkillService{Report: nopReporter{}}.SkillDocument()
	if err != nil {
		t.Fatalf("SkillDocument: %v", err)
	}
	body := string(doc)

	for _, id := range catalog.ModuleIDs() {
		if !strings.Contains(body, id) {
			t.Errorf("the host skill must name the %q module (list it under --with)", id)
		}
	}
	for _, id := range catalog.ServiceIDs() {
		if !strings.Contains(body, id) {
			t.Errorf("the host skill must name the %q compose service", id)
		}
	}
	for _, variant := range types.RemoteVariants {
		if !strings.Contains(body, variant) {
			t.Errorf("the host skill must name the %q remote variant", variant)
		}
	}
}

// The document is what an agent matches on and then follows, so its shape (the
// frontmatter an agent runtime parses) and the marker install relies on must
// both hold.
func TestHostSkillDocumentShape(t *testing.T) {
	doc, err := SkillService{Report: nopReporter{}}.SkillDocument()
	if err != nil {
		t.Fatalf("SkillDocument: %v", err)
	}
	body := string(doc)

	if !strings.HasPrefix(body, "---\n") {
		t.Fatal("the skill must open with YAML frontmatter")
	}
	end := strings.Index(body[4:], "\n---\n")
	if end == -1 {
		t.Fatal("the skill's frontmatter is not terminated")
	}
	frontmatter := body[4 : end+4]
	if !strings.Contains(frontmatter, "name: "+domain.HostSkillName) {
		t.Errorf("frontmatter must declare `name: %s`:\n%s", domain.HostSkillName, frontmatter)
	}
	descIdx := strings.Index(frontmatter, "description: ")
	if descIdx == -1 {
		t.Fatalf("frontmatter must declare a description:\n%s", frontmatter)
	}
	if desc := strings.SplitN(frontmatter[descIdx:], "\n", 2)[0]; len(desc) < 80 {
		t.Errorf("the description must explain when to use the skill, got %q", desc)
	}

	if !strings.Contains(body, domain.HostSkillMarker) {
		t.Error("the shipped document must carry the managed marker, or install can never tell its own copies apart from the user's")
	}

	// The habits that keep an agent from hanging on a TUI or writing into a
	// path that does not survive a recreate.
	for _, frag := range []string{
		"--no-interactive",
		"devcontainer.config.json",
		".dc_<workspace>",
		"/workspaces/<workspace>",
		"devcontainer-cli shell --",
		"port-forward",
		"devcontainer-cli context",
	} {
		if !strings.Contains(body, frag) {
			t.Errorf("the host skill must mention %q", frag)
		}
	}
}
