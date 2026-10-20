package skills

import (
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestValidateInstallRef(t *testing.T) {
	cases := []struct {
		ref, skill string
		ok         bool
	}{
		{"owner/repo", "", true},
		{"anthropics/skills", "webapp-testing", true},
		{"https://github.com/owner/repo.git", "", true},
		{"git@github.com:owner/repo.git", "my_skill.v2", true},
		{"", "", false},
		{"owner/repo extra", "", false},
		{"owner/repo;rm -rf ~", "", false},
		{"owner/repo$(id)", "", false},
		{"owner/repo#skill", "", false},
		{"-g", "", false},
		{"owner/repo", "a b", false},
		{"owner/repo", "x;id", false},
		{"owner/repo", "--global", false},
	}
	for _, c := range cases {
		err := ValidateInstallRef(c.ref, c.skill)
		if (err == nil) != c.ok {
			t.Errorf("ValidateInstallRef(%q, %q) error = %v, want ok=%v", c.ref, c.skill, err, c.ok)
		}
	}
}

// The built-in catalogue must already satisfy the rule a user-defined skill is
// held to, or the validator would be stricter than what ships.
func TestBuiltinSkillsPassValidateInstallRef(t *testing.T) {
	for _, spec := range All {
		if err := ValidateInstallRef(spec.Ref, spec.Skill); err != nil {
			t.Errorf("built-in skill %q: %v", spec.ID, err)
		}
	}
}

// AGENTS.md names webapp-testing as the example of a skill declaring every
// module its instructions use: its scripts are Python Playwright on a browser.
func TestWebappTestingRequiresItsToolchain(t *testing.T) {
	spec := Get(types.SkillWebappTesting)
	if spec == nil {
		t.Fatal("webapp-testing is missing from the catalogue")
	}
	for _, want := range []types.ModuleID{types.ModulePython, types.ModuleChrome} {
		if !slices.Contains(spec.RequiresModules, want) {
			t.Errorf("webapp-testing requires_modules = %v, missing %q", spec.RequiresModules, want)
		}
	}
}
