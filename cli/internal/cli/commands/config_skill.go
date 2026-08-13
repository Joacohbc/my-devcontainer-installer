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
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/modules/skills"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// The skill group is attached under `config` (see newConfigCommand); it is not
// registered as a top-level command. Not to be confused with the top-level
// `skill` command, which installs the host-side devcontainer-cli skill — this
// one manages the agent skills selectable with --skill when generating a
// project.
func newConfigSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "List built-in and user-defined agent skills",
		Long: `devcontainer-cli config skill — the agent skills selectable with --skill (or
in the interactive wizard), installed project-scoped into the workspace via
the Skills CLI ('npx skills add').

Built-in skills ship with the CLI. Your own are saved as
~/.devcontainer-cli/skills/<id>.yml — a flat manifest, no scripts or directory
shape, since a skill's install source is already a git ref, not files this
repo ships:

  id: my-skill
  label: My Skill (does X)
  ref: owner/repo          # or a full repository URL
  skill: skill-name        # only needed when ref holds more than one skill
  requires_modules: [nodejs]
  context:                 # optional — its entry in the generated ~/CONTEXT.md
    title: My Skill
    body: |
      What it teaches an agent to do.

A user-defined id shadows a built-in of the same id. Pass it to
'--skill <id>' (comma-separated for several).

Subcommands:
  list          List built-in and user-defined skills.
  info <id>     Print one skill's full resolved definition.
  add <id>      Create (or, with --force, overwrite) a user-defined skill.
  remove <id..> Delete user-defined skills (built-ins cannot be removed).`,
		Example: `  devcontainer-cli config skill list
  devcontainer-cli config skill info firecrawl
  devcontainer-cli config skill add my-skill --ref owner/repo
  devcontainer-cli config skill remove my-skill
  devcontainer-cli --with nodejs --skill my-skill`,
	}
	cmd.AddCommand(newConfigSkillListCommand())
	cmd.AddCommand(newConfigSkillInfoCommand())
	cmd.AddCommand(newConfigSkillAddCommand())
	cmd.AddCommand(newConfigSkillRemoveCommand())
	return cmd
}

func newConfigSkillListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List built-in and user-defined agent skills",
		Long:         "devcontainer-cli config skill list — show every available agent skill, grouped\ninto built-in and your own, with the modules each one requires.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			all := (service.ConfigService{Report: console}).AgentSkills()

			var builtin, user []*skills.Spec
			width := 0
			for _, s := range all {
				if len(s.ID) > width {
					width = len(s.ID)
				}
				if isBuiltinSkill(s.ID) {
					builtin = append(builtin, s)
				} else {
					user = append(user, s)
				}
			}

			printGroup := func(title string, group []*skills.Spec, emptyHint string) {
				console.Print(ui.HeaderS(title) + "\n")
				if len(group) == 0 {
					console.Print("  " + ui.Subtle(emptyHint) + "\n")
					return
				}
				for _, s := range group {
					// Pad the plain id first, then style it, so ANSI codes don't
					// throw off column alignment.
					id := ui.Bold(fmt.Sprintf("%-*s", width, string(s.ID)))
					console.Print(fmt.Sprintf("  %s  %s\n", id, ui.Subtle(s.Label)))
					if len(s.RequiresModules) > 0 {
						mods := make([]string, 0, len(s.RequiresModules))
						for _, m := range s.RequiresModules {
							mods = append(mods, string(m))
						}
						console.Print(fmt.Sprintf("  %s  %s\n", strings.Repeat(" ", width), ui.Subtle("requires: "+strings.Join(mods, ", "))))
					}
				}
			}

			printGroup("Built-in skills", builtin, "(none)")
			console.Print("\n")
			printGroup("Your skills", user, "(none — add one at ~/.devcontainer-cli/skills/<id>.yml)")
			return nil
		},
	}
}

// isBuiltinSkill reports whether id names one of the skills shipped with the
// CLI (catalog.AgentSkills) rather than one loaded from a user's skill dir.
func isBuiltinSkill(id types.SkillID) bool {
	for _, s := range catalog.AgentSkills {
		if s.ID == id {
			return true
		}
	}
	return false
}

