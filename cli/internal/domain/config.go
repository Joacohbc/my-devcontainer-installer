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

	return &cfg, nil
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
