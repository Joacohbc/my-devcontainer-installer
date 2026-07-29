package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

// aliasFileTemplate seeds a fresh user alias file. It is intentionally all
// comments: an empty-but-documented file makes the two-layer model obvious and
// cannot change any container's behaviour on its own.
const aliasFileTemplate = `#!/bin/sh
# Your own devcontainer shell aliases and functions.
#
# This file is synced into the shared-config Docker volume and symlinked into
# every container as ~/.alias.sh, so it applies everywhere WITHOUT rebuilding
# any image. It is sourced after the CLI's baked defaults
# (~/.devcontainer_aliases.sh), so anything defined here overrides them.
#
# After editing, push it to the volume with:
#     devcontainer-cli config shared sync alias.sh --force
#
# Guard anything tool-specific so this file stays valid in every image variant:
#
#     if command -v kubectl >/dev/null 2>&1; then
#         alias k='kubectl'
#     fi
#
# Undo one of the defaults by unaliasing it:
#
#     unalias npm 2>/dev/null   # go back to the real npm
`

// AliasFilePath returns the host path of the user's own alias file.
func (s ConfigService) AliasFilePath() string {
	return domain.UserAliasFilePath()
}

// ReadAliasFile returns the user alias file's path and contents. exists is
// false (with no error) when the file has not been created yet.
func (s ConfigService) ReadAliasFile() (path, content string, exists bool, err error) {
	path = domain.UserAliasFilePath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, "", false, nil
	}
	if err != nil {
		return path, "", false, err
	}
	return path, string(data), true, nil
}

// EnsureAliasFile creates the user alias file from the template when missing and
// reports whether it had to create it. An existing file is never touched.
func (s ConfigService) EnsureAliasFile() (path string, created bool, err error) {
	path = domain.UserAliasFilePath()
	if _, statErr := os.Stat(path); statErr == nil {
		return path, false, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return path, false, statErr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(aliasFileTemplate), 0o644); err != nil {
		return path, false, err
	}
	s.Report.Success("✓ Created %s", path)
	return path, true, nil
}

// ResetAliasFile overwrites the user alias file with the template, discarding
// whatever was there. Destructive: the caller must have confirmed first.
func (s ConfigService) ResetAliasFile() (string, error) {
	path := domain.UserAliasFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, []byte(aliasFileTemplate), 0o644); err != nil {
		return path, err
	}
	s.Report.Success("✓ Reset %s to the default template", path)
	return path, nil
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
