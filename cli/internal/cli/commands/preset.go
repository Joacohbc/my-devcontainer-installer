package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newPresetCommand()) }

func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage presets",
	}
	cmd.AddCommand(newPresetListCommand())
	cmd.AddCommand(newPresetCreateCommand())
	cmd.AddCommand(newPresetCopyCommand())
	return cmd
}

func newPresetListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List builtin and user-defined presets",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, p := range (service.ConfigService{Report: console}).Presets() {
				console.Printf("%-20s %-10s %s\n", p.ID, p.Source, p.Label)
				if len(p.Modules) > 0 {
					console.Printf("  modules:  %s\n", strings.Join(p.Modules, ", "))
				}
				if len(p.Services) > 0 {
					console.Printf("  services: %s\n", strings.Join(p.Services, ", "))
				}
			}
			return nil
		},
	}
}

func newPresetCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create <preset-id>",
		Short:        "Create a new custom preset using the interactive wizard",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runPresetCreate,
	}
	addInteractiveFlag(cmd)
	return cmd
}

func runPresetCreate(cmd *cobra.Command, args []string) error {
	presetID := args[0]
	// Validate preset ID format (alphanumeric, dashes, underscores)
	for _, r := range presetID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid preset ID: %s. Use only alphanumeric characters, dashes, and underscores", presetID)
		}
	}

	if !interactiveFlag(cmd) {
		return fmt.Errorf("preset create is an interactive wizard; cannot run with --no-interactive")
	}

	presetsDir := filepath.Join(domain.GlobalConfigDir(), "presets")
	if _, ok := catalog.Resolve(presetID, presetsDir); ok {
		return fmt.Errorf("preset %q already exists", presetID)
	}

	label, err := console.Ask("Enter a label/description for the preset:")
	if err != nil {
		return err
	}
	if label == "" {
		label = fmt.Sprintf("Custom Preset %s", presetID)
	}

	svc := service.GenerateService{Report: console}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	baseConfig := domain.DefaultConfig(cwd)

	config, err := svc.Configure(baseConfig, cwd, console)
	if err != nil {
		return err
	}

	var modules []string
	for _, m := range config.Dockerfile.Modules {
		modules = append(modules, string(m.ID))
	}

	var services []string
	for _, s := range types.NormalizeServices(config.Compose.Services) {
		if s.ID != "devcontainer" {
			services = append(services, string(s.ID))
		}
	}

	preset := catalog.Preset{
		ID:       presetID,
		Label:    label,
		Modules:  modules,
		Services: services,
		Mode:     config.Mode,
	}

	if err := os.MkdirAll(presetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create presets directory: %w", err)
	}

	presetPath := filepath.Join(presetsDir, presetID+".yml")
	data, err := yaml.Marshal(preset)
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}

	if err := os.WriteFile(presetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save preset: %w", err)
	}

	console.Success("Preset %q successfully created at %s", presetID, presetPath)
	return nil
}

func newPresetCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "copy <existing-preset-id> <new-preset-id>",
		Short:        "Copy an existing preset (builtin or user) to a new user preset",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE:         runPresetCopy,
	}
	addInteractiveFlag(cmd)
	return cmd
}

func runPresetCopy(cmd *cobra.Command, args []string) error {
	existingID := args[0]
	newID := args[1]

	// Validate new preset ID format
	for _, r := range newID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid new preset ID: %s. Use only alphanumeric characters, dashes, and underscores", newID)
		}
	}

	presetsDir := filepath.Join(domain.GlobalConfigDir(), "presets")
	p, ok := catalog.Resolve(existingID, presetsDir)
	if !ok {
		return fmt.Errorf("preset %q not found", existingID)
	}

	if _, ok := catalog.Resolve(newID, presetsDir); ok {
		return fmt.Errorf("preset %q already exists", newID)
	}

	var label string
	var err error
	if interactiveFlag(cmd) {
		label, err = console.Ask(fmt.Sprintf("Enter a label/description for the new preset (default: %q):", p.Label))
		if err != nil {
			label = p.Label
		}
	} else {
		label = p.Label
	}
	if label == "" {
		label = p.Label
	}

	preset := catalog.Preset{
		ID:       newID,
		Label:    label,
		Modules:  p.Modules,
		Services: p.Services,
		Mode:     p.Mode,
	}

	if err := os.MkdirAll(presetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create presets directory: %w", err)
	}

	presetPath := filepath.Join(presetsDir, newID+".yml")
	data, err := yaml.Marshal(preset)
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}

	if err := os.WriteFile(presetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save preset: %w", err)
	}

	console.Success("Preset %q successfully copied to %q at %s", existingID, newID, presetPath)
	return nil
}
