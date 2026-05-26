package registry

import (
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/modules/compose"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/modules/dockerfile"
)

// DockerfileModules is the ordered catalogue of all Dockerfile layer modules.
var DockerfileModules = []*dockerfile.DockerfileModuleSpec{
	dockerfile.BaseModule,
	dockerfile.GithubCliModule,
	dockerfile.DodModule,
	dockerfile.JavaTemurinModule,
	dockerfile.JavaOpenjdkModule,
	dockerfile.PythonModule,
	dockerfile.SqliteModule,
	dockerfile.GolangModule,
	dockerfile.DbclientsModule,
	dockerfile.NodejsModule,
	dockerfile.PnpmModule,
	dockerfile.BunModule,
	dockerfile.AiClisModule,
	dockerfile.TmuxModule,
	dockerfile.CleanupModule,
}

// ComposeServices is the ordered catalogue of all compose services.
var ComposeServices = []*compose.ServiceSpec{
	compose.DevcontainerService,
	compose.DindEngineService,
	compose.MongoService,
	compose.RedisService,
	compose.PostgresService,
	compose.TunnelService,
}

// GetDockerfileModule returns the module with the given ID, or nil.
func GetDockerfileModule(id string) *dockerfile.DockerfileModuleSpec {
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

// GetDockerfileModuleCore looks up a module in the global catalogue for use by
// code that only has access to core types (e.g., domain/resolver).
func GetDockerfileModuleCore(id string) *dockerfile.DockerfileModuleSpec {
	return GetDockerfileModule(id)
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
func AlwaysOnModules() []*dockerfile.DockerfileModuleSpec {
	var out []*dockerfile.DockerfileModuleSpec
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
