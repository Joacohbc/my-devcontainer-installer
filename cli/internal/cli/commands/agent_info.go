package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/ui"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func newAgentCLIInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli-info",
		Short: "Print the catalogue an 'agent create' is composed from",
		Long: `devcontainer-cli agent cli-info — print what this CLI can put in a
devcontainer: the Dockerfile modules, the compose services, the profiles that
bundle them, the agent skills, how to add scripts of your own, the built-in
scripts 'agent copy --asset' can drop into a container, and the paths every
project follows.

Read it before composing an 'agent create': it is generated from the live
catalogue, including any profile or skill defined under ~/.devcontainer-cli/,
so the ids it lists are the ids this binary accepts. --json emits the same
content structured.

This describes the CLI. For what a specific running container has inside it,
use 'context' instead.`,
		Example: `  # Human-readable catalogue
  devcontainer-cli agent cli-info

  # Structured, for a script or an agent to parse
  devcontainer-cli agent cli-info --json`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runAgentCLIInfo,
	}
	cmd.Flags().Bool("json", false, "Emit the catalogue as JSON instead of the human-readable report")
	return cmd
}

func runAgentCLIInfo(cmd *cobra.Command, _ []string) error {
	info := service.AgentService{Report: console}.Info(version)

	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		out, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	printAgentInfo(info)
	return nil
}

func printAgentInfo(info service.AgentInfo) {
	console.Header("devcontainer-cli %s — what you can build with it", info.Version)
	console.Print("\n")

	printAgentModules(info)
	printAgentServices(info)
	printAgentProfiles(info)
	printAgentSkills(info)
	printAgentScripts(info)
	printAgentAssets(info)
	printAgentPaths(info)
}

// agentInfoSection prints a titled section, or a subtle placeholder when empty.
func agentInfoSection(title string, lines []string) {
	console.Print(ui.HeaderS(title) + "\n")
	if len(lines) == 0 {
		console.Print("  " + ui.Subtle("(none)") + "\n\n")
		return
	}
	for _, line := range lines {
		console.Print("  " + line + "\n")
	}
	console.Print("\n")
}

// agentInfoRow renders one id/description row, padding the plain id before
// styling it so ANSI codes do not throw off the column alignment.
func agentInfoRow(id string, width int, rest ...string) string {
	return fmt.Sprintf("%s  %s", ui.Bold(fmt.Sprintf("%-*s", width, id)), strings.Join(rest, "  "))
}

func agentIDWidth(ids []string) int {
	width := 0
	for _, id := range ids {
		if len(id) > width {
			width = len(id)
		}
	}
	return width
}

func printAgentModules(info service.AgentInfo) {
	ids := make([]string, 0, len(info.Modules))
	for _, m := range info.Modules {
		ids = append(ids, m.ID)
	}
	width := agentIDWidth(ids)

	var selectable, automatic []string
	for _, m := range info.Modules {
		notes := []string{ui.Subtle(m.Label)}
		if len(m.Requires) > 0 {
			notes = append(notes, ui.Subtle("requires: "+strings.Join(m.Requires, ", ")))
		}
		if len(m.Conflicts) > 0 {
			notes = append(notes, ui.Subtle("conflicts: "+strings.Join(m.Conflicts, ", ")))
		}
		for _, env := range m.RequiresEnv {
			notes = append(notes, ui.Subtle("env: "+env.Name))
		}
		row := agentInfoRow(m.ID, width, notes...)
		if m.Selectable {
			selectable = append(selectable, row)
			continue
		}
		reason := "always applied"
		if m.Internal {
			reason = "added for you by --skill"
		}
		automatic = append(automatic, agentInfoRow(m.ID, width, ui.Subtle(m.Label), ui.Subtle("("+reason+")")))
	}

	agentInfoSection("Modules — ids for 'agent create --with a,b,c'", selectable)
	agentInfoSection("Modules applied automatically — never pass these to --with", automatic)
}

func printAgentServices(info service.AgentInfo) {
	ids := make([]string, 0, len(info.Services))
	for _, s := range info.Services {
		ids = append(ids, s.ID)
	}
	width := agentIDWidth(ids)

	var lines []string
	for _, s := range info.Services {
		if !s.Selectable {
			continue
		}
		notes := []string{ui.Subtle(s.Label)}
		if s.RequiresModule != "" {
			notes = append(notes, ui.Subtle("client module: "+s.RequiresModule))
		}
		lines = append(lines, agentInfoRow(s.ID, width, notes...))
	}
	agentInfoSection("Services — ids for 'agent create --service a,b'; reachable inside by service name", lines)
}

