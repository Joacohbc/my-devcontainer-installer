package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

type Defaults struct {
	DBUser        string `json:"dbUser,omitempty"`
	DBPassword    string `json:"dbPassword,omitempty"`
	SSHKeyPath    string `json:"sshKeyPath,omitempty"`
	SSHConfigFile string `json:"sshConfigFile,omitempty"`
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

// ManagedKnownHostsPath is the path of the dedicated known_hosts file the CLI
// manages for devcontainers (e.g. <config>/ssh/known_hosts). Unlike the key it
// is never overridable: it is an internal bookkeeping file, not user key
// material, and generated Host blocks point at it so a rebuilt container's new
// host key can be re-pinned without ever touching ~/.ssh/known_hosts.
func ManagedKnownHostsPath() string {
	return filepath.Join(GlobalConfigDir(), "ssh", types.SSHKnownHostsName)
}

// DefaultManagedSSHConfigPath is the path of the SSH config file the CLI owns
// (~/.ssh/devcontainer-cli.config). Every managed Host block lives there instead
// of in the user's ~/.ssh/config, which only ever gains a single Include line.
func DefaultManagedSSHConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", types.SSHConfigName)
}

// UserSSHConfigPath is the path of the user's own ~/.ssh/config. The CLI reads
// it (to detect alias collisions and host references) but only ever writes the
// Include directive pointing at the managed file.
func UserSSHConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// ResolveSSHConfigPath returns the managed SSH config path to use, with
// precedence flagOverride → cfg.Defaults.SSHConfigFile → the default.
func ResolveSSHConfigPath(flagOverride string) string {
	if flagOverride != "" {
		return flagOverride
	}
	cfg := LoadGlobalConfig()
	if cfg.Defaults != nil && cfg.Defaults.SSHConfigFile != "" {
		return cfg.Defaults.SSHConfigFile
	}
	return DefaultManagedSSHConfigPath()
}

// UserAliasFilePath is the host path of the user's own shell alias file.
//
// It deliberately lives in the home directory rather than under
// GlobalConfigDir(): it is the host side of the "alias.sh" shared-config entry
// (types.SharedConfigEntries), whose host source is always
// $HOME/<entry.Target>. Keeping the two in sync is what lets
// `config shared sync alias.sh` push this exact file into the shared volume
// with no special-casing.
func UserAliasFilePath() string {
	home, _ := os.UserHomeDir()
	entry, ok := types.SharedConfigEntryByID(types.SharedConfigAliasID)
	if !ok {
		return filepath.Join(home, ".alias.sh")
	}
	return filepath.Join(home, entry.Target)
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
