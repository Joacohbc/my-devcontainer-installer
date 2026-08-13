package catalog_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestLoadUserSkills_Valid(t *testing.T) {
	dir := t.TempDir()
	manifest := "id: my-skill\nlabel: My Skill (does X)\nref: owner/repo\nskill: sel\nrequires_modules: [nodejs]\ncontext:\n  title: My Skill\n  body: What it teaches an agent to do.\n"
	if err := os.WriteFile(filepath.Join(dir, "my-skill.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	specs := catalog.LoadUserSkills(dir)
	if len(specs) != 1 {
		t.Fatalf("expected 1 skill, got %d: %+v", len(specs), specs)
	}
	s := specs[0]
	if s.ID != "my-skill" || s.Label != "My Skill (does X)" || s.Ref != "owner/repo" || s.Skill != "sel" {
		t.Errorf("unexpected spec: %+v", s)
	}
	if len(s.RequiresModules) != 1 || s.RequiresModules[0] != types.ModuleNodejs {
		t.Errorf("expected requires_modules [nodejs], got %v", s.RequiresModules)
	}
	if s.Context == nil {
		t.Fatal("expected a Context func from the yaml context block")
	}
	sec := s.Context()
	if sec.Title != "My Skill" || sec.Body != "What it teaches an agent to do." {
		t.Errorf("unexpected context section: %+v", sec)
	}
}

// A skill with no context block simply has nothing to say in CONTEXT.md — not
// an error, the same tolerance a module's Context has.
func TestLoadUserSkills_NoContextIsFine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bare.yml"), []byte("id: bare\nlabel: Bare\nref: owner/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	specs := catalog.LoadUserSkills(dir)
	if len(specs) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(specs))
	}
	if specs[0].Context != nil {
		t.Error("expected no Context func without a context block")
	}
}

// Missing id or ref — the two fields nothing else stands in for — skips the
// entry instead of erroring, so one bad file doesn't take every other skill
// down with it.
func TestLoadUserSkills_SkipsIncomplete(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"no-id.yml":       "label: X\nref: owner/repo\n",
		"no-ref.yml":      "id: no-ref\nlabel: X\n",
		"not-yaml.yml":    "{{{not yaml",
		"good.yml":        "id: good\nref: owner/repo\n",
		"wrong-suffix.sh": "id: nope\nref: owner/repo\n",
	}
	for name, body := range cases {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	specs := catalog.LoadUserSkills(dir)
	if len(specs) != 1 || specs[0].ID != "good" {
		t.Errorf("expected only 'good' to load, got %+v", specs)
	}
}

func TestLoadUserSkills_MissingDir(t *testing.T) {
	if specs := catalog.LoadUserSkills(filepath.Join(t.TempDir(), "does-not-exist")); specs != nil {
		t.Errorf("expected nil for a missing directory, got %v", specs)
	}
	if specs := catalog.LoadUserSkills(""); specs != nil {
		t.Errorf("expected nil for an empty directory, got %v", specs)
	}
}

// A user-defined skill shadows a built-in of the same id, same precedence
// catalog.All gives a user profile.
func TestAllAgentSkills_UserShadowsBuiltin(t *testing.T) {
	dir := t.TempDir()
	manifest := "id: firecrawl\nlabel: My Firecrawl Fork\nref: me/firecrawl\n"
	if err := os.WriteFile(filepath.Join(dir, "firecrawl.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	all := catalog.AllAgentSkills(dir)
	found := false
	for _, s := range all {
		if s.ID != types.SkillFirecrawl {
			continue
		}
		found = true
		if s.Ref != "me/firecrawl" {
			t.Errorf("expected the user-defined firecrawl to win, got ref %q", s.Ref)
		}
	}
	if !found {
		t.Fatal("expected firecrawl in the merged list")
	}
	// Built-ins not shadowed still show up.
	if catalog.GetAgentSkill(types.SkillAgentBrowser, dir) == nil {
		t.Error("expected an unshadowed built-in to still resolve")
	}
}

func TestAllAgentSkills_NoDirsIsJustBuiltins(t *testing.T) {
	all := catalog.AllAgentSkills()
	if len(all) != len(catalog.AgentSkills) {
		t.Errorf("got %d, want %d built-ins", len(all), len(catalog.AgentSkills))
	}
}