// skillInfoView is the yaml shape 'config skill info' prints — the same
// fields catalog's (unexported) userSkillManifest reads, so a built-in and a
// user-defined skill render identically regardless of which one is actually
// backed by a yaml file on disk.
type skillInfoView struct {
	ID              string   `yaml:"id"`
	Label           string   `yaml:"label,omitempty"`
	Ref             string   `yaml:"ref"`
	Skill           string   `yaml:"skill,omitempty"`
	RequiresModules []string `yaml:"requires_modules,omitempty"`
	Context         *struct {
		Title string `yaml:"title"`
		Body  string `yaml:"body"`
	} `yaml:"context,omitempty"`
}

func newConfigSkillInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <skill-id>",
		Short: "Show one agent skill's full resolved definition",
		Long: `devcontainer-cli config skill info — print one skill's full definition: built-in
or user-defined, its install ref, required modules, and its ~/CONTEXT.md
entry when it has one.`,
		Example:           "  devcontainer-cli config skill info firecrawl",
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		RunE:              runConfigSkillInfo,
		ValidArgsFunction: completeAgentSkillArgs,
	}
}

func runConfigSkillInfo(cmd *cobra.Command, args []string) error {
	id := types.SkillID(args[0])
	spec := catalog.GetAgentSkill(id, domain.SkillDirs()...)
	if spec == nil {
		return fmt.Errorf("skill %q not found", id)
	}

	source := "user"
	if isBuiltinSkill(spec.ID) {
		source = "built-in"
	}

	console.Header("%s", spec.ID)
	console.NewLine()
	console.Info("  Source:  %s", source)
	console.Info("  Install: %s", spec.InstallRef())
	console.NewLine()

	view := skillInfoView{ID: string(spec.ID), Label: spec.Label, Ref: spec.Ref, Skill: spec.Skill}
	for _, m := range spec.RequiresModules {
		view.RequiresModules = append(view.RequiresModules, string(m))
	}
	if spec.Context != nil {
		sec := spec.Context()
		view.Context = &struct {
			Title string `yaml:"title"`
			Body  string `yaml:"body"`
		}{Title: sec.Title, Body: sec.Body}
	}

	data, err := yaml.Marshal(view)
	if err != nil {
		return err
	}
	console.Print(string(data))
	return nil
}

// userSkillPath returns the on-disk path of a user-defined skill id, in
// either accepted extension.
func userSkillPath(id string) (string, bool) {
	for _, ext := range []string{".yml", ".yaml"} {
		p := filepath.Join(domain.SkillDir(), id+ext)
		if fileExists(p) {
			return p, true
		}
	}
	return "", false
}

func newConfigSkillAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <skill-id>",
		Short: "Create (or overwrite) a user-defined agent skill",
		Long: `devcontainer-cli config skill add — write a user-defined agent skill at
~/.devcontainer-cli/skills/<id>.yml, selectable with --skill exactly like a
built-in one — including under an id that already names a built-in, which
your own then shadows.

--ref is the only field that cannot be left empty (the source the Skills CLI
resolves — an owner/repo shorthand or a repository URL); everything else is
optional. A value missing from the flags is prompted for when interactive;
with --no-interactive every value must come from a flag.`,
		Example: `  devcontainer-cli config skill add my-skill --ref owner/repo --label "My Skill"
  devcontainer-cli config skill add my-skill --ref owner/repo --requires-modules nodejs,python
  devcontainer-cli config skill add firecrawl --ref me/firecrawl --force   # shadow the built-in`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runConfigSkillAdd,
	}
	cmd.Flags().String("label", "", "Human-readable label")
	cmd.Flags().String("ref", "", "Install source the Skills CLI resolves (owner/repo, or a repository URL)")
	cmd.Flags().String("skill", "", "Skill selector, only needed when --ref holds more than one skill")
	cmd.Flags().String("requires-modules", "", "Comma-separated Dockerfile module ids this skill's tooling needs")
	cmd.Flags().String("context-title", "", "Title of its ~/CONTEXT.md entry (optional; defaults to the label or id)")
	cmd.Flags().String("context-body", "", "Body of its ~/CONTEXT.md entry (optional; a title with no body is dropped)")
	cmd.Flags().Bool("force", false, "Overwrite an existing user-defined skill with the same id")
	addInteractiveFlag(cmd)
	_ = cmd.RegisterFlagCompletionFunc("requires-modules", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ModuleIDs()), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func runConfigSkillAdd(cmd *cobra.Command, args []string) error {
	id := args[0]
	if err := domain.ValidateSkillID(id); err != nil {
		return err
	}

	interactive := interactiveFlag(cmd)
	force, _ := cmd.Flags().GetBool("force")
	if _, ok := userSkillPath(id); ok && !force {
		return fmt.Errorf("user-defined skill %q already exists; pass --force to overwrite", id)
	}

	label, _ := cmd.Flags().GetString("label")
	ref, _ := cmd.Flags().GetString("ref")
	skillSel, _ := cmd.Flags().GetString("skill")
	rawModules, _ := cmd.Flags().GetString("requires-modules")
	ctxTitle, _ := cmd.Flags().GetString("context-title")
	ctxBody, _ := cmd.Flags().GetString("context-body")

	if ref == "" {
		if !interactive {
			return fmt.Errorf("--ref required in non-interactive mode")
		}
		var err error
		ref, err = console.Ask("Install ref (owner/repo, or a repository URL):")
		if err != nil {
			return err
		}
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("--ref cannot be empty")
	}

	if label == "" && interactive && !cmd.Flags().Changed("label") {
		asked, err := console.Ask("Label (optional, empty to skip):")
		if err != nil {
			return err
		}
		label = asked
	}

	if isBuiltinSkill(types.SkillID(id)) {
		console.Info("Note: %q is also a built-in skill id; yours takes precedence.", id)
	}

	view := skillInfoView{ID: id, Label: strings.TrimSpace(label), Ref: ref, Skill: strings.TrimSpace(skillSel)}
	for _, m := range splitCSV(rawModules) {
		view.RequiresModules = append(view.RequiresModules, m)
	}
	if ctxBody != "" {
		title := ctxTitle
		if title == "" {
			title = view.Label
		}
		if title == "" {
			title = id
		}
		view.Context = &struct {
			Title string `yaml:"title"`
			Body  string `yaml:"body"`
		}{Title: title, Body: ctxBody}
	}

	data, err := yaml.Marshal(view)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(domain.SkillDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}
	path := filepath.Join(domain.SkillDir(), id+".yml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to save skill: %w", err)
	}

	console.Success("Skill %q successfully created at %s", id, path)
	return nil
}

func newConfigSkillRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <skill-id...>",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete user-defined agent skills (built-ins cannot be removed)",
		Long: `devcontainer-cli config skill remove — delete one or more of your own agent
skills from ~/.devcontainer-cli/skills/.

Only user-defined skills can be removed; attempting to remove a built-in
errors out. Every id is validated before anything is deleted, so a bad id
aborts the whole operation. Skill ids tab-complete.`,
		Example: `  devcontainer-cli config skill remove my-skill
  devcontainer-cli config skill rm old-skill another --yes`,
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      true,
		RunE:              runConfigSkillRemove,
		ValidArgsFunction: completeUserSkillArgs,
	}
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func runConfigSkillRemove(cmd *cobra.Command, args []string) error {
	// Resolve every target up front so we fail before deleting anything.
	paths := make([]string, 0, len(args))
	for _, id := range args {
		path, ok := userSkillPath(id)
		if !ok {
			if isBuiltinSkill(types.SkillID(id)) {
				return fmt.Errorf("skill %q is a built-in skill and cannot be removed", id)
			}
			return fmt.Errorf("user-defined skill %q not found", id)
		}
		paths = append(paths, path)
	}

	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("refusing to remove skill(s) without confirmation; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.Confirm(fmt.Sprintf("Remove %d user-defined skill(s): %s?", len(args), strings.Join(args, ", ")))
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
			return fmt.Errorf("failed to remove skill %q: %w", args[i], err)
		}
		console.Success("Removed skill %q", args[i])
	}
	return nil
}

func completeUserSkillArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, s := range catalog.LoadUserSkills(domain.SkillDir()) {
		id := string(s.ID)
		if strings.HasPrefix(id, toComplete) && !slices.Contains(args, id) {
			out = append(out, id)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeAgentSkillArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return catalog.AgentSkillIDs(domain.SkillDirs()...), cobra.ShellCompDirectiveNoFileComp
}
