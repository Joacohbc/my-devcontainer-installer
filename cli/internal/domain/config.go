package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func ConfigPath(cwd string) string {
	return filepath.Join(cwd, types.ConfigFile)
}

func LoadConfig(cwd string) (*types.DevcontainerConfig, error) {
	p := ConfigPath(cwd)
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg types.DevcontainerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", types.ConfigFile, err)
	}

	if cfg.Workspace == "" {
		cfg.Workspace = SanitizeDockerName(filepath.Base(cwd), "devcontainer")
	}

	isLegacyMode := cfg.Mode == "" || cfg.Mode == "custom" || cfg.Mode == "standalone"
	if isLegacyMode {
		cfg.Mode = types.BuildModeLocalCached
	}

	cfg.Dockerfile.Modules = migrateDbclients(cfg.Dockerfile.Modules)

	return &cfg, nil
}

// dbclientLegacyIDs maps the legacy "dbclients" module's per-client option
// values to the individual client modules that replaced it.
var dbclientLegacyIDs = map[string]types.ModuleID{
	"postgres": types.ModulePostgresClient,
	"redis":    types.ModuleRedisClient,
	"mongo":    types.ModuleMongoClient,
}

// migrateDbclients expands the legacy combined "dbclients" module (a single
// module carrying a "clients" multiselect) into the individual per-client
// modules so configs written before the split keep working.
func migrateDbclients(modules []types.SelectedModule) []types.SelectedModule {
	out := make([]types.SelectedModule, 0, len(modules))
	for _, m := range modules {
		if m.ID != "dbclients" {
			out = append(out, m)
			continue
		}
		clients := types.CoerceStrings(m.Options["clients"])
		if len(clients) == 0 {
			clients = []string{"postgres", "redis", "mongo"}
		}
		for _, c := range clients {
			if id, ok := dbclientLegacyIDs[c]; ok {
				out = append(out, types.SelectedModule{ID: id, Options: map[string]any{}})
			}
		}
	}
	return out
}

func SaveConfig(config *types.DevcontainerConfig, cwd string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(ConfigPath(cwd), data, 0644)
}

func DefaultConfig(cwd string) *types.DevcontainerConfig {
	workspace := SanitizeDockerName(filepath.Base(cwd), "devcontainer")
	return &types.DevcontainerConfig{
		Mode:      types.BuildModeLocalCached,
		Image:     workspace + ":local",
		Workspace: workspace,
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{},
		},
		Compose: types.ComposeConfig{
			Services: []any{},
			Subnet:   "172.25.0.0/28",
		},
		Env: map[string]string{},
	}
}
