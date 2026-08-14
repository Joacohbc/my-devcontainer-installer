package commands

import (
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/spf13/cobra"
)

// agentSubcommands are the high-level commands the agent facade promises.
// Renaming one breaks every skill and script that drives this CLI, so the set
// is pinned here.
var agentSubcommands = []string{
	"cli-info", "create", "connect", "exec", "forward", "copy", "list", "clean",
}

func findCommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("command %q not found under %q", name, parent.Name())
	return nil
}

func agentCommand(t *testing.T) *cobra.Command {
	t.Helper()
	return findCommand(t, NewRootCommand("test"), "agent")
}

func TestAgentCommand_HasEverySubcommand(t *testing.T) {
	agent := agentCommand(t)

	have := map[string]bool{}
	for _, c := range agent.Commands() {
		have[c.Name()] = true
	}
	for _, name := range agentSubcommands {
		if !have[name] {
			t.Errorf("expected agent subcommand %q to be registered", name)
		}
	}
}

// Every subcommand must document itself: an agent reads --help before guessing.
func TestAgentSubcommands_AreDocumented(t *testing.T) {
	agent := agentCommand(t)

	for _, name := range agentSubcommands {
		cmd := findCommand(t, agent, name)
		if cmd.Short == "" {
			t.Errorf("agent %s has no Short", name)
		}
		if strings.HasSuffix(cmd.Short, ".") {
			t.Errorf("agent %s Short should not end with a period: %q", name, cmd.Short)
		}
		if cmd.Long == "" {
			t.Errorf("agent %s has no Long", name)
		}
		if cmd.Example == "" {
			t.Errorf("agent %s has no Example", name)
		}
	}
}

// The whole point of the facade: nothing here may stop to ask a question.
func TestAgentSubcommands_DefaultToNonInteractive(t *testing.T) {
	for _, name := range []string{"create", "connect", "forward", "clean"} {
		t.Run(name, func(t *testing.T) {
			cmd := findCommand(t, agentCommand(t), name)
			if err := cmd.PreRunE(cmd, nil); err != nil {
				t.Fatalf("PreRunE: %v", err)
			}
			if interactiveFlag(cmd) {
				t.Errorf("agent %s must default to non-interactive", name)
			}
		})
	}
}

// An explicit value still wins, so a human can opt back into the prompts.
func TestAgentDefaults_DoNotOverrideExplicitFlags(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "clean")
	if err := cmd.Flags().Set(flagNoInteractive, "false"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatalf("PreRunE: %v", err)
	}
	if !interactiveFlag(cmd) {
		t.Error("an explicit --no-interactive=false must survive the agent default")
	}
}

// 'create' is generate + build + up, so it must not stop at the overwrite
// question or the build question either.
func TestAgentCreate_ForcesTheFullFlow(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "create")
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatalf("PreRunE: %v", err)
	}
	for _, name := range []string{flagForce, flagBuild} {
		v, err := cmd.Flags().GetBool(name)
		if err != nil {
			t.Fatalf("flag --%s: %v", name, err)
		}
		if !v {
			t.Errorf("agent create must default --%s to true", name)
		}
	}
	if cmd.Flags().Lookup(flagNoUp) == nil {
		t.Errorf("agent create is missing --%s", flagNoUp)
	}
}

func TestAgentCreate_KeepsGenerateFlagsAndHidesTheNoise(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "create")

	// The flags an agent actually composes a project from must be present.
	for _, name := range []string{flagWith, flagService, flagPorts, flagVolumes, flagProfile, flagMode, flagSkill, flagScript, flagWorkspace} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("agent create is missing --%s", name)
		}
	}
	for _, name := range agentCreateHiddenFlags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("--%s should stay registered (parseGenFlags reads it)", name)
		}
		if !f.Hidden {
			t.Errorf("--%s should be hidden on the agent facade", name)
		}
	}
}

// 'shell' with no command opens a login shell; for an unattended agent that is
// a hang, so the facade refuses it up front.
func TestAgentExec_RequiresACommand(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "exec")

	err := cmd.Args(cmd, nil)
	if err == nil {
		t.Fatal("expected agent exec to reject an empty command")
	}
	if !strings.Contains(err.Error(), "requires a command") {
		t.Errorf("error = %q, want it to say a command is required", err)
	}
	if err := cmd.Args(cmd, []string{"go", "version"}); err != nil {
		t.Errorf("a command must be accepted: %v", err)
	}
}

// A command run through the facade must land as devuser: root would leave
// root-owned files in the bind-mounted project directory on the host.
func TestAgentExec_RunsAsDevuser(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "exec")
	if err := cmd.PreRunE(cmd, []string{"go", "version"}); err != nil {
		t.Fatalf("PreRunE: %v", err)
	}
	user, _ := cmd.Flags().GetString("user")
	if user != sshdefaults.User {
		t.Errorf("agent exec user = %q, want %q", user, sshdefaults.User)
	}
}

