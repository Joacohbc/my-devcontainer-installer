package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type GlobalConfig struct {
	Registry string `json:"registry,omitempty"`
}

const DefaultRegistry = "ghcr.io/joacohbc/"

func GlobalConfigDir() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, "devcontainer-cli")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", "devcontainer-cli")
	}
	// Read XDG_CONFIG_HOME at call time so tests can override it with os.Setenv.
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		home, _ := os.UserHomeDir()
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "devcontainer-cli")
}

func GlobalConfigPath() string {
	return filepath.Join(GlobalConfigDir(), "config.json")
}

func LoadGlobalConfig() GlobalConfig {
	data, err := os.ReadFile(GlobalConfigPath())
	if err != nil {
		return GlobalConfig{}
	}
	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return GlobalConfig{}
	}
	return cfg
}

func SaveGlobalConfig(cfg GlobalConfig) error {
	dir := GlobalConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(GlobalConfigPath(), data, 0644)
}

func normalizeRegistry(url string) string {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return trimmed
	}
	if !strings.HasSuffix(trimmed, "/") {
		return trimmed + "/"
	}
	return trimmed
}

func ResolveRegistry(flagOverride, perProject string) string {
	if flagOverride != "" {
		return normalizeRegistry(flagOverride)
	}
	if perProject != "" {
		return normalizeRegistry(perProject)
	}
	cfg := LoadGlobalConfig()
	if cfg.Registry != "" {
		return normalizeRegistry(cfg.Registry)
	}
	return DefaultRegistry
}
