package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// The preset group is attached under `config` (see newConfigCommand); it is not
// registered as a top-level command.
func newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage presets (list/create/copy)",
	}
	cmd.AddCommand(newPresetListCommand())
	cmd.AddCommand(newPresetCreateCommand())
	cmd.AddCommand(newPresetCopyCommand())
	return cmd
}

func newPresetListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List builtin and user-defined presets (module bundles)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			presets := (service.ConfigService{Report: console}).Presets()

			var builtin, user []catalog.Preset
			width := 0
			for _, p := range presets {
				if len(p.ID) > width {
					width = len(p.ID)
				}
				if p.Source == "user" {
					user = append(user, p)
				} else {
					builtin = append(builtin, p)
				}
			}

			printGroup := func(title string, group []catalog.Preset, emptyHint string) {
				console.Print(ui.HeaderS(title) + "\n")
				if len(group) == 0 {
					console.Print("  " + ui.Subtle(emptyHint) + "\n")
					return
				}
				for _, p := range group {
					// Pad the plain id first, then style it, so ANSI codes don't
					// throw off column alignment.
					id := ui.Bold(fmt.Sprintf("%-*s", width, p.ID))
					console.Print(fmt.Sprintf("  %s  %s\n", id, ui.Subtle(strings.Join(p.Modules, ", "))))
				}
			}

			printGroup("Built-in presets", builtin, "(none)")
			console.Print("\n")
			printGroup("Your presets", user, "(none — create one with 'config preset create')")
			return nil
		},
	}
}

func newPresetCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a new custom preset (module bundle) using the interactive wizard",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runPresetCreate,
	}
	addInteractiveFlag(cmd)
	return cmd
}

func runPresetCreate(cmd *cobra.Command, _ []string) error {
	if !interactiveFlag(cmd) {
		return fmt.Errorf("preset create is an interactive wizard; cannot run with --no-interactive")
	}

	presetsDir := filepath.Join(domain.GlobalConfigDir(), "presets")

	presetID, err := console.Ask("Preset ID (alphanumeric, dashes, underscores):")
	if err != nil {
		return err
	}
	presetID = strings.TrimSpace(presetID)
	if err := validatePresetID(presetID); err != nil {
		return err
	}
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

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	// A preset is a pure module bundle: the wizard only offers module selection
	// (no services, ports, volumes or build mode).
	modules, err := (service.GenerateService{Report: console}).SelectModules(domain.DefaultConfig(cwd), console)
	if err != nil {
		return err
	}

	if err := savePreset(presetsDir, catalog.Preset{ID: presetID, Label: label, Modules: modules}); err != nil {
		return err
	}

	console.Success("Preset %q successfully created at %s", presetID, filepath.Join(presetsDir, presetID+".yml"))
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

	if err := validatePresetID(newID); err != nil {
		return err
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

	if err := savePreset(presetsDir, catalog.Preset{ID: newID, Label: label, Modules: p.Modules}); err != nil {
		return err
	}

	console.Success("Preset %q successfully copied to %q at %s", existingID, newID, filepath.Join(presetsDir, newID+".yml"))
	return nil
}

// validatePresetID rejects empty ids and ids with characters outside the
// [a-zA-Z0-9_-] set used for the on-disk preset filename.
func validatePresetID(id string) error {
	if id == "" {
		return fmt.Errorf("preset ID cannot be empty")
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid preset ID: %s. Use only alphanumeric characters, dashes, and underscores", id)
		}
	}
	return nil
}

// savePreset marshals p to YAML and writes it to <dir>/<id>.yml, creating dir.
func savePreset(dir string, p catalog.Preset) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create presets directory: %w", err)
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, p.ID+".yml"), data, 0644); err != nil {
		return fmt.Errorf("failed to save preset: %w", err)
	}
	return nil
}
