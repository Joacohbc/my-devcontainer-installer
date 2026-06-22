package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
		Short: "Manage reusable module-bundle presets",
		Long: `devcontainer-cli config preset — manage presets, named bundles of Dockerfile
modules you can reuse when generating projects.

A preset is just a list of module ids (no services, ports, volumes or build mode).
Built-in presets ship with the CLI; your own are saved under
~/.devcontainer-cli/presets/. Pass a preset to 'devcontainer-cli --preset <id>'
(or pick one in the interactive wizard) to pre-select its modules.

Subcommands:
  list                List built-in and user presets with their modules.
  create              Create a user preset via an interactive module picker.
  copy <from> <to>    Copy any preset into a new user preset.
  remove <id...>      Delete user presets (built-ins cannot be removed).`,
		Example: `  devcontainer-cli config preset list
  devcontainer-cli config preset create
  devcontainer-cli config preset copy web my-web
  devcontainer-cli --preset my-web`,
	}
	cmd.AddCommand(newPresetListCommand())
	cmd.AddCommand(newPresetCreateCommand())
	cmd.AddCommand(newPresetCopyCommand())
	cmd.AddCommand(newPresetRemoveCommand())
	return cmd
}

func newPresetListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List built-in and user presets with their modules",
		Long:         "devcontainer-cli config preset list — show every available preset, grouped\ninto built-in and your own, with the module ids each one bundles.",
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
		Use:   "create",
		Short: "Create a user preset via an interactive module picker",
		Long: `devcontainer-cli config preset create — interactively create a new user preset.

It prompts for a preset id and label, then opens the module picker so you can
choose which Dockerfile modules the preset bundles, and saves it under
~/.devcontainer-cli/presets/. This is an interactive command: it cannot run with
--no-interactive.`,
		Example:      "  devcontainer-cli config preset create",
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
		Use:   "copy <existing-preset-id> <new-preset-id>",
		Short: "Copy any preset into a new user preset",
		Long: `devcontainer-cli config preset copy — duplicate an existing preset (built-in or
user) into a new user preset you can then edit.

The new preset is saved under ~/.devcontainer-cli/presets/ with the same modules.
In interactive mode you're asked for a label; with --no-interactive the source
label is kept.`,
		Example: `  # Fork the built-in 'web' preset
  devcontainer-cli config preset copy web my-web`,
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

func newPresetRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <preset-id...>",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete user presets (built-ins cannot be removed)",
		Long: `devcontainer-cli config preset remove — delete one or more of your own presets
from ~/.devcontainer-cli/presets/.

Only user presets can be removed; attempting to remove a built-in preset errors
out. Every id is validated before anything is deleted, so a bad id aborts the
whole operation. Preset ids tab-complete.`,
		Example: `  devcontainer-cli config preset remove my-web
  devcontainer-cli config preset rm old-preset another --yes`,
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              runPresetRemove,
		ValidArgsFunction: completeUserPresetArgs,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runPresetRemove(cmd *cobra.Command, args []string) error {
	presetsDir := filepath.Join(domain.GlobalConfigDir(), "presets")

	// Resolve every target up front so we fail before deleting anything.
	paths := make([]string, 0, len(args))
	for _, id := range args {
		path, ok := userPresetPath(presetsDir, id)
		if !ok {
			if _, isPreset := catalog.Resolve(id, presetsDir); isPreset {
				return fmt.Errorf("preset %q is a built-in preset and cannot be removed", id)
			}
			return fmt.Errorf("user preset %q not found", id)
		}
		paths = append(paths, path)
	}

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("refusing to remove preset(s) without confirmation; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.Confirm(fmt.Sprintf("Remove %d user preset(s): %s?", len(args), strings.Join(args, ", ")))
		if err != nil {
			return err
		}
		if !proceed {
			console.Info("Cancelled.")
			return nil
		}
	}

	for i, path := range paths {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove preset %q: %w", args[i], err)
		}
		console.Success("Removed preset %q", args[i])
	}
	return nil
}

// userPresetPath returns the on-disk path of a user preset id (.yml or .yaml).
func userPresetPath(dir, id string) (string, bool) {
	for _, ext := range []string{".yml", ".yaml"} {
		p := filepath.Join(dir, id+ext)
		if fileExists(p) {
			return p, true
		}
	}
	return "", false
}

func completeUserPresetArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, p := range catalog.LoadUserPresets(filepath.Join(domain.GlobalConfigDir(), "presets")) {
		if strings.HasPrefix(p.ID, toComplete) && !slices.Contains(args, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
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
