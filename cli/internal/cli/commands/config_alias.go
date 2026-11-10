package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

// The alias group is attached under `config` (see newConfigCommand); it is not
// registered as a top-level command.
func newConfigAliasCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage your own shell aliases for every container",
		Long: `devcontainer-cli config alias — manage your own shell aliases.

Aliases are stored in the CLI config (not in a file you hand-edit) and shared by
every container. Each container sources two files, in this order:

  ~/.devcontainer_aliases.sh   the CLI's defaults, baked into the image
                               (kill_port, npm->pnpm, pip->uv, prompt-free agents)
  ~/.alias.sh                  YOURS — rendered from these aliases

Because yours is sourced last it always wins. Set them with 'config alias set',
then push them into the shared volume with 'config shared sync' so every
container picks them up — no image rebuild, no restart (a new shell is enough).`,
		Example: `  # List configured aliases
  devcontainer-cli config alias list

  # Add or update one, then apply it everywhere
  devcontainer-cli config alias set ll "ls -la"
  devcontainer-cli config shared sync

  # Remove one
  devcontainer-cli config alias unset ll`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newConfigAliasListCommand())
	cmd.AddCommand(newConfigAliasSetCommand())
	cmd.AddCommand(newConfigAliasUnsetCommand())
	return cmd
}

func newConfigAliasListCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List configured shell aliases",
		Long:         `devcontainer-cli config alias list — print every shell alias currently stored in the CLI config.`,
		Example:      `  devcontainer-cli config alias list`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runConfigAliasList,
	}
}

func newConfigAliasSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <command>",
		Short: "Add or update an alias in the CLI config",
		Long: `devcontainer-cli config alias set — store an alias in the CLI config.

The name must be a shell identifier (letters, digits, '_', '-', '.'); the command
is everything after it. This only writes the config — run 'config shared sync' to
push it into running containers.`,
		Example: `  devcontainer-cli config alias set gs "git status"
  devcontainer-cli config alias set k kubectl`,
		Args:         cobra.MinimumNArgs(2),
		SilenceUsage: true,
		RunE:         runConfigAliasSet,
	}
}

func newConfigAliasUnsetCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "unset <name>",
		Short:             "Remove an alias from the CLI config",
		Long:              `devcontainer-cli config alias unset — remove an alias. Run 'config shared sync' afterwards to drop it from running containers.`,
		Example:           `  devcontainer-cli config alias unset gs`,
		Args:              cobra.ExactArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completeAliasNames,
		RunE:              runConfigAliasUnset,
	}
}

func runConfigAliasList(_ *cobra.Command, _ []string) error {
	svc := service.ConfigService{Report: console}
	aliases := svc.Aliases()
	console.Info("Aliases stored in %s", domain.GlobalConfigPath())
	if len(aliases) == 0 {
		console.Warn("No aliases configured yet (add one with: config alias set <name> <command>).")
		return nil
	}
	console.NewLine()
	for _, name := range sortedAliasNames(aliases) {
		console.Print(fmt.Sprintf("  %s = %s\n", name, aliases[name]))
	}
	console.NewLine()
	console.Info("Apply them to every container with: devcontainer-cli config shared sync")
	return nil
}

func runConfigAliasSet(_ *cobra.Command, args []string) error {
	svc := service.ConfigService{Report: console}
	name, command := args[0], strings.Join(args[1:], " ")
	if err := svc.SetAlias(name, command); err != nil {
		return err
	}
	console.Info("Apply it to every container with: devcontainer-cli config shared sync")
	return nil
}

func runConfigAliasUnset(_ *cobra.Command, args []string) error {
	svc := service.ConfigService{Report: console}
	existed, err := svc.UnsetAlias(args[0])
	if err != nil {
		return err
	}
	if !existed {
		console.Warn("No alias named %q.", args[0])
		return nil
	}
	console.Info("Apply the removal to every container with: devcontainer-cli config shared sync")
	return nil
}

// completeAliasNames tab-completes existing alias names for `unset`.
func completeAliasNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	svc := service.ConfigService{Report: console}
	return sortedAliasNames(svc.Aliases()), cobra.ShellCompDirectiveNoFileComp
}

// sortedAliasNames returns the alias names in a stable order, so listing and
// completion never disagree about it (Go map iteration is randomized).
func sortedAliasNames(aliases map[string]string) []string {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
