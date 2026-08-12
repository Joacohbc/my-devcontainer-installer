package service

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

// ConfigService reads and writes global CLI config keys and project config
// import/export. It owns validation, persistence and success reporting; the cli
// parses args and prints the read values.
type ConfigService struct {
	Report Reporter
}

const (
	fallbackDBUser     = types.FallbackDBUser
	fallbackDBPassword = types.FallbackDBPassword
)

func defaulted(stored, fallback string) (value string, customized bool, err error) {
	if stored == "" {
		return fallback, false, nil
	}
	return stored, true, nil
}

func globalDefaults(cfg domain.GlobalConfig) domain.Defaults {
	if cfg.Defaults == nil {
		return domain.Defaults{}
	}
	return *cfg.Defaults
}

// GetGlobalDefault returns the effective value of a global config key and
// whether it was explicitly customized (vs the built-in fallback).
func (s ConfigService) GetGlobalDefault(key string) (value string, customized bool, err error) {
	cfg := domain.LoadGlobalConfig()
	switch key {
	case "registry":
		return defaulted(cfg.Registry, domain.DefaultRegistry)
	case "db-user":
		return defaulted(globalDefaults(cfg).DBUser, fallbackDBUser)
	case "db-password":
		return defaulted(globalDefaults(cfg).DBPassword, fallbackDBPassword)
	case "ssh-key":
		return defaulted(globalDefaults(cfg).SSHKeyPath, domain.DefaultManagedSSHKeyPath())
	case "ssh-config-file":
		return defaulted(globalDefaults(cfg).SSHConfigFile, domain.DefaultManagedSSHConfigPath())
	}
	return "", false, fmt.Errorf("unknown config key: %s", key)
}

// SetGlobalDefault validates and persists value for a global config key.
func (s ConfigService) SetGlobalDefault(key, value string) error {
	cfg := domain.LoadGlobalConfig()
	switch key {
	case "registry":
		cfg.Registry = value
	case "db-user":
		ensureDefaults(&cfg)
		cfg.Defaults.DBUser = value
	case "db-password":
		ensureDefaults(&cfg)
		cfg.Defaults.DBPassword = value
	case "ssh-key":
		ensureDefaults(&cfg)
		cfg.Defaults.SSHKeyPath = value
	case "ssh-config-file":
		ensureDefaults(&cfg)
		cfg.Defaults.SSHConfigFile = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		return err
	}
	s.Report.Success("✓ Set %s = %s (in %s)", key, value, domain.GlobalConfigPath())
	return nil
}

// UnsetGlobalDefault clears a global config key, reverting to its default.
func (s ConfigService) UnsetGlobalDefault(key string) error {
	cfg := domain.LoadGlobalConfig()
	var fallback string
	switch key {
	case "registry":
		cfg.Registry = ""
		fallback = domain.DefaultRegistry
	case "db-user":
		ensureDefaults(&cfg)
		cfg.Defaults.DBUser = ""
		fallback = fallbackDBUser
	case "db-password":
		ensureDefaults(&cfg)
		cfg.Defaults.DBPassword = ""
		fallback = fallbackDBPassword
	case "ssh-key":
		ensureDefaults(&cfg)
		cfg.Defaults.SSHKeyPath = ""
		fallback = domain.DefaultManagedSSHKeyPath()
	case "ssh-config-file":
		ensureDefaults(&cfg)
		cfg.Defaults.SSHConfigFile = ""
		fallback = domain.DefaultManagedSSHConfigPath()
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		return err
	}
	s.Report.Success("✓ Unset %s (will use default: %s).", key, fallback)
	return nil
}

func ensureDefaults(cfg *domain.GlobalConfig) {
	if cfg.Defaults == nil {
		cfg.Defaults = &domain.Defaults{}
	}
}

// Aliases returns the user's configured shell aliases (name → command) from the
// global CLI config. The map is never nil.
func (s ConfigService) Aliases() map[string]string {
	cfg := domain.LoadGlobalConfig()
	if cfg.Aliases == nil {
		return map[string]string{}
	}
	return cfg.Aliases
}

// SetAlias validates and persists a single alias in the global config. An
// existing name is overwritten. It does not touch any container — call
// SyncAliasesToVolume (or `config alias sync`) to apply it.
func (s ConfigService) SetAlias(name, command string) error {
	if !domain.IsValidAliasName(name) {
		return fmt.Errorf("invalid alias name %q: use letters, digits, '_', '-' or '.', starting with a letter or '_'", name)
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("alias %q needs a non-empty command", name)
	}
	cfg := domain.LoadGlobalConfig()
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]string{}
	}
	cfg.Aliases[name] = command
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		return err
	}
	s.Report.Success("✓ Set alias %s='%s' (in %s)", name, command, domain.GlobalConfigPath())
	return nil
}

// UnsetAlias removes an alias from the global config, reporting whether it was
// present. Removing an absent alias is a no-op, not an error.
func (s ConfigService) UnsetAlias(name string) (existed bool, err error) {
	cfg := domain.LoadGlobalConfig()
	if _, ok := cfg.Aliases[name]; !ok {
		return false, nil
	}
	delete(cfg.Aliases, name)
	if err := domain.SaveGlobalConfig(cfg); err != nil {
		return true, err
	}
	s.Report.Success("✓ Removed alias %s", name)
	return true, nil
}

// RenderedAliases returns the shell script that ~/.alias.sh is populated with,
// derived from the configured aliases.
func (s ConfigService) RenderedAliases() string {
	return domain.RenderUserAliases(s.Aliases())
}

// ExportConfigYAML renders the project's devcontainer config as YAML.
func (s ConfigService) ExportConfigYAML(cwd string) ([]byte, error) {
	cfg, err := domain.LoadConfig(cwd)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("no devcontainer.config.json found in %s", cwd)
	}
	return yaml.Marshal(cfg)
}

// ParseConfigYAML unmarshals and validates raw YAML into a config.
func (s ConfigService) ParseConfigYAML(data []byte) (*types.DevcontainerConfig, error) {
	var cfg types.DevcontainerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid yaml: %w", err)
	}
	if !domain.IsValidDockerName(cfg.Workspace) {
		return nil, fmt.Errorf("invalid workspace name: %s", cfg.Workspace)
	}
	if cfg.Image != "" && !domain.IsValidImageName(cfg.Image) {
		return nil, fmt.Errorf("invalid image name: %s", cfg.Image)
	}
	if cfg.Compose.Subnet != "" && !domain.IsValidCidr(cfg.Compose.Subnet) {
		return nil, fmt.Errorf("invalid CIDR: %s", cfg.Compose.Subnet)
	}
	return &cfg, nil
}

// SaveProjectConfig writes devcontainer.config.json under cwd.
func (s ConfigService) SaveProjectConfig(cwd string, cfg *types.DevcontainerConfig) error {
	return domain.SaveConfig(cfg, cwd)
}

// Profiles returns the builtin and user-defined profiles.
func (s ConfigService) Profiles() []catalog.Profile {
	return catalog.All(domain.ProfileDirs()...)
}
