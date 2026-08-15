package domain

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// SkillDirName is where user-defined agent skills are written and read from.
const SkillDirName = "skills"

// SkillDir is the directory a new user-defined skill is written to.
func SkillDir() string {
	return filepath.Join(GlobalConfigDir(), SkillDirName)
}

// SkillDirs is every directory user-defined skills are read from. Unlike
// ProfileDirs there is no legacy predecessor to also read — skills are new.
func SkillDirs() []string {
	return []string{SkillDir()}
}

// ValidateSkillID rejects empty ids and ids with characters outside the
// [a-zA-Z0-9_-] set used for the on-disk skill manifest name — the same
// charset ValidateProfileID enforces, for the same reason (it becomes a file
// name).
func ValidateSkillID(id string) error {
	if id == "" {
		return fmt.Errorf("skill ID cannot be empty")
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid skill ID: %s. Use only alphanumeric characters, dashes, and underscores", id)
		}
	}
	return nil
}

// ValidateSkills rejects unknown skill ids and an unknown mode, before either
// can reach a compose file that would then install nothing. "Unknown" checks
// both the built-in catalogue and the user's own skills under SkillDirs.
func ValidateSkills(config types.SkillsConfig) error {
	if config.Mode != "" && !slices.Contains(types.SkillModes, config.Mode) {
		return fmt.Errorf("invalid skills mode %q: expected one of %s", config.Mode, skillModeList())
	}
	for _, id := range catalog.ExpandSkillGroups(config.Skills) {
		if catalog.GetAgentSkill(id, SkillDirs()...) == nil {
			return fmt.Errorf("unknown skill: %s", id)
		}
	}
	return nil
}

func skillModeList() string {
	modes := make([]string, 0, len(types.SkillModes))
	for _, m := range types.SkillModes {
		modes = append(modes, string(m))
	}
	return strings.Join(modes, ", ")
}

// ApplySelectedSkills makes the config's module list reflect its skills: the
// skills module carries the installer, and requiring nodejs is what puts npx in
// the image. It is applied to the config rather than resolved at render time so
// the persisted project says plainly which modules it has.
func ApplySelectedSkills(config *types.DevcontainerConfig) {
	if config.Skills.IsEmpty() {
		return
	}
	if slices.ContainsFunc(config.Dockerfile.Modules, func(m types.SelectedModule) bool {
		return m.ID == types.ModuleSkills
	}) {
		return
	}
	config.Dockerfile.Modules = append(config.Dockerfile.Modules, types.SelectedModule{ID: types.ModuleSkills})
}

// SkillRefs are the selected skills as the installer receives them, in catalogue
// order and without duplicates. Unknown ids are skipped; ValidateSkills is what
// turns them into an error.
func SkillRefs(config types.SkillsConfig) []string {
	seen := map[types.SkillID]bool{}
	expanded := catalog.ExpandSkillGroups(config.Skills)
	refs := make([]string, 0, len(expanded))
	for _, spec := range catalog.AllAgentSkills(SkillDirs()...) {
		if !slices.Contains(expanded, spec.ID) || seen[spec.ID] {
			continue
		}
		seen[spec.ID] = true
		refs = append(refs, spec.InstallRef())
	}
	return refs
}

// SkillsEnv is the compose environment carrying the project's skills into the
// container. Both entries are literal values rather than `${NAME:-}` lookups:
// they are the CLI's own answer, not something the user fills into the .env.
func SkillsEnv(config types.SkillsConfig) []string {
	if config.IsEmpty() {
		return nil
	}
	return []string{
		fmt.Sprintf("%s=%s", types.SkillsEnvVar, strings.Join(SkillRefs(config), " ")),
		fmt.Sprintf("%s=%s", types.SkillsModeEnvVar, config.ResolvedMode()),
	}
}

// MissingSkillModules are the modules a selected skill needs and the project
// does not have. They are reported rather than added silently: a skill wanting
// a whole toolchain is a decision for the user, unlike the installer's own
// nodejs requirement.
func MissingSkillModules(config *types.DevcontainerConfig) []types.ModuleID {
	selected := map[types.ModuleID]bool{}
	for _, m := range config.Dockerfile.Modules {
		selected[m.ID] = true
	}
	var missing []types.ModuleID
	expanded := catalog.ExpandSkillGroups(config.Skills.Skills)
	for _, id := range expanded {
		spec := catalog.GetAgentSkill(id, SkillDirs()...)
		if spec == nil {
			continue
		}
		for _, required := range spec.RequiresModules {
			if !selected[required] && !slices.Contains(missing, required) {
				missing = append(missing, required)
			}
		}
	}
	return missing
}
