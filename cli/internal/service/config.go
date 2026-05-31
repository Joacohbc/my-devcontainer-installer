package service

import (
	"fmt"
	"path/filepath"
	"strconv"

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
	fallbackDBUser     = "devuser"
	fallbackDBPassword = "devpass"
	fallbackSSHPort    = "2222"
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
	case "ssh-port":
		if port := globalDefaults(cfg).SSHHostPort; port != 0 {
			return strconv.Itoa(port), true, nil
		}
		return fallbackSSHPort, false, nil
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
	case "ssh-port":
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port: %s (must be between 1 and 65535)", value)
		}
		ensureDefaults(&cfg)
		cfg.Defaults.SSHHostPort = port
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
	case "ssh-port":
		ensureDefaults(&cfg)
		cfg.Defaults.SSHHostPort = 0
		fallback = fallbackSSHPort
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

// Presets returns the builtin and user-defined presets.
func (s ConfigService) Presets() []catalog.Preset {
	return catalog.All(filepath.Join(domain.GlobalConfigDir(), "presets"))
}