func printAgentProfiles(info service.AgentInfo) {
	ids := make([]string, 0, len(info.Profiles))
	for _, p := range info.Profiles {
		ids = append(ids, p.ID)
	}
	width := agentIDWidth(ids)

	var lines []string
	for _, p := range info.Profiles {
		tag := ui.Subtle("[local]")
		if p.Remote {
			tag = ui.Subtle("[remote]")
		}
		notes := []string{tag, ui.Subtle(strings.Join(p.Modules, ", "))}
		if len(p.Scripts) > 0 {
			notes = append(notes, ui.Subtle("scripts: "+strings.Join(p.Scripts, ", ")))
		}
		if len(p.Skills) > 0 {
			notes = append(notes, ui.Subtle("skills: "+strings.Join(p.Skills, ", ")))
		}
		if len(p.Ports) > 0 {
			notes = append(notes, ui.Subtle("ports: "+strings.Join(p.Ports, ", ")))
		}
		lines = append(lines, agentInfoRow(p.ID, width, notes...))
	}
	agentInfoSection("Profiles — ids for 'agent create --profile <id>'", lines)
	console.Print("  " + ui.Subtle("[remote] ids also work as a pull target: 'agent create --mode profiles --profile <id>'.") + "\n")
	console.Print("  " + ui.Subtle("A [local] one has no published image; it builds under the default --mode custom.") + "\n\n")
}

func printAgentSkills(info service.AgentInfo) {
	ids := make([]string, 0, len(info.Skills))
	for _, s := range info.Skills {
		ids = append(ids, s.ID)
	}
	width := agentIDWidth(ids)

	var lines []string
	for _, s := range info.Skills {
		notes := []string{ui.Subtle(s.Label), ui.Subtle(s.InstallRef)}
		if len(s.RequiresModules) > 0 {
			notes = append(notes, ui.Subtle("needs: "+strings.Join(s.RequiresModules, ", ")))
		}
		lines = append(lines, agentInfoRow(s.ID, width, notes...))
	}
	agentInfoSection("Agent skills — ids for 'agent create --skill a,b'; installed into the project workspace", lines)
}

func printAgentScripts(info service.AgentInfo) {
	lines := []string{
		"Add a script of your own with " + ui.Bold(info.Scripts.Flag) + " (repeatable).",
		ui.Subtle("The file name must match " + info.Scripts.NamePattern + "; default lifecycle is " + info.Scripts.Default + "."),
		"",
	}
	width := agentIDWidth(func() []string {
		out := make([]string, 0, len(info.Scripts.Whens))
		for _, w := range info.Scripts.Whens {
			out = append(out, w.When)
		}
		return out
	}())
	for _, w := range info.Scripts.Whens {
		lines = append(lines,
			agentInfoRow(w.When, width, ui.Subtle(w.Description)),
			fmt.Sprintf("%s  %s", strings.Repeat(" ", width), ui.Subtle("lands in: "+w.Location)),
		)
	}
	agentInfoSection("Scripts — how to run your own code in the container", lines)
}

func printAgentAssets(info service.AgentInfo) {
	ids := make([]string, 0, len(info.Assets))
	for _, a := range info.Assets {
		ids = append(ids, a.Name)
	}
	width := agentIDWidth(ids)

	lines := make([]string, 0, len(info.Assets))
	for _, a := range info.Assets {
		lines = append(lines, agentInfoRow(a.Name, width, ui.Subtle(a.Label)))
	}
	agentInfoSection("Built-in scripts — names for 'agent copy --asset <name>'", lines)
}

func printAgentPaths(info service.AgentInfo) {
	p := info.Paths
	rows := [][2]string{
		{"project mount", p.WorkspaceMount},
		{"short alias", p.WorkspaceAlias},
		{"home", p.DevUserHome},
		{"container context", p.ContextFile},
		{"generated files", p.ProjectDir},
		{"project config", p.ConfigFile},
		{"start scripts", p.PostScriptStartDir},
		{"manual scripts", p.PostScriptDir},
		{"devcontainer name", p.DevcontainerName},
	}
	labels := make([]string, 0, len(rows))
	for _, r := range rows {
		labels = append(labels, r[0])
	}
	width := agentIDWidth(labels)

	lines := make([]string, 0, len(rows)+2)
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-*s  %s", width, r[0], ui.Subtle(r[1])))
	}
	lines = append(lines, "", ui.Subtle("Nothing outside the project mount and "+p.DevUserHome+" survives a recreate."))
	agentInfoSection("Paths — "+p.WorkspacePlaceholder+" is the project's workspace name", lines)
}
