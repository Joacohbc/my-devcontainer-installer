package commands

import (
	"fmt"
	"os"
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
  list                  List built-in and user-defined skills.
  info <id>             Print one skill's full resolved definition.
  add <id|source>       Create a user-defined skill, or discover from a source.
  remove <id..>         Delete user-defined skills (built-ins cannot be removed).
  install [skill-ids..] Install agent skills at project level in an ephemeral container.`,
		Example: `  devcontainer-cli config skill list
  devcontainer-cli config skill info agent-browser
  devcontainer-cli config skill add anthropics/skills
  devcontainer-cli config skill add anthropics/skills --all
  devcontainer-cli config skill add anthropics/skills --pick pdf
  devcontainer-cli config skill add my-skill --ref owner/repo
  devcontainer-cli config skill remove my-skill
  devcontainer-cli config skill install agent-browser
  devcontainer-cli config skill install --all
  devcontainer-cli --with nodejs --skill my-skill`,
	}
	cmd.AddCommand(newConfigSkillListCommand())
	cmd.AddCommand(newConfigSkillInfoCommand())
	cmd.AddCommand(newConfigSkillAddCommand())
	cmd.AddCommand(newConfigSkillRemoveCommand())
	cmd.AddCommand(newConfigSkillInstallCommand())
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

func newConfigSkillInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <skill-id>",
		Short: "Show one agent skill's full resolved definition",
		Long: `devcontainer-cli config skill info — print one skill's full definition: built-in
or user-defined, its install ref, required modules, and its ~/CONTEXT.md
entry when it has one.`,
		Example:           "  devcontainer-cli config skill info agent-browser",
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

	view := types.UserSkillManifest{ID: string(spec.ID), Label: spec.Label, Ref: spec.Ref, Skill: spec.Skill}
	for _, m := range spec.RequiresModules {
		view.RequiresModules = append(view.RequiresModules, string(m))
	}
	if spec.Context != nil {
		sec := spec.Context()
		view.Context = &types.UserSkillContext{Title: sec.Title, Body: sec.Body}
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
	return domain.UserSkillPath(id)
}

var configSkillServiceFactory = func() service.ConfigService {
	return service.ConfigService{Report: console}
}

var skillPrompter service.Prompter = console

func newConfigSkillAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <skill-id|source>",
		Short: "Add a user-defined agent skill or discover skills from a source",
		Long: `devcontainer-cli config skill add — add user-defined agent skills saved at
~/.devcontainer-cli/skills/<id>.yml, selectable with --skill exactly like built-in
skills.

When passed a repository or source (containing '/', ':', or starting with '.'),
it queries the Skills CLI to discover available skills and lets you choose which
ones to save. Use --all to save all discovered skills, or --pick a,b to select
specific skills without prompting. In non-interactive mode, either --all or --pick
is required.

When passed a skill id without source indicators, it creates (or with --force
overwrites) a single user-defined skill definition. --ref is required in this mode.`,
		Example: `  # Discover skills from a repository and pick interactively
  devcontainer-cli config skill add anthropics/skills

  # Discover and save all skills non-interactively
  devcontainer-cli config skill add anthropics/skills --all

  # Discover and pick specific skills
  devcontainer-cli config skill add anthropics/skills --pick pdf,convex-best-practices

  # Add a single skill manually
  devcontainer-cli config skill add my-skill --ref owner/repo --label "My Skill"
  devcontainer-cli config skill add my-skill --ref owner/repo --requires-modules nodejs,python
  devcontainer-cli config skill add agent-browser --ref me/agent-browser --force   # shadow the built-in`,
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
	cmd.Flags().Bool("all", false, "Select all discovered skills from source")
	cmd.Flags().String("pick", "", "Comma-separated list of discovered skill names or IDs to save")
	addInteractiveFlag(cmd)
	_ = cmd.RegisterFlagCompletionFunc("requires-modules", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeCSV(toComplete, catalog.ModuleIDs()), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func isSkillSourceArg(arg string) bool {
	if strings.HasPrefix(arg, "-") {
		return false
	}
	return strings.Contains(arg, "/") || strings.Contains(arg, ":") || strings.HasPrefix(arg, ".")
}

func runConfigSkillAdd(cmd *cobra.Command, args []string) error {
	arg := args[0]
	if strings.HasPrefix(arg, "-") {
		return fmt.Errorf("invalid source %q: source cannot start with '-'", arg)
	}
	if isSkillSourceArg(arg) {
		return runConfigSkillAddDiscovery(cmd, arg)
	}
	return runConfigSkillAddManual(cmd, arg)
}

func runConfigSkillAddDiscovery(cmd *cobra.Command, source string) error {
	allFlag, _ := cmd.Flags().GetBool("all")
	pickFlag, _ := cmd.Flags().GetString("pick")
	force, _ := cmd.Flags().GetBool("force")
	interactive := interactiveFlag(cmd)

	if allFlag && pickFlag != "" {
		return fmt.Errorf("cannot specify both --all and --pick")
	}

	svc := configSkillServiceFactory()
	discovered, err := svc.DiscoverSkills(source)
	if err != nil {
		return err
	}

	var selected []*domain.DiscoveredSkill

	if allFlag {
		selected = discovered
	} else if pickFlag != "" {
		picks := splitCSV(pickFlag)
		matched, err := matchDiscoveredSkills(discovered, picks, source)
		if err != nil {
			return err
		}
		selected = matched
	} else if len(discovered) == 1 {
		selected = discovered
	} else if !interactive {
		return fmt.Errorf("non-interactive mode requires --all or --pick")
	} else {
		choices := make([]service.Option, 0, len(discovered))
		for _, d := range discovered {
			choices = append(choices, service.Option{
				Value: string(d.ID),
				Label: formatSkillDiscoveryOptionLabel(d),
			})
		}
		picked, err := skillPrompter.Multiselect("Select skills to save:", choices, nil)
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			console.Info("No skills selected.")
			return nil
		}
		for _, p := range picked {
			for _, d := range discovered {
				if string(d.ID) == p.Value {
					selected = append(selected, d)
					break
				}
			}
		}
	}

	return svc.SaveUserSkills(selected, force)
}

func matchDiscoveredSkills(discovered []*domain.DiscoveredSkill, picks []string, source string) ([]*domain.DiscoveredSkill, error) {
	var selected []*domain.DiscoveredSkill
	for _, pick := range picks {
		pick = strings.TrimSpace(pick)
		if pick == "" {
			continue
		}
		var found *domain.DiscoveredSkill
		for _, d := range discovered {
			if strings.EqualFold(string(d.ID), pick) || strings.EqualFold(d.Name, pick) {
				found = d
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("skill %q not found in %s", pick, source)
		}
		if !slices.Contains(selected, found) {
			selected = append(selected, found)
		}
	}
	return selected, nil
}

func formatSkillDiscoveryOptionLabel(d *domain.DiscoveredSkill) string {
	shortDesc := strings.TrimSpace(d.Description)
	if idx := strings.Index(shortDesc, "\n"); idx != -1 {
		shortDesc = strings.TrimSpace(shortDesc[:idx])
	}
	var parts []string
	if shortDesc != "" {
		parts = append(parts, fmt.Sprintf("%s — %s", d.Name, shortDesc))
	} else {
		parts = append(parts, d.Name)
	}
	if d.Snyk != "" {
		parts = append(parts, fmt.Sprintf("[snyk: %s]", d.Snyk))
	}
	return strings.Join(parts, " ")
}

func runConfigSkillAddManual(cmd *cobra.Command, id string) error {
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

	manifest := types.UserSkillManifest{
		ID:    id,
		Label: strings.TrimSpace(label),
		Ref:   ref,
		Skill: strings.TrimSpace(skillSel),
	}
	for _, m := range splitCSV(rawModules) {
		manifest.RequiresModules = append(manifest.RequiresModules, m)
	}
	if ctxBody != "" {
		title := ctxTitle
		if title == "" {
			title = manifest.Label
		}
		if title == "" {
			title = id
		}
		manifest.Context = &types.UserSkillContext{
			Title: title,
			Body:  ctxBody,
		}
	}

	svc := configSkillServiceFactory()
	return svc.SaveUserSkill(manifest, force)
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

func newConfigSkillInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [skill-ids...]",
		Short: "Install agent skills at project level in an ephemeral container",
		Long: `devcontainer-cli config skill install — install agent skills into the project workspace
using a temporary lightweight Node container ('npx --yes skills add').

No devcontainer stack or docker-compose services are started. Files written to the
project are owned by the host user.

If skill IDs are provided as arguments, only those skills are installed.
If no skill IDs are specified:
  - If devcontainer.config.json defines configured skills, those are installed.
  - If interactive and no skills are configured, prompts with a multiselect picker.
  - If non-interactive and no skills are configured, returns an actionable error.

Use --all to install all available agent skills (both built-in and user-defined).`,
		Example: `  # Install specific skills
  devcontainer-cli config skill install agent-browser

  # Install all available skills
  devcontainer-cli config skill install --all

  # Install skills configured in the current project
  devcontainer-cli config skill install

  # Target a specific project directory
  devcontainer-cli config skill install --dir /path/to/project

  # Use a custom container image
  devcontainer-cli config skill install --image node:22-slim`,
		SilenceUsage:      true,
		ValidArgsFunction: completeInstallSkillArgs,
		RunE:              runConfigSkillInstall,
	}
	addSkillInstallFlags(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func addSkillInstallFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "Install all available agent skills (built-in and user-defined)")
	cmd.Flags().String("image", service.DefaultSkillInstallerImage, "Docker image to run the skill installer in")
	cmd.Flags().String("dir", "", "Target project directory (default: current directory)")
}

func runConfigSkillInstall(cmd *cobra.Command, args []string) error {
	allSkills, _ := cmd.Flags().GetBool("all")
	imageName, _ := cmd.Flags().GetString("image")
	targetDir, _ := cmd.Flags().GetString("dir")
	isInteractive := interactiveFlag(cmd)

	skillService := service.SkillInstallService{
		Report: console,
		Prompt: console,
	}
	return skillService.Install(args, service.SkillInstallOptions{
		Dir:         targetDir,
		Image:       imageName,
		All:         allSkills,
		Interactive: isInteractive,
	})
}

func completeInstallSkillArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	allSkillIDs := catalog.AgentSkillIDs(domain.SkillDirs()...)
	var matchingIDs []string
	for _, id := range allSkillIDs {
		if !slices.Contains(args, id) && strings.HasPrefix(id, toComplete) {
			matchingIDs = append(matchingIDs, id)
		}
	}
	return matchingIDs, cobra.ShellCompDirectiveNoFileComp
}
