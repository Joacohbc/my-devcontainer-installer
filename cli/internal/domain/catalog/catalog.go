package catalog

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/skills"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// DockerfileModules is the ordered catalogue of all Dockerfile layer modules.
var DockerfileModules = []*dockerfile.ModuleSpec{
	dockerfile.BaseModule,
	// Always-on, and must stay directly after BaseModule: the zsh installer in
	// BaseModule rewrites ~/.zshrc from scratch, so anything appending to the rc
	// files has to run after it.
	dockerfile.AliasesModule,
	dockerfile.GithubCliModule,
	dockerfile.JavaTemurinModule,
	dockerfile.JavaOpenjdkModule,
	dockerfile.PythonModule,
	dockerfile.SqliteModule,
	dockerfile.GolangModule,
	dockerfile.PhpModule,
	dockerfile.RustModule,
	dockerfile.CCppModule,
	dockerfile.PostgresClientModule,
	dockerfile.RedisClientModule,
	dockerfile.MongoClientModule,
	dockerfile.NodejsModule,
	dockerfile.PnpmModule,
	dockerfile.YarnModule,
	dockerfile.BunModule,
	dockerfile.ClaudeCodeModule,
	dockerfile.OpencodeModule,
	dockerfile.CodexCliModule,
	dockerfile.AntigravityCliModule,
	dockerfile.CopilotCliModule,
	dockerfile.GraphifyModule,
	dockerfile.CavemanModule,
	dockerfile.ClaudeMemModule,
	dockerfile.ContextModeModule,
	dockerfile.ZellijModule,
	dockerfile.ChromeModule,
	dockerfile.FfmpegModule,
	dockerfile.DodModule,
	dockerfile.NgrokModule,
	dockerfile.CloudflaredModule,
	dockerfile.SkillsModule,
	dockerfile.CleanupModule,
}

// ComposeServices is the ordered catalogue of all compose services.
var ComposeServices = []*compose.ServiceSpec{
	compose.DevcontainerService,
	compose.MongoService,
	compose.RedisService,
	compose.PostgresService,
}

// GetDockerfileModule returns the module with the given ID, or nil.
func GetDockerfileModule(id types.ModuleID) *dockerfile.ModuleSpec {
	for _, m := range DockerfileModules {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// GetComposeService returns the service with the given ID, or nil.
func GetComposeService(id types.ServiceID) *compose.ServiceSpec {
	for _, s := range ComposeServices {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// AllModuleIDs returns the IDs of all registered Dockerfile modules.
func AllModuleIDs() []types.ModuleID {
	ids := make([]types.ModuleID, len(DockerfileModules))
	for i, m := range DockerfileModules {
		ids[i] = m.ID
	}
	return ids
}

// ModuleIDs returns every Dockerfile module id as a string, in catalog order.
func ModuleIDs() []string {
	ids := make([]string, len(DockerfileModules))
	for i, m := range DockerfileModules {
		ids[i] = string(m.ID)
	}
	return ids
}

// ServiceIDs returns every compose service id as a string, in catalog order.
func ServiceIDs() []string {
	ids := make([]string, len(ComposeServices))
	for i, s := range ComposeServices {
		ids[i] = string(s.ID)
	}
	return ids
}

// AlwaysOnModules returns modules with Always == true.
func AlwaysOnModules() []*dockerfile.ModuleSpec {
	var out []*dockerfile.ModuleSpec
	for _, m := range DockerfileModules {
		if m.Always {
			out = append(out, m)
		}
	}
	return out
}

// AlwaysOnServices returns services with Always == true.
func AlwaysOnServices() []*compose.ServiceSpec {
	var out []*compose.ServiceSpec
	for _, s := range ComposeServices {
		if s.Always {
			out = append(out, s)
		}
	}
	return out
}

// CategorizedEntry is a user-selectable module or service grouped under a UI
// category for the interactive wizard. IsService distinguishes a compose
// service from a Dockerfile module so the wizard can route the selection.
type CategorizedEntry struct {
	ID        string
	Label     string
	IsService bool
}

// SelectableByCategory groups every user-selectable Dockerfile module and
// compose service by its UICategory. Always-on modules (base/cleanup),
// always-on/internal services (devcontainer), and any entry without a
// UICategory are excluded. Entries keep their catalogue order within a group.
func SelectableByCategory() map[types.UICategory][]CategorizedEntry {
	out := map[types.UICategory][]CategorizedEntry{}
	for _, m := range DockerfileModules {
		if m.Always || m.Internal || m.UICategory == "" {
			continue
		}
		out[m.UICategory] = append(out[m.UICategory], CategorizedEntry{ID: string(m.ID), Label: m.Label})
	}
	for _, s := range ComposeServices {
		if s.Always || s.Internal || s.UICategory == "" {
			continue
		}
		out[s.UICategory] = append(out[s.UICategory], CategorizedEntry{ID: string(s.ID), Label: s.Label, IsService: true})
	}
	return out
}

// CategoriesInOrder returns the UI categories that contain at least one
// selectable entry, in the canonical display order (types.UICategoryOrder).
func CategoriesInOrder() []types.UICategory {
	grouped := SelectableByCategory()
	var out []types.UICategory
	for _, c := range types.UICategoryOrder {
		if len(grouped[c]) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// AgentSkills is the built-in catalogue of installable agent skills.
var AgentSkills = skills.All

// AllAgentSkills is every known skill: the user's own, loaded from dirs, then
// the built-ins not shadowed by one of the same id — the same precedence
// catalog.All gives a user profile over a built-in one. dirs is normally
// domain.SkillDirs(); a caller passing none just gets AgentSkills.
func AllAgentSkills(dirs ...string) []*skills.Spec {
	seen := map[types.SkillID]bool{}
	var out []*skills.Spec
	for _, dir := range dirs {
		for _, s := range LoadUserSkills(dir) {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	for _, s := range AgentSkills {
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out
}

// GetAgentSkill returns the spec for a skill id — built-in, or user-defined
// under one of dirs — or nil when it is unknown.
func GetAgentSkill(id types.SkillID, dirs ...string) *skills.Spec {
	for _, s := range AllAgentSkills(dirs...) {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// ExpandSkillGroups replaces any Category ID or alias in the list with the IDs of its constituent Skills.
func ExpandSkillGroups(ids []types.SkillID) []types.SkillID {
	var expanded []types.SkillID
	for _, id := range ids {
		if cat := skills.GetCategory(id); cat != nil {
			expanded = append(expanded, cat.Skills...)
			continue
		}
		expanded = append(expanded, id)
	}
	return expanded
}

// AgentSkillIDs returns every known skill id — built-in and user-defined
// under dirs — in catalogue order. It also includes Category IDs.
func AgentSkillIDs(dirs ...string) []string {
	all := AllAgentSkills(dirs...)
	ids := make([]string, 0, len(all)+len(skills.Categories))
	for _, s := range all {
		ids = append(ids, string(s.ID))
	}
	for _, c := range skills.Categories {
		ids = append(ids, string(c.ID))
	}
	return ids
}
