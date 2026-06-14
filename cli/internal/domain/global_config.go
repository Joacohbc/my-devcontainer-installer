package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

type Defaults struct {
	DBUser     string `json:"dbUser,omitempty"`
	DBPassword string `json:"dbPassword,omitempty"`
	SSHKeyPath string `json:"sshKeyPath,omitempty"`
}

type GlobalConfig struct {
	Registry string    `json:"registry,omitempty"`
	Defaults *Defaults `json:"defaults,omitempty"`
}

const DefaultRegistry = "ghcr.io/joacohbc/"

func GlobalConfigDir() string {
	// Read XDG_CONFIG_HOME at call time so tests can override it with os.Setenv.
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "devcontainer-cli")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devcontainer-cli")
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

const (
	fallbackDBUser     = "devuser"
	fallbackDBPassword = "devpass"
)

func ResolveDBCredentials() (user, password string) {
	cfg := LoadGlobalConfig()
	if cfg.Defaults == nil {
		return fallbackDBUser, fallbackDBPassword
	}
	user = cfg.Defaults.DBUser
	if user == "" {
		user = fallbackDBUser
	}
	password = cfg.Defaults.DBPassword
	if password == "" {
		password = fallbackDBPassword
	}
	return user, password
}

// DefaultManagedSSHKeyPath is the path of the single shared SSH key managed by
// the CLI, stored under the global config dir (e.g. <config>/ssh/id_devcontainer).
func DefaultManagedSSHKeyPath() string {
	return filepath.Join(GlobalConfigDir(), "ssh", types.SSHKeyName)
}

// ResolveSSHKeyPath returns the SSH key path to use, with precedence
// flagOverride → cfg.Defaults.SSHKeyPath → DefaultManagedSSHKeyPath().
func ResolveSSHKeyPath(flagOverride string) string {
	if flagOverride != "" {
		return flagOverride
	}
	cfg := LoadGlobalConfig()
	if cfg.Defaults != nil && cfg.Defaults.SSHKeyPath != "" {
		return cfg.Defaults.SSHKeyPath
	}
	return DefaultManagedSSHKeyPath()
}
