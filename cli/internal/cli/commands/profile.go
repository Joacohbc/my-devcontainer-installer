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
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// The profile group is attached under `config` (see newConfigCommand); it is not
// registered as a top-level command.
func newProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "profile",
		Aliases: []string{"preset"},
		Short:   "Manage reusable profiles (module bundles + custom scripts)",
		Long: `devcontainer-cli config profile — manage profiles, named bundles of Dockerfile
modules plus your own scripts, that you can reuse when generating projects.

Built-in profiles ship with the CLI, some of them (like 'scraper') carrying
scripts of their own; your own are saved under ~/.devcontainer-cli/profiles/. A
profile with no scripts is a single <id>.yml; one that carries scripts is an
<id>/ directory holding profile.yml and the .sh files next to it. Copying a
built-in that ships scripts writes them out as a normal user profile you can
edit. Pass a profile to 'devcontainer-cli --profile <id>' (or pick
one in the interactive wizard) to pre-select its modules and add its scripts.

Each script declares when it runs: 'build' bakes it into the image, 'start' runs
it once per container, 'manual' only copies it to ~/post-script/.

A profile can also carry agent skills, installed project-scoped into the
workspace by the Skills CLI. Their mode is 'manual' (the default — you run
'install_skills' yourself) or 'auto' (installed on every container start);
either one pulls in the nodejs module.

Subcommands:
  list                List built-in and user profiles with their modules.
  create              Create a user profile via an interactive picker.
  copy <from> <to>    Copy any profile into a new user profile.
  remove <id...>      Delete user profiles (built-ins cannot be removed).

'preset' is accepted as a deprecated alias for this command.`,
		Example: `  devcontainer-cli config profile list
  devcontainer-cli config profile create
  devcontainer-cli config profile copy nodejs my-node
  devcontainer-cli --profile my-node`,
	}
	cmd.AddCommand(newProfileListCommand())
	cmd.AddCommand(newProfileCreateCommand())
	cmd.AddCommand(newProfileCopyCommand())
	cmd.AddCommand(newProfileRemoveCommand())
	return cmd
}

func newProfileListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List built-in and user profiles with their modules",
		Long:         "devcontainer-cli config profile list — show every available profile, grouped\ninto built-in and your own, with the module ids each one bundles and the custom\nscripts it carries.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles := (service.ConfigService{Report: console}).Profiles()

			var builtin, user []catalog.Profile
			width := 0
			for _, p := range profiles {
				if len(p.ID) > width {
					width = len(p.ID)
				}
				if p.Source == "user" {
					user = append(user, p)
				} else {
					builtin = append(builtin, p)
				}
			}

			printGroup := func(title string, group []catalog.Profile, emptyHint string) {
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
					if len(p.Scripts) > 0 {
						console.Print(fmt.Sprintf("  %s  %s\n", strings.Repeat(" ", width), ui.Subtle("scripts: "+describeScripts(p.Scripts))))
					}
					if len(p.Skills) > 0 {
						console.Print(fmt.Sprintf("  %s  %s\n", strings.Repeat(" ", width), ui.Subtle(describeSkills(p))))
					}
				}
			}

			printGroup("Built-in profiles", builtin, "(none)")
			console.Print("\n")
			printGroup("Your profiles", user, "(none — create one with 'config profile create')")
			return nil
		},
	}
}

// describeScripts renders a profile's scripts as "name (when), name (when)".
func describeScripts(scripts []types.CustomScript) string {
	out := make([]string, 0, len(scripts))
	for _, s := range scripts {
		out = append(out, fmt.Sprintf("%s (%s)", s.File, s.ResolvedWhen()))
	}
	return strings.Join(out, ", ")
}

