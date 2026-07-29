package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	// Aliases are the user's own shell aliases, name → command. They are the
	// single source of truth for ~/.alias.sh in every container: rendered by
	// RenderUserAliases and pushed into the shared-config volume with
	// `config alias sync`. Managed with `config alias set/unset`, never by hand.
	Aliases map[string]string `json:"aliases,omitempty"`
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

// aliasNamePattern is the accepted shape of an alias name: a POSIX-ish
// identifier optionally with '-' and '.', so it is safe to write as
// `alias <name>=...` without quoting the left-hand side.
var aliasNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// IsValidAliasName reports whether name is a legal alias name.
func IsValidAliasName(name string) bool {
	return aliasNamePattern.MatchString(name)
}

// RenderUserAliases renders the user's aliases into a POSIX shell script, the
// body of ~/.alias.sh inside every container. Names are emitted in sorted order
// so the output is deterministic (stable diffs, stable volume writes), and each
// value is single-quoted with embedded quotes escaped the POSIX way ('\”) so an
// arbitrary command survives verbatim. Returns just the managed header when
// there are no aliases, so the file is always valid to source.
func RenderUserAliases(aliases map[string]string) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# devcontainer-cli — your own shell aliases.\n")
	b.WriteString("# GENERATED from the CLI config; edit with `devcontainer-cli config alias set/unset`\n")
	b.WriteString("# and apply with `devcontainer-cli config alias sync`. Manual edits are overwritten.\n")

	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString("alias " + name + "='" + strings.ReplaceAll(aliases[name], "'", `'\''`) + "'\n")
	}
	return b.String()
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
