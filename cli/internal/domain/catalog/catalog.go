package catalog

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/dockerfile"
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
	dockerfile.DbclientsModule,
	dockerfile.NodejsModule,
	dockerfile.PnpmModule,
	dockerfile.BunModule,
	dockerfile.ClaudeCodeModule,
	dockerfile.OpencodeModule,
	dockerfile.CodexCliModule,
	dockerfile.AntigravityCliModule,
	dockerfile.CopilotCliModule,
	dockerfile.TmuxModule,
	dockerfile.CleanupModule,
}

// ComposeServices is the ordered catalogue of all compose services.
var ComposeServices = []*compose.ServiceSpec{
	compose.DevcontainerService,
	compose.MongoService,
	compose.RedisService,
	compose.PostgresService,
	compose.MysqlService,
	compose.TunnelService,
}

// GetDockerfileModule returns the module with the given ID, or nil.
func GetDockerfileModule(id string) *dockerfile.ModuleSpec {
	for _, m := range DockerfileModules {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// GetComposeService returns the service with the given ID, or nil.
func GetComposeService(id string) *compose.ServiceSpec {
	for _, s := range ComposeServices {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// AllModuleIDs returns the IDs of all registered Dockerfile modules.
func AllModuleIDs() []string {
	ids := make([]string, len(DockerfileModules))
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