// describeSkills renders a profile's agent skills as "skills: a, b (auto)".
func describeSkills(p catalog.Profile) string {
	ids := make([]string, 0, len(p.Skills))
	for _, id := range p.Skills {
		ids = append(ids, string(id))
	}
	mode := p.SkillsMode
	if mode == "" {
		mode = types.DefaultSkillMode
	}
	return fmt.Sprintf("skills: %s (%s)", strings.Join(ids, ", "), mode)
}

func newProfileCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user profile via an interactive picker",
		Long: `devcontainer-cli config profile create — interactively create a new user profile.

It prompts for a profile id and label, opens the module picker so you can choose
which Dockerfile modules the profile bundles, then lets you add your own scripts
(copied into the profile, so it stays self-contained) and pick when each one
runs. Saved under ~/.devcontainer-cli/profiles/. This is an interactive command:
it cannot run with --no-interactive.`,
		Example:      "  devcontainer-cli config profile create",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runProfileCreate,
	}
	addInteractiveFlag(cmd)
	return cmd
}

func runProfileCreate(cmd *cobra.Command, _ []string) error {
	if !interactiveFlag(cmd) {
		return fmt.Errorf("profile create is an interactive wizard; cannot run with --no-interactive")
	}

	profileID, err := console.Ask("Profile ID (alphanumeric, dashes, underscores):")
	if err != nil {
		return err
	}
	profileID = strings.TrimSpace(profileID)
	if err := domain.ValidateProfileID(profileID); err != nil {
		return err
	}
	if _, ok := catalog.Resolve(profileID, domain.ProfileDirs()...); ok {
		return fmt.Errorf("profile %q already exists", profileID)
	}

	label, err := console.Ask("Enter a label/description for the profile:")
	if err != nil {
		return err
	}
	if label == "" {
		label = fmt.Sprintf("Custom Profile %s", profileID)
	}

	cwd, err := currentDir()
	if err != nil {
		return err
	}

	// A profile's modules are a pure bundle: the wizard only offers module
	// selection (no services, ports, volumes or build mode).
	modules, err := (service.GenerateService{Report: console}).SelectModules(domain.DefaultConfig(cwd), console)
	if err != nil {
		return err
	}

	scripts, err := askCustomScripts()
	if err != nil {
		return err
	}

	skills, err := (service.GenerateService{Report: console}).SelectSkills(types.SkillsConfig{}, console)
	if err != nil {
		return err
	}

	profile := catalog.Profile{
		ID:         profileID,
		Label:      label,
		Modules:    modules,
		Scripts:    scripts,
		Skills:     skills.Skills,
		SkillsMode: skills.Mode,
	}
	path, err := saveProfile(domain.ProfileDir(), profile)
	if err != nil {
		return err
	}

	console.Success("Profile %q successfully created at %s", profileID, path)
	return nil
}

// askCustomScripts collects the user's own scripts for a new profile: a host
// path and, for each, when it should run. The files themselves are copied in by
// saveProfile, so the profile stays self-contained.
func askCustomScripts() ([]types.CustomScript, error) {
	add, err := console.Confirm("Add custom scripts to this profile?")
	if err != nil {
		return nil, err
	}
	if !add {
		return nil, nil
	}

	whenChoices := make([]service.Option, 0, len(types.ScriptWhens))
	for _, w := range types.ScriptWhens {
		whenChoices = append(whenChoices, service.Option{Value: string(w), Label: scriptWhenLabel(w)})
	}

	var scripts []types.CustomScript
	for {
		path, err := console.Ask("Path to the script (.sh) — empty to finish:")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) == "" {
			return scripts, nil
		}
		script, err := domain.ParseScriptSpec(path)
		if err != nil {
			console.Warn("%v", err)
			continue
		}
		if _, serr := os.Stat(script.Source); serr != nil {
			console.Warn("cannot read %s: %v", script.Source, serr)
			continue
		}
		// The path may already carry a `when` suffix (the --script spelling); if it
		// does, that is what the picker starts on instead of the default.
		when, err := console.Select(fmt.Sprintf("When should %s run?", script.File), whenChoices, whenOption(whenChoices, script.ResolvedWhen()))
		if err != nil {
			return nil, err
		}
		script.When = types.ScriptWhen(when.Value)

		// Two different paths can share a base name (~/a/setup.sh and
		// ~/b/setup.sh). They would land on one file inside the profile, so the
		// second silently replaced the first and the manifest listed it twice.
		// Make the overwrite a decision instead.
		if i := indexOfScript(scripts, script); i >= 0 {
			replace, err := console.Confirm(fmt.Sprintf("%s is already in this profile (%s). Replace it?", script.File, scripts[i].Source))
			if err != nil {
				return nil, err
			}
			if !replace {
				console.Info("Skipped %s. Rename the file if you need both.", script.Source)
				continue
			}
			scripts[i] = script
			continue
		}
		scripts = append(scripts, script)
	}
}

