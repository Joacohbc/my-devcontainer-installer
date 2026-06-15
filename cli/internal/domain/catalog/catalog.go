package catalog

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// DockerfileModules is the ordered catalogue of all Dockerfile layer modules.
var DockerfileModules = []*dockerfile.ModuleSpec{
	dockerfile.BaseModule,
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
	dockerfile.ZellijModule,
	dockerfile.ChromeModule,
	dockerfile.DodModule,
	dockerfile.CleanupModule,
}

// ComposeServices is the ordered catalogue of all compose services.
var ComposeServices = []*compose.ServiceSpec{
	compose.DevcontainerService,
	compose.MongoService,
	compose.RedisService,
	compose.PostgresService,
	compose.TunnelService,
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
		if m.Always || m.UICategory == "" {
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
