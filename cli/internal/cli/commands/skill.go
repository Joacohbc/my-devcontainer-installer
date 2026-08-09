package commands

import (
	"fmt"
	"os"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func init() { register(newSkillCommand()) }

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the agent skill that teaches a host AI assistant to use this CLI",
		Long: `devcontainer-cli skill — install the agent skill for THIS CLI on your machine.

The skill is a document an AI assistant running on the host (Claude Code,
Antigravity, …) reads to learn how to drive devcontainer-cli: generate a
devcontainer for a project, run builds and tests inside it, publish or forward
ports, inspect what the container has installed, and tear it down. Install it
once and your assistant can put work in a container instead of on your machine.

This is the host half of the CLI's agent support. The container half needs no
setup: every generated image already carries the 'devcontainer-context' skill,
which tells an agent running INSIDE a container where it is.

With no subcommand this lists the install targets and what is currently at each.

Subcommands:
  install    Write the skill into your agents' skill directories.
  remove     Delete the skills this CLI installed.
  show       Print the document to stdout (to pipe it somewhere else).`,
		Example: `  # See where the skill would go and what is installed today
  devcontainer-cli skill

  # Install it for every supported agent
  devcontainer-cli skill install

  # Only for Claude Code, and only in this project
  devcontainer-cli skill install --agent claude --scope project

  # Feed it to an agent that keeps skills somewhere else
  devcontainer-cli skill show > ~/my-agent/skills/devcontainer-cli.md`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runSkillStatus,
	}
	addSkillTargetFlags(cmd)
	cmd.AddCommand(newSkillInstallCommand())
	cmd.AddCommand(newSkillRemoveCommand())
	cmd.AddCommand(newSkillShowCommand())
	return cmd
}

func newSkillInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the skill into your agents' skill directories",
		Long: `devcontainer-cli skill install — write the skill into every supported agent's
skill directory (narrow it with --agent).

Re-running it upgrades a skill installed by an older version of the CLI, so run
it again after 'upgrade-cli'. A file the CLI did not write is never replaced
without --force. Agents load skills at session start, so restart yours after
installing.`,
		Example: `  # Every supported agent, in your home directory
  devcontainer-cli skill install

  # Just Claude Code
  devcontainer-cli skill install --agent claude

  # Scoped to the current project instead of the whole machine
  devcontainer-cli skill install --scope project`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runSkillInstall,
	}
	addSkillTargetFlags(cmd)
	cmd.Flags().Bool("force", false, "Replace a skill file at the target path that this CLI did not write")
	return cmd
}

func newSkillRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"uninstall"},
		Short:   "Remove the skill from your agents' skill directories",
		Long: `devcontainer-cli skill remove — delete the skill this CLI installed.

Only files the CLI wrote are removed; a skill of the same name you wrote
yourself is kept unless --force is passed. Targets with nothing installed are
reported and skipped.`,
		Example: `  devcontainer-cli skill remove
  devcontainer-cli skill remove --agent agents --yes`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runSkillRemove,
	}
	addSkillTargetFlags(cmd)
	cmd.Flags().Bool("force", false, "Also remove a skill file at the target path that this CLI did not write")
	addYesFlag(cmd)
	addInteractiveFlag(cmd)
	return cmd
}

func newSkillShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the skill document to stdout",
		Long: `devcontainer-cli skill show — print the skill document as shipped by this
binary, without installing anything. Use it to review the skill, or to place it
by hand for an agent whose skill directory the CLI does not know about.`,
		Example: `  devcontainer-cli skill show
  devcontainer-cli skill show > ~/.config/some-agent/skills/devcontainer-cli.md`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runSkillShow,
	}
}

// addSkillTargetFlags registers the flags that select which agents and which
// base directory a skill operation acts on.
func addSkillTargetFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("agent", nil, "Agents to act on (default: all); repeatable or comma-separated")
	cmd.Flags().String("scope", domain.SkillScopeGlobal, "Where the skill lives: global (your home, every project) or project (the current directory only)")
	_ = cmd.RegisterFlagCompletionFunc("agent", staticCompletion(domain.HostSkillAgentIDs()...))
	_ = cmd.RegisterFlagCompletionFunc("scope", staticCompletion(domain.SkillScopes...))
}

// skillTargets reads the --agent/--scope flags into the agents to act on and
// the base directory their skill dirs hang off.
func skillTargets(cmd *cobra.Command) ([]domain.SkillAgent, string, error) {
	ids, _ := cmd.Flags().GetStringSlice("agent")
	agents, err := domain.ResolveHostSkillAgents(ids)
	if err != nil {
		return nil, "", err
	}
	scope, _ := cmd.Flags().GetString("scope")
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	base, err := domain.HostSkillBaseDir(scope, cwd)
	if err != nil {
		return nil, "", err
	}
	return agents, base, nil
}

func runSkillStatus(cmd *cobra.Command, _ []string) error {
	agents, base, err := skillTargets(cmd)
	if err != nil {
		return err
	}
	svc := service.SkillService{Report: console}
	targets, err := svc.Status(base, agents)
	if err != nil {
		return err
	}
	console.Info("Skill '%s' — install targets under %s", domain.HostSkillName, base)
	console.NewLine()
	for _, t := range targets {
		console.Print(fmt.Sprintf("  %-12s %-9s %s\n", t.Agent.ID, t.State, t.Path))
	}
	console.NewLine()
	console.Info("Install or update it with: devcontainer-cli skill install")
	return nil
}

func runSkillInstall(cmd *cobra.Command, _ []string) error {
	agents, base, err := skillTargets(cmd)
	if err != nil {
		return err
	}
	force, _ := cmd.Flags().GetBool("force")
	svc := service.SkillService{Report: console}
	return svc.Install(base, agents, force)
}

func runSkillRemove(cmd *cobra.Command, _ []string) error {
	agents, base, err := skillTargets(cmd)
	if err != nil {
		return err
	}
	force, _ := cmd.Flags().GetBool("force")
	svc := service.SkillService{Report: console}
	if !yesFlag(cmd) {
		if !interactiveFlag(cmd) {
			return fmt.Errorf("removing the skill deletes files; pass --yes to confirm in non-interactive mode")
		}
		proceed, err := console.ConfirmDefault(fmt.Sprintf("Remove the '%s' skill from %s?", domain.HostSkillName, base), false)
		if err != nil {
			return err
		}
		if !proceed {
			console.Warn("Cancelled.")
			return nil
		}
	}
	return svc.Remove(base, agents, force)
}

func runSkillShow(_ *cobra.Command, _ []string) error {
	svc := service.SkillService{Report: console}
	content, err := svc.SkillDocument()
	if err != nil {
		return err
	}
	console.Print(string(content))
	return nil
}