// indexOfScript finds an already-collected script that would occupy the same
// file name inside the profile, or -1.
func indexOfScript(scripts []types.CustomScript, s types.CustomScript) int {
	for i, cur := range scripts {
		if cur.File == s.File {
			return i
		}
	}
	return -1
}

// whenOption is the choice matching w, so the picker opens on it.
func whenOption(choices []service.Option, w types.ScriptWhen) service.Option {
	for _, c := range choices {
		if c.Value == string(w) {
			return c
		}
	}
	return choices[0]
}

func scriptWhenLabel(w types.ScriptWhen) string {
	switch w {
	case types.ScriptWhenStart:
		return "start — run once per container on start"
	case types.ScriptWhenManual:
		return "manual — only copy it to ~/post-script/"
	default:
		return "build — bake it into the image"
	}
}

func newProfileCopyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy <existing-profile-id> <new-profile-id>",
		Short: "Copy any profile into a new user profile",
		Long: `devcontainer-cli config profile copy — duplicate an existing profile (built-in or
user) into a new user profile you can then edit.

The new profile is saved under ~/.devcontainer-cli/profiles/ with the same
modules and a copy of the source profile's scripts. In interactive mode you're
asked for a label; with --no-interactive the source label is kept.`,
		Example: `  # Fork the built-in 'nodejs' profile
  devcontainer-cli config profile copy nodejs my-node`,
		Args:              cobra.ExactArgs(2),
		SilenceUsage:      true,
		RunE:              runProfileCopy,
		ValidArgsFunction: completeProfileCopyArgs,
	}
	addInteractiveFlag(cmd)
	return cmd
}

func runProfileCopy(cmd *cobra.Command, args []string) error {
	existingID := args[0]
	newID := args[1]

	if err := domain.ValidateProfileID(newID); err != nil {
		return err
	}

	p, ok := catalog.Resolve(existingID, domain.ProfileDirs()...)
	if !ok {
		return fmt.Errorf("profile %q not found", existingID)
	}

	if _, ok := catalog.Resolve(newID, domain.ProfileDirs()...); ok {
		return fmt.Errorf("profile %q already exists", newID)
	}

	var label string
	var err error
	if interactiveFlag(cmd) {
		label, err = console.Ask(fmt.Sprintf("Enter a label/description for the new profile (default: %q):", p.Label))
		if err != nil {
			label = p.Label
		}
	} else {
		label = p.Label
	}
	if label == "" {
		label = p.Label
	}

	// Resolve the source scripts so saveProfile copies the files themselves, not
	// just the manifest entries — the copy has to stand on its own.
	scripts, err := domain.ProfileScripts(p)
	if err != nil {
		return err
	}

	path, err := saveProfile(domain.ProfileDir(), catalog.Profile{
		ID:         newID,
		Label:      label,
		Modules:    p.Modules,
		Scripts:    scripts,
		Skills:     p.Skills,
		SkillsMode: p.SkillsMode,
	})
	if err != nil {
		return err
	}

	console.Success("Profile %q successfully copied to %q at %s", existingID, newID, path)
	return nil
}

func newProfileRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <profile-id...>",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete user profiles (built-ins cannot be removed)",
		Long: `devcontainer-cli config profile remove — delete one or more of your own profiles
from ~/.devcontainer-cli/profiles/.

Only user profiles can be removed; attempting to remove a built-in profile
errors out. Every id is validated before anything is deleted, so a bad id aborts
the whole operation. A profile that carries scripts is a directory, and removing
it removes those scripts with it. Profile ids tab-complete.`,
		Example: `  devcontainer-cli config profile remove my-node
  devcontainer-cli config profile rm old-profile another --yes`,
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              runProfileRemove,
		ValidArgsFunction: completeUserProfileArgs,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runProfileRemove(cmd *cobra.Command, args []string) error {
	// Resolve every target up front so we fail before deleting anything.
	paths := make([]string, 0, len(args))
	for _, id := range args {
		path, ok := userProfilePath(id)
		if !ok {
			if _, isProfile := catalog.Resolve(id, domain.ProfileDirs()...); isProfile {
				return fmt.Errorf("profile %q is a built-in profile and cannot be removed", id)
			}
			return fmt.Errorf("user profile %q not found", id)
		}
		paths = append(paths, path)
	}

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("refusing to remove profile(s) without confirmation; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.Confirm(fmt.Sprintf("Remove %d user profile(s): %s?", len(args), strings.Join(args, ", ")))
		if err != nil {
			return err
		}
		if !proceed {
			console.Info("Cancelled.")
			return nil
		}
	}

	for i, path := range paths {
		// A directory-shaped profile owns its scripts, so it goes as a whole.
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove profile %q: %w", args[i], err)
		}
		console.Success("Removed profile %q", args[i])
	}
	return nil
}

// userProfilePath returns the on-disk path of a user profile id, in either
// shape and in either the current or the legacy directory.
func userProfilePath(id string) (string, bool) {
	for _, dir := range domain.ProfileDirs() {
		for _, ext := range []string{".yml", ".yaml"} {
			if p := filepath.Join(dir, id+ext); fileExists(p) {
				return p, true
			}
		}
		p := filepath.Join(dir, id)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, true
		}
	}
	return "", false
}

func completeUserProfileArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, p := range catalog.LoadUserProfiles(domain.ProfileDirs()...) {
		if strings.HasPrefix(p.ID, toComplete) && !slices.Contains(args, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeProfileCopyArgs completes the source id of `profile copy` with every
// profile; the destination is a new name, so it is left uncompleted.
func completeProfileCopyArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, p := range catalog.All(domain.ProfileDirs()...) {
		if strings.HasPrefix(p.ID, toComplete) {
			out = append(out, p.ID)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// saveProfile writes p under dir and returns the path it wrote. A profile with
// scripts becomes a directory holding profile.yml plus a copy of each script,
// so it is self-contained; one without stays a flat <id>.yml.
func saveProfile(dir string, p catalog.Profile) (string, error) {
	if len(p.Scripts) == 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create profiles directory: %w", err)
		}
		path := filepath.Join(dir, p.ID+".yml")
		return path, writeProfileManifest(path, p)
	}

	profileDir := filepath.Join(dir, p.ID)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create profile directory: %w", err)
	}
	for _, s := range p.Scripts {
		if s.Source == "" {
			continue
		}
		// Goes through the domain reader so copying a repo-shipped profile lands
		// its embedded scripts on disk just like a user profile's.
		data, err := domain.ReadCustomScript(s)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(profileDir, s.File), data, 0o755); err != nil {
			return "", fmt.Errorf("failed to copy script %s: %w", s.File, err)
		}
	}
	path := filepath.Join(profileDir, catalog.ProfileFileName)
	return path, writeProfileManifest(path, p)
}

func writeProfileManifest(path string, p catalog.Profile) error {
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	return nil
}