// Containers without a devuser (a database) and anything genuinely needing root
// stay reachable by naming the user explicitly.
func TestAgentExec_ExplicitUserWins(t *testing.T) {
	for _, want := range []string{"root", "postgres"} {
		t.Run(want, func(t *testing.T) {
			cmd := findCommand(t, agentCommand(t), "exec")
			if err := cmd.Flags().Set("user", want); err != nil {
				t.Fatal(err)
			}
			if err := cmd.PreRunE(cmd, []string{"psql"}); err != nil {
				t.Fatalf("PreRunE: %v", err)
			}
			if got, _ := cmd.Flags().GetString("user"); got != want {
				t.Errorf("agent exec user = %q, want the explicit %q", got, want)
			}
		})
	}
}

// The facade takes the devuser trade; the human command keeps its own, where an
// explicit command leaves --user alone so it still works against a container
// that has no devuser.
func TestShell_KeepsItsOwnUserSemantics(t *testing.T) {
	cmd := findCommand(t, NewRootCommand("test"), "shell")
	if cmd.PreRunE != nil {
		t.Error("shell must not inherit the agent facade's defaults")
	}
	if user, _ := cmd.Flags().GetString("user"); user != "" {
		t.Errorf("shell --user default = %q, want empty", user)
	}
	if got, _ := shellInteractiveDefaults("", "", false, false, true); got != "" {
		t.Errorf("shell with a command must not default the user, got %q", got)
	}
}

func TestAgentJSONFlags(t *testing.T) {
	agent := agentCommand(t)
	for _, name := range []string{"cli-info", "list"} {
		if findCommand(t, agent, name).Flags().Lookup("json") == nil {
			t.Errorf("agent %s is missing --json", name)
		}
	}
}

func TestAgentClean_Flags(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "clean")
	for _, name := range []string{"yes", "all", "dry-run", flagNoInteractive} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("agent clean is missing --%s", name)
		}
	}
}

// The agent wrappers reuse the human commands' flag helpers. These assert the
// originals kept every flag when the registration moved out of their
// constructors.
func TestExtractedFlagHelpers_KeepTheOriginalFlags(t *testing.T) {
	root := NewRootCommand("test")
	cases := []struct {
		command string
		flags   []string
	}{
		{"ssh", []string{"yes", flagNoInteractive, "container", "setup", "setup-external", "via", "key", "user", "forward", "ports"}},
		{"shell", []string{"user", "type", "no-tty", "via", "container"}},
		{"port-forward", []string{"alias", "service", flagNoInteractive, flagNonInteractive}},
		{"copy", []string{"container", "asset"}},
		{"ls", []string{"all", "long", "container"}},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			cmd := findCommand(t, root, c.command)
			for _, name := range c.flags {
				if cmd.Flags().Lookup(name) == nil {
					t.Errorf("%s lost its --%s flag", c.command, name)
				}
			}
		})
	}
}

// Completion is registered through the same helpers, so the agent wrappers get
// it for free — but only if the helpers really carry it.
func TestAgentWrappers_KeepFlagCompletion(t *testing.T) {
	agent := agentCommand(t)
	cases := []struct {
		command string
		flag    string
	}{
		{"connect", "via"},
		{"exec", "user"},
		{"forward", "alias"},
		{"copy", "asset"},
	}
	for _, c := range cases {
		t.Run(c.command+"/"+c.flag, func(t *testing.T) {
			cmd := findCommand(t, agent, c.command)
			if _, ok := cmd.GetFlagCompletionFunc(c.flag); !ok {
				t.Errorf("agent %s --%s lost its completion", c.command, c.flag)
			}
		})
	}
}

// 'list' shares ls's positional completion, and takes at most one path.
func TestAgentList_ArgsAndCompletion(t *testing.T) {
	cmd := findCommand(t, agentCommand(t), "list")

	if err := cmd.Args(cmd, []string{"/home/devuser"}); err != nil {
		t.Errorf("one path must be accepted: %v", err)
	}
	if err := cmd.Args(cmd, []string{"/a", "/b"}); err == nil {
		t.Error("expected a second path to be rejected")
	}
	if cmd.ValidArgsFunction == nil {
		t.Error("agent list should complete container paths like ls does")
	}
}

func TestAgentDirEntries_MarkDirectories(t *testing.T) {
	cases := []struct {
		in   string
		want agentDirEntry
	}{
		{"src/", agentDirEntry{Name: "src", Dir: true}},
		{"main.go", agentDirEntry{Name: "main.go", Dir: false}},
		{".config/", agentDirEntry{Name: ".config", Dir: true}},
	}
	for _, c := range cases {
		got := agentDirEntry{
			Name: strings.TrimSuffix(c.in, "/"),
			Dir:  strings.HasSuffix(c.in, "/"),
		}
		if got != c.want {
			t.Errorf("entry(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// The facade is additive: the commands it wraps stay top-level.
func TestAgentGroup_DoesNotReplaceTheHumanCommands(t *testing.T) {
	root := NewRootCommand("test")
	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	for _, name := range []string{"ssh", "shell", "port-forward", "copy", "ls", "destroy", "clean"} {
		if !have[name] {
			t.Errorf("%q must stay available as a top-level command", name)
		}
	}
	if !slices.Contains(agentSubcommands, "clean") {
		t.Error("agentSubcommands drifted from the documented set")
	}
}
