package commands

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/pick"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/catalog"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/assets"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func TestNewRootCommand_RegistersAllSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	want := []string{
		"ssh", "clean", "port-forward", "run", "down", "destroy",
		"start", "stop", "restart", "update",
		"upgrade-cli", "config", "cleanup-tips", "shell", "logs", "copy",
		"up", "status", "ls", "info", "network", "context", "compose",
		"agent",
	}
	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	for _, name := range want {
		if !have[name] {
			t.Errorf("expected subcommand %q to be registered", name)
		}
	}
}

func TestComposeCommand_AliasAndPassthrough(t *testing.T) {
	root := NewRootCommand("test")
	var compose *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "compose" {
			compose = c
			break
		}
	}
	if compose == nil {
		t.Fatal("compose command not registered")
	}
	if !slices.Contains(compose.Aliases, "dc") {
		t.Errorf("expected compose to have alias %q, got %v", "dc", compose.Aliases)
	}
	// Flag parsing must be disabled so tokens like "-it" reach docker compose
	// instead of being consumed (and rejected) by cobra.
	if !compose.DisableFlagParsing {
		t.Error("expected compose to disable flag parsing for passthrough")
	}
}

func TestProjectRoot(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain project dir", "/projects/app", "/projects/app"},
		{"inside build dir", "/projects/app/.dc_app/build", "/projects/app"},
		{"inside project dir root", "/projects/app/.dc_app", "/projects/app"},
		{"deeper under build", "/projects/app/.dc_app/build/scripts", "/projects/app"},
		{"workspace with dashes", "/projects/app/.dc_my-ws/build", "/projects/app"},
		{"bare .dc_ prefix is not a build segment", "/projects/app/.dc_", "/projects/app/.dc_"},
		{"unrelated dotdir untouched", "/projects/app/.docker/build", "/projects/app/.docker/build"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := projectRoot(c.in); got != c.want {
				t.Errorf("projectRoot(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCompleteComposeArgs(t *testing.T) {
	// First positional token → compose verbs, prefix-filtered.
	verbs, dir := completeComposeArgs(nil, nil, "ex")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if len(verbs) != 1 || !strings.HasPrefix(verbs[0], "exec\t") {
		t.Errorf("prefix 'ex' should complete to exec; got %v", verbs)
	}

	// Later tokens → the project's compose service keys (not container names).
	tempDir := t.TempDir()
	t.Chdir(tempDir)
	workspace := resolveWorkspace(tempDir)
	wsDir := filepath.Join(tempDir, ".dc_"+workspace, "build")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	doc := "services:\n  postgres:\n    image: postgres\n  redis:\n    image: redis\n"
	if err := os.WriteFile(filepath.Join(wsDir, "docker-compose.yml"), []byte(doc), 0644); err != nil {
		t.Fatal(err)
	}

	svcs, dir := completeComposeArgs(nil, []string{"exec"}, "")
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", dir)
	}
	if !slices.Equal(svcs, []string{"postgres", "redis"}) {
		t.Errorf("service completion = %v, want [postgres redis]", svcs)
	}

	// Service names are prefix-filtered too.
	filtered, _ := completeComposeArgs(nil, []string{"logs"}, "re")
	if !slices.Equal(filtered, []string{"redis"}) {
		t.Errorf("prefix 're' = %v, want [redis]", filtered)
	}
}

func TestCleanSubcommands_HaveAliases(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean command not registered")
	}

	want := map[string][]string{
		"containers": {"container", "rm"},
		"images":     {"image", "rmi"},
		"catalog":    {"deprecated", "registry"},
		"ssh":        {"sshs"},
		"networks":   {"network"},
		"volumes":    {"volume"},
	}

	for _, sub := range clean.Commands() {
		aliases, ok := want[sub.Name()]
		if !ok {
			continue
		}
		for _, a := range aliases {
			if !slices.Contains(sub.Aliases, a) {
				t.Errorf("expected clean subcommand %q to have alias %q, got %v", sub.Name(), a, sub.Aliases)
			}
		}
		delete(want, sub.Name())
	}
	for name := range want {
		t.Errorf("clean subcommand %q not found", name)
	}
}

func TestCleanCommands_AcceptArgsAndComplete(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean command not registered")
	}

	byName := map[string]*cobra.Command{}
	for _, c := range clean.Commands() {
		byName[c.Name()] = c
	}

	for _, name := range []string{"containers", "images"} {
		cmd := byName[name]
		if cmd == nil {
			t.Fatalf("clean subcommand %q not registered", name)
		}
		if cmd.Args != nil {
			if err := cmd.Args(cmd, []string{"some-name"}); err != nil {
				t.Errorf("clean %s should accept a positional arg: %v", name, err)
			}
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("clean %s should register positional-arg completion", name)
		}
	}
}

func TestCleanCommand_HasResourceSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean command not registered")
	}
	have := map[string]bool{}
	for _, c := range clean.Commands() {
		have[c.Name()] = true
	}
	for _, name := range []string{"catalog", "containers", "images", "ssh", "networks", "volumes", "all"} {
		if !have[name] {
			t.Errorf("expected clean subcommand %q", name)
		}
	}
}

func TestNetworkCommand_HasConnectAndDisconnect(t *testing.T) {
	root := NewRootCommand("test")
	var network *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "network" {
			network = c
			break
		}
	}
	if network == nil {
		t.Fatal("network command not registered")
	}
	have := map[string]*cobra.Command{}
	for _, c := range network.Commands() {
		have[c.Name()] = c
	}
	for _, name := range []string{"connect", "disconnect"} {
		sub := have[name]
		if sub == nil {
			t.Errorf("expected network subcommand %q", name)
			continue
		}
		if err := sub.Args(sub, nil); err == nil {
			t.Errorf("network %s should require at least one container argument", name)
		}
	}
	if have["connect"].Flags().Lookup("alias") == nil {
		t.Error("network connect missing --alias flag")
	}
	if have["disconnect"].Flags().Lookup("alias") != nil {
		t.Error("network disconnect should not have an --alias flag")
	}
}

func TestCleanAndSubcommands_HaveAllFlag(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean command not registered")
	}
	if clean.Flags().Lookup("all") == nil {
		t.Error("expected clean command to have --all flag")
	}

	byName := map[string]*cobra.Command{}
	for _, c := range clean.Commands() {
		byName[c.Name()] = c
	}

	for _, name := range []string{"containers", "images", "networks", "volumes", "all"} {
		sub := byName[name]
		if sub == nil {
			t.Errorf("clean subcommand %q not registered", name)
			continue
		}
		if sub.Flags().Lookup("all") == nil {
			t.Errorf("expected clean subcommand %q to have --all flag", name)
		}
	}
}

func TestRootCommand_HasGenerateFlags(t *testing.T) {
	root := NewRootCommand("test")
	for _, name := range []string{"mode", "profile", "with", "service", "image", "workspace", "force", "build", "no-build", "version"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("expected root flag --%s", name)
		}
	}
}

func TestCleanSshCommand_Flags(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean command not registered")
	}
	var sshSub *cobra.Command
	for _, c := range clean.Commands() {
		if c.Name() == "ssh" {
			sshSub = c
			break
		}
	}
	if sshSub == nil {
		t.Fatal("clean ssh subcommand not registered")
	}
	for _, name := range []string{"dry-run", "yes", "no-interactive"} {
		if sshSub.Flags().Lookup(name) == nil {
			t.Errorf("expected clean ssh flag --%s", name)
		}
	}
}

func TestParseGenFlags_Ports(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantNil bool
		want    []string
	}{
		{name: "unset", args: nil, wantNil: true},
		{name: "none", args: []string{"--ports", "none"}, want: []string{}},
		{name: "empty", args: []string{"--ports", ""}, want: []string{}},
		{name: "list", args: []string{"--ports", "8080:80, 5432:5432"}, want: []string{"8080:80", "5432:5432"}},
		{name: "explicit-ip", args: []string{"--ports", "0.0.0.0:8080:80"}, want: []string{"0.0.0.0:8080:80"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "generate"}
			addGenerateFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			flags, err := parseGenFlags(cmd)
			if err != nil {
				t.Fatalf("parseGenFlags: %v", err)
			}
			if tc.wantNil {
				if flags.ports != nil {
					t.Errorf("expected nil ports, got %v", *flags.ports)
				}
				return
			}
			if flags.ports == nil {
				t.Fatalf("expected non-nil ports for args %v", tc.args)
			}
			if len(*flags.ports) != len(tc.want) {
				t.Fatalf("ports = %v, want %v", *flags.ports, tc.want)
			}
			for i := range tc.want {
				if (*flags.ports)[i] != tc.want[i] {
					t.Errorf("ports[%d] = %q, want %q", i, (*flags.ports)[i], tc.want[i])
				}
			}
		})
	}
}

func TestCleanupInstructionLines(t *testing.T) {
	cfg := &types.DevcontainerConfig{Workspace: "myws"}
	out := strings.Join(cleanupInstructionLines(cfg), "\n")

	// Native commands must be surfaced as the primary action.
	for _, native := range []string{
		"devcontainer-cli ls",
		"devcontainer-cli down -v",
		"devcontainer-cli destroy",
		"devcontainer-cli clean containers",
		"devcontainer-cli clean images",
		"devcontainer-cli clean",
		"devcontainer-cli update",
	} {
		if !strings.Contains(out, native) {
			t.Errorf("expected cleanup tips to mention native command %q", native)
		}
	}

	// The raw docker equivalents must still be shown for reference.
	for _, raw := range []string{
		"docker compose down -v",
		"docker image prune -a",
		"docker compose build --no-cache",
	} {
		if !strings.Contains(out, raw) {
			t.Errorf("expected cleanup tips to keep docker equivalent %q", raw)
		}
	}
}

func TestParseGenFlags_Volumes(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantNil bool
		want    []string
	}{
		{name: "unset", args: nil, wantNil: true},
		{name: "none", args: []string{"--volumes", "none"}, want: []string{}},
		{name: "empty", args: []string{"--volumes", ""}, want: []string{}},
		{name: "list", args: []string{"--volumes", "myvol:/data, ./cache:/cache"}, want: []string{"myvol:/data", "./cache:/cache"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "generate"}
			addGenerateFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			flags, err := parseGenFlags(cmd)
			if err != nil {
				t.Fatalf("parseGenFlags: %v", err)
			}
			if tc.wantNil {
				if flags.volumes != nil {
					t.Errorf("expected nil volumes, got %v", *flags.volumes)
				}
				return
			}
			if flags.volumes == nil {
				t.Fatalf("expected non-nil volumes for args %v", tc.args)
			}
			if len(*flags.volumes) != len(tc.want) {
				t.Fatalf("volumes = %v, want %v", *flags.volumes, tc.want)
			}
			for i := range tc.want {
				if (*flags.volumes)[i] != tc.want[i] {
					t.Errorf("volumes[%d] = %q, want %q", i, (*flags.volumes)[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseGenFlags_SharedConfig(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantNil bool
		want    bool
	}{
		{name: "unset", args: nil, wantNil: true},
		{name: "explicit-true", args: []string{"--shared-config=true"}, want: true},
		{name: "explicit-false", args: []string{"--shared-config=false"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "generate"}
			addGenerateFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			flags, err := parseGenFlags(cmd)
			if err != nil {
				t.Fatalf("parseGenFlags: %v", err)
			}
			if tc.wantNil {
				if flags.sharedConfig != nil {
					t.Errorf("expected nil sharedConfig, got %v", *flags.sharedConfig)
				}
				return
			}
			if flags.sharedConfig == nil {
				t.Fatalf("expected non-nil sharedConfig for args %v", tc.args)
			}
			if *flags.sharedConfig != tc.want {
				t.Errorf("sharedConfig = %v, want %v", *flags.sharedConfig, tc.want)
			}
		})
	}
}

func TestRunCommand_HasSharedConfigFlag(t *testing.T) {
	cmd := newRunCommand()
	if cmd.Flags().Lookup("shared-config") == nil {
		t.Error("expected run --shared-config flag")
	}
}

func TestRunCommand_HasCopyAIScriptsFlag(t *testing.T) {
	cmd := newRunCommand()
	if cmd.Flags().Lookup("copy-ai-scripts") == nil {
		t.Error("expected run --copy-ai-scripts flag")
	}
}

func TestRunCommand_HasProfileFlag(t *testing.T) {
	cmd := newRunCommand()
	if cmd.Flags().Lookup("profile") == nil {
		t.Error("expected run --profile flag")
	}
	if cmd.Flags().Lookup("variant") != nil {
		t.Error("--variant was removed, not just deprecated; expected no such flag")
	}
}

// resolveRunProfile must reject a [local]-only profile (nothing published to
// pull) but accept "ssh" (the hand-built full image, not a catalog profile).
func TestResolveRunProfile_RejectsLocalOnlyAcceptsSSH(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cmd := newRunCommand()
	if err := cmd.Flags().Set("profile", "scraper"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRunProfile(cmd, true); err == nil {
		t.Error("expected an error for a [local]-only profile")
	}

	cmd = newRunCommand()
	if err := cmd.Flags().Set("profile", "ssh"); err != nil {
		t.Fatal(err)
	}
	got, err := resolveRunProfile(cmd, true)
	if err != nil {
		t.Fatalf("resolveRunProfile: %v", err)
	}
	if got != "ssh" {
		t.Errorf("got %q, want ssh", got)
	}
}

func TestShellInteractiveDefaults(t *testing.T) {
	cases := []struct {
		name             string
		user, shellType  string
		userSet, typeSet bool
		hasCommand       bool
		wantUser, wantSh string
	}{
		{"interactive applies devuser+zsh", "", "", false, false, false, "devuser", "zsh"},
		{"interactive respects explicit flags", "root", "bash", true, true, false, "root", "bash"},
		{"explicit command leaves flags bare", "", "", false, false, true, "", ""},
		{"explicit command keeps given user", "root", "", true, false, true, "root", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotUser, gotSh := shellInteractiveDefaults(c.user, c.shellType, c.userSet, c.typeSet, c.hasCommand)
			if gotUser != c.wantUser || gotSh != c.wantSh {
				t.Errorf("shellInteractiveDefaults = (%q, %q); want (%q, %q)", gotUser, gotSh, c.wantUser, c.wantSh)
			}
		})
	}
}

func TestCleanVolumesCommand_HasSharedFlag(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean" {
			clean = c
		}
	}
	if clean == nil {
		t.Fatal("clean command not found")
	}
	var vol *cobra.Command
	for _, c := range clean.Commands() {
		if c.Name() == "volumes" {
			vol = c
		}
	}
	if vol == nil {
		t.Fatal("clean volumes subcommand not found")
	}
	if vol.Flags().Lookup("shared") == nil {
		t.Error("expected clean volumes --shared flag")
	}
}

// sharedConfigSubcommand resolves `config shared <name>`, failing the test if any
// level of the group is missing.
func sharedConfigSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("config command not found")
	}
	sharedCmd := findSubcommand(configCmd, "shared")
	if sharedCmd == nil {
		t.Fatal("'config shared' command not found")
	}
	sub := findSubcommand(sharedCmd, name)
	if sub == nil {
		t.Fatalf("'config shared %s' command not found", name)
	}
	return sub
}

func TestSyncConfigCommand_FlagsAndArgs(t *testing.T) {
	sync := sharedConfigSubcommand(t, "sync")
	for _, name := range []string{"force", "yes", "no-interactive"} {
		if sync.Flags().Lookup(name) == nil {
			t.Errorf("expected config shared sync flag --%s", name)
		}
	}

	if _, err := resolveSharedConfigArgs(nil); err != nil {
		t.Errorf("no args should resolve to all entries: %v", err)
	}
	entries, err := resolveSharedConfigArgs([]string{"claude", "gh"})
	if err != nil || len(entries) != 2 {
		t.Errorf("expected claude+gh to resolve, got %v / %v", entries, err)
	}
	if _, err := resolveSharedConfigArgs([]string{"bogus"}); err == nil {
		t.Error("expected error for unknown tool id")
	}
}

func TestBackupConfigCommand_FlagsAndArgs(t *testing.T) {
	backup := sharedConfigSubcommand(t, "backup")
	if backup.Flags().Lookup("output") == nil {
		t.Error("expected config shared backup flag --output")
	}
}

func TestRestoreConfigCommand_FlagsAndArgs(t *testing.T) {
	restore := sharedConfigSubcommand(t, "restore")
	for _, name := range []string{"force", "yes", "no-interactive"} {
		if restore.Flags().Lookup(name) == nil {
			t.Errorf("expected config shared restore flag --%s", name)
		}
	}
	if restore.Args == nil {
		t.Fatal("expected config shared restore to require at least the zip-file argument")
	}
	if err := restore.Args(restore, nil); err == nil {
		t.Error("expected config shared restore to reject invocation with no arguments")
	}
}

func TestConfigSharedCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	sharedCmd := findSubcommand(configCmd, "shared")
	if sharedCmd == nil {
		t.Fatal("expected 'config shared' command to be registered")
	}
	for _, name := range []string{"sync", "backup", "restore"} {
		if findSubcommand(sharedCmd, name) == nil {
			t.Errorf("expected 'config shared %s' subcommand", name)
		}
	}
}

func TestConfigAliasCommand_Structure(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	aliasCmd := findSubcommand(configCmd, "alias")
	if aliasCmd == nil {
		t.Fatal("expected 'config alias' command to be registered")
	}
	// Bare `config alias` lists the aliases; it takes no positional args.
	if err := aliasCmd.Args(aliasCmd, []string{"extra"}); err == nil {
		t.Error("expected 'config alias' to reject positional arguments")
	}
	for _, name := range []string{"set", "unset", "sync"} {
		if findSubcommand(aliasCmd, name) == nil {
			t.Errorf("expected 'config alias %s' subcommand", name)
		}
	}
	// set takes a name plus a command; unset takes exactly the name.
	if err := findSubcommand(aliasCmd, "set").Args(nil, []string{"only-name"}); err == nil {
		t.Error("expected 'config alias set' to require a name and a command")
	}
	if err := findSubcommand(aliasCmd, "unset").Args(nil, []string{"a", "b"}); err == nil {
		t.Error("expected 'config alias unset' to accept exactly one argument")
	}
	// It is not a top-level command.
	if findSubcommand(root, "alias") != nil {
		t.Error("expected no top-level 'alias' command; it lives under 'config'")
	}
}

// Listing and completion share sortedAliasNames so they can never disagree
// about the order Go's randomized map iteration would otherwise give them.
func TestSortedAliasNames(t *testing.T) {
	got := sortedAliasNames(map[string]string{"gs": "git status", "ll": "ls -la", "k": "kubectl", "_x": "echo"})
	want := []string{"_x", "gs", "k", "ll"}
	if !slices.Equal(got, want) {
		t.Errorf("sortedAliasNames = %v, want %v", got, want)
	}
	if got := sortedAliasNames(nil); len(got) != 0 {
		t.Errorf("sortedAliasNames(nil) = %v, want empty", got)
	}
}

func TestConfigCommand_HasSSHConfigFileSubcommand(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	sub := findSubcommand(configCmd, "ssh-config-file")
	if sub == nil {
		t.Fatal("expected 'config ssh-config-file' command to be registered")
	}
	if sub.Flags().Lookup("unset") == nil {
		t.Error("expected config ssh-config-file flag --unset")
	}
}

func TestContextCommand_Flags(t *testing.T) {
	root := NewRootCommand("test")
	ctx := findSubcommand(root, "context")
	if ctx == nil {
		t.Fatal("expected 'context' command to be registered")
	}
	for _, name := range []string{"container", "json"} {
		if ctx.Flags().Lookup(name) == nil {
			t.Errorf("expected context flag --%s", name)
		}
	}
	if err := ctx.Args(ctx, []string{"extra"}); err == nil {
		t.Error("expected 'context' to reject positional arguments")
	}
}

func TestSharedConfigCommands_NotTopLevel(t *testing.T) {
	root := NewRootCommand("test")
	for _, name := range []string{"sync-config", "backup-config", "restore-config"} {
		if findSubcommand(root, name) != nil {
			t.Errorf("expected no top-level %q command; it now lives under 'config shared'", name)
		}
	}
}

func TestConfigCommand_HasRegistrySubcommand(t *testing.T) {
	root := NewRootCommand("test")
	var config *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "config" {
			config = c
			break
		}
	}
	if config == nil {
		t.Fatal("config command not found")
	}
	found := false
	for _, sub := range config.Commands() {
		if sub.Name() == "registry" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'config registry' subcommand")
	}
}

func TestConfigCommand_HasDefaultsSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	var config *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "config" {
			config = c
			break
		}
	}
	if config == nil {
		t.Fatal("config command not found")
	}
	wants := []string{"db-user", "db-password", "ssh-key"}
	have := map[string]bool{}
	for _, sub := range config.Commands() {
		have[sub.Name()] = true
	}
	for _, name := range wants {
		if !have[name] {
			t.Errorf("expected config subcommand %q to be registered", name)
		}
	}
}

// config ssh-key is a bespoke command exposing --unset, --generate, --public
// and --private beyond the plain get/set verbs.
func TestConfigSSHKeyCommand_Flags(t *testing.T) {
	root := NewRootCommand("test")
	var sshKey *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() != "config" {
			continue
		}
		for _, sub := range c.Commands() {
			if sub.Name() == "ssh-key" {
				sshKey = sub
			}
		}
	}
	if sshKey == nil {
		t.Fatal("config ssh-key subcommand not found")
	}
	for _, flag := range []string{"unset", "generate", "public", "private"} {
		if sshKey.Flags().Lookup(flag) == nil {
			t.Errorf("expected config ssh-key to define --%s", flag)
		}
	}
}

func TestParsePortMapping(t *testing.T) {
	cases := []struct {
		in        string
		service   string
		local     int
		host      string
		container int
		wantErr   bool
	}{
		{in: "3000", local: 3000, host: "localhost", container: 3000},
		{in: "8080:80", local: 8080, host: "localhost", container: 80},
		{in: "5432:postgres:5432", local: 5432, host: "postgres", container: 5432},
		{in: "5432", service: "postgres", local: 5432, host: "postgres", container: 5432},
		{in: "0", wantErr: true},
		{in: "70000", wantErr: true},
		{in: "a:b:c:d", wantErr: true},
	}
	for _, c := range cases {
		got, err := parsePortMapping(c.in, c.service)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePortMapping(%q,%q): expected error", c.in, c.service)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePortMapping(%q,%q): unexpected error %v", c.in, c.service, err)
			continue
		}
		if got.localPort != c.local || got.targetHost != c.host || got.containerPort != c.container {
			t.Errorf("parsePortMapping(%q,%q) = %+v, want local=%d host=%s container=%d", c.in, c.service, got, c.local, c.host, c.container)
		}
	}
}

func TestParsePortMapping_ConflictingService(t *testing.T) {
	if _, err := parsePortMapping("5432:postgres:5432", "redis"); err == nil {
		t.Error("expected conflicting target host error")
	}
}

func TestBuildSshTunnels(t *testing.T) {
	tunnels, err := buildSshTunnels("3000, 8080:80", "myws", "myws-devcontainer-ssh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(tunnels))
	}
	want := tunnels[0]
	if want.LocalPort != 3000 || want.ContainerPort != 3000 || want.TargetHost != "localhost" ||
		want.Alias != "myws" || want.ContainerName != "myws-devcontainer-ssh" || !want.IsDevcontainer {
		t.Errorf("tunnel 0 = %+v", want)
	}
	if tunnels[1].LocalPort != 8080 || tunnels[1].ContainerPort != 80 {
		t.Errorf("tunnel 1 = %+v", tunnels[1])
	}
	if _, err := buildSshTunnels("not-a-port", "myws", "c"); err == nil {
		t.Error("expected error for an invalid ports spec")
	}
}

func TestSshCommandRegistered(t *testing.T) {
	root := NewRootCommand("test")
	sshCmd := findSubcommand(root, "ssh")
	if sshCmd == nil {
		t.Fatal("expected 'ssh' command to be registered")
	}
	for _, name := range []string{"yes", "no-interactive", "forward", "ports", "container", "via", "setup", "setup-external", "key", "user"} {
		if sshCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected 'ssh' to register --%s", name)
		}
	}
}

// 'ssh --via' has no local compose project to target a remote-only container
// with, so it must be paired with --container.
func TestRunSsh_ViaRequiresContainer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"--via", "me@docker-host"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := runSsh(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--container") {
		t.Errorf("runSsh error = %v, want an error requiring --container", err)
	}
}

func TestDestroyCommandRegistered(t *testing.T) {
	root := NewRootCommand("test")
	destroyCmd := findSubcommand(root, "destroy")
	if destroyCmd == nil {
		t.Fatal("expected 'destroy' command to be registered")
	}
	for _, name := range []string{"yes", "no-interactive", "container"} {
		if destroyCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected 'destroy' to register --%s", name)
		}
	}
}

func TestParsePortsList(t *testing.T) {
	pairs, err := parsePortsList("3000, 8080:80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].localPort != 3000 || pairs[0].containerPort != 3000 {
		t.Errorf("pair 0 = %+v", pairs[0])
	}
	if pairs[1].localPort != 8080 || pairs[1].containerPort != 80 {
		t.Errorf("pair 1 = %+v", pairs[1])
	}
	if _, err := parsePortsList(""); err == nil {
		t.Error("expected error for empty ports list")
	}
}

func TestParseSSHConfigContent(t *testing.T) {
	content := `Host alpha beta
    HostName 1.2.3.4
# comment
Host gamma
    User x
Host *.wildcard
`
	hosts := parseSSHConfigContent(content)
	want := []string{"alpha", "beta", "gamma"}
	if len(hosts) != len(want) {
		t.Fatalf("got %v, want %v", hosts, want)
	}
	for i, h := range want {
		if hosts[i] != h {
			t.Errorf("host[%d] = %q, want %q", i, hosts[i], h)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("nodejs, golang ,, zellij")
	want := []string{"nodejs", "golang", "zellij"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestCompleteCSV(t *testing.T) {
	allModules := []string{"nodejs", "python", "golang", "zellij", "bun"}
	cases := []struct {
		toComplete string
		want       []string
	}{
		{toComplete: "no", want: []string{"nodejs"}},
		{toComplete: "nodejs,go", want: []string{"nodejs,golang"}},
		{toComplete: "nodejs, python, g", want: []string{"nodejs, python,golang"}},
		{toComplete: "nodejs,python,zellij,bun,c", want: []string{}},
	}
	for _, c := range cases {
		got := completeCSV(c.toComplete, allModules)
		if len(got) != len(c.want) {
			t.Errorf("completeCSV(%q) = %v, want %v", c.toComplete, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("completeCSV(%q)[%d] = %q, want %q", c.toComplete, i, got[i], c.want[i])
			}
		}
	}
}

func TestListSshHosts_ReadsConfig(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer func() {
		os.Setenv("HOME", origHome)
	}()

	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sshDir, "config")
	configContent := `
Host web-server db-server
    HostName 10.0.0.1
Host cache-server
    HostName 10.0.0.2
Host *.wildcard
    HostName 10.0.0.3
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	hosts := listSshHosts()
	want := []string{"web-server", "db-server", "cache-server"}
	if len(hosts) != len(want) {
		t.Fatalf("listSshHosts() returned %v, want %v", hosts, want)
	}
	for i, h := range want {
		if hosts[i] != h {
			t.Errorf("hosts[%d] = %q, want %q", i, hosts[i], h)
		}
	}
}

// The remote config block builds purely from the alias, remote host and
// container — with an empty installResult — proving no key generation or
// installation needs to have happened first.
func TestBuildConfigBlock_RemoteNeedsNoKeyInstall(t *testing.T) {
	f := &setupSshFlags{
		alias:     "myws",
		user:      "devuser",
		key:       "~/.ssh/id_myws",
		remote:    "user@host",
		container: "myws-devcontainer-ssh",
	}
	block, err := buildConfigBlock(sshdefaults.ModeRemote, f, installResult{}, sshdefaults.KindWorkspace, "myws")
	if err != nil {
		t.Fatalf("buildConfigBlock(remote): %v", err)
	}
	for _, want := range []string{"# devcontainer-cli:managed v=1 kind=workspace ref=myws alias=myws", "Host myws", "ProxyCommand ssh user@host", "myws-devcontainer-ssh"} {
		if !strings.Contains(block, want) {
			t.Errorf("remote block missing %q:\n%s", want, block)
		}
	}
}

// emitRemoteConfig hands over the shared managed key: it renders the export
// snippet (embedding the private+public key bytes) plus a config block whose
// IdentityFile uses the ~/.ssh form, never this host's absolute home path.
func TestEmitRemoteConfig_ExportsSharedKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_devcontainer")
	if err := os.WriteFile(keyPath, []byte("PRIV-KEY"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 PUB"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	f := &setupSshFlags{
		alias:     "myws",
		user:      "devuser",
		key:       keyPath,
		remote:    "user@host",
		container: "myws-devcontainer-ssh",
	}
	ssh := service.SshService{Report: console}

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	emitErr := emitRemoteConfig(ssh, f, sshdefaults.KindWorkspace, "myws")
	_ = w.Close()
	os.Stdout = orig
	if emitErr != nil {
		t.Fatalf("emitRemoteConfig: %v", emitErr)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "IdentityFile ~/.ssh/"+sshdefaults.KeyName) {
		t.Errorf("remote block should render IdentityFile with ~/.ssh, got:\n%s", got)
	}
	for _, frag := range []string{"PRIV-KEY", "ssh-ed25519 PUB", "ProxyCommand ssh user@host"} {
		if !strings.Contains(got, frag) {
			t.Errorf("export output missing %q:\n%s", frag, got)
		}
	}
	// The container already has the shared key in authorized_keys from the host
	// run, so remote output must not include a container-install one-liner.
	if strings.Contains(got, "docker exec") || strings.Contains(got, "authorized_keys") {
		t.Errorf("remote output must not install into the container, got:\n%s", got)
	}
	if strings.Contains(got, dir) {
		t.Errorf("remote output must not leak this host's absolute key path, got:\n%s", got)
	}
	// f itself must stay untouched — the tilde rewrite is local to the render.
	if f.key != keyPath {
		t.Errorf("emitRemoteConfig mutated f.key to %q", f.key)
	}
}

// -c is the shorthand for --container on 'ssh', matching every other command
// that targets a container.
func TestSshCommand_ContainerShorthand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"-c", "my-container"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		t.Fatalf("collectSetupSshFlags: %v", err)
	}
	if f.container != "my-container" {
		t.Errorf("container = %q, want %q", f.container, "my-container")
	}
	if !f.containerExplicit {
		t.Error("containerExplicit = false, want true when -c is passed")
	}
}

// --via is a separate flag from --setup-external, both string-valued and unset
// by default; collectSetupSshFlags must read it into f.via without touching
// f.remote (which only --setup-external fills).
func TestCollectSetupSshFlags_Via(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"--via", "me@docker-host", "-c", "dc-ssh"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		t.Fatalf("collectSetupSshFlags: %v", err)
	}
	if f.via != "me@docker-host" {
		t.Errorf("via = %q, want %q", f.via, "me@docker-host")
	}
	if f.remote != "" {
		t.Errorf("remote = %q, want empty", f.remote)
	}
}

// --setup-external maps onto f.remote (the paste-a-private-key jump mode).
func TestCollectSetupSshFlags_SetupExternal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"--setup-external", "me@docker-host"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		t.Fatalf("collectSetupSshFlags: %v", err)
	}
	if f.remote != "me@docker-host" {
		t.Errorf("remote = %q, want %q", f.remote, "me@docker-host")
	}
}

// --setup-external and --via are two different ways of routing setup through a
// second host (paste-a-private-key vs. client-driven); combining them is
// ambiguous and must be rejected before any docker/ssh work starts.
func TestRunSshSetup_ExternalAndViaMutuallyExclusive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"--setup-external", "me@host", "--via", "me@host", "-c", "dc-ssh"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	_, err := runSshSetup(cmd)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("runSshSetup error = %v, want a mutually-exclusive error", err)
	}
}

// --via has no local compose project to fall back on (the container lives on
// a different host), so it must be paired with an explicit --container.
func TestRunSshSetup_ViaRequiresContainer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse([]string{"--via", "me@docker-host"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	_, err := runSshSetup(cmd)
	if err == nil || !strings.Contains(err.Error(), "--container") {
		t.Errorf("runSshSetup error = %v, want an error requiring --container", err)
	}
}

// With no --key, setup defaults to the shared managed key under the CLI
// config dir, not a per-host ~/.ssh path.
func TestCollectSetupSshFlags_DefaultsToManagedKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		t.Fatalf("collectSetupSshFlags: %v", err)
	}
	if f.key != domain.DefaultManagedSSHKeyPath() {
		t.Errorf("default key = %q, want managed default %q", f.key, domain.DefaultManagedSSHKeyPath())
	}
}

// The shared key is reused by every workspace: applyWorkspaceDefaults rewrites
// the alias/container but must NOT rewrite the key per workspace.
func TestApplyWorkspaceDefaults_KeepsSharedKey(t *testing.T) {
	managed := domain.DefaultManagedSSHKeyPath()
	f := &setupSshFlags{
		alias:     sshdefaults.Alias,
		key:       managed,
		container: sshdefaults.ServiceName,
	}
	applyWorkspaceDefaults(f, "myws")
	if f.key != managed {
		t.Errorf("applyWorkspaceDefaults rewrote f.key to %q; the shared key must be left untouched", f.key)
	}
	if f.alias != "myws" {
		t.Errorf("expected alias defaulted to workspace, got %q", f.alias)
	}
}

func TestListComposeServices_ReadsCompose(t *testing.T) {
	tempDir := t.TempDir()
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	composeContent := `
services:
  web:
    image: nginx
  db:
    image: postgres
  redis:
    image: redis
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0600); err != nil {
		t.Fatal(err)
	}

	services := listComposeServices(composePath)
	want := map[string]bool{"web": true, "db": true, "redis": true}
	if len(services) != len(want) {
		t.Fatalf("listComposeServices() returned %v, expected %d services", services, len(want))
	}
	for _, s := range services {
		if !want[s] {
			t.Errorf("unexpected service in completion suggestions: %s", s)
		}
	}
}

func TestShellCommand_HasFlags(t *testing.T) {
	root := NewRootCommand("test")
	var shell *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "shell" {
			shell = c
			break
		}
	}
	if shell == nil {
		t.Fatal("shell command not found")
	}
	for _, name := range []string{"user", "no-tty", "via"} {
		if shell.Flags().Lookup(name) == nil {
			t.Errorf("expected shell flag --%s", name)
		}
	}
	if shell.Flags().ShorthandLookup("T") == nil {
		t.Error("expected shell flag shorthand -T for --no-tty")
	}
}

// shell rides entirely on 'docker exec', so --via has no local compose project
// to target a remote-only container with and must be paired with --container,
// same rule as ssh/setup-ssh.
func TestRunShell_ViaRequiresContainer(t *testing.T) {
	cmd := newShellCommand()
	if err := cmd.Flags().Parse([]string{"--via", "me@docker-host"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := runShell(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--container") {
		t.Errorf("runShell error = %v, want an error requiring --container", err)
	}
}

func TestLogsCommand_HasFlags(t *testing.T) {
	root := NewRootCommand("test")
	var logs *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "logs" {
			logs = c
			break
		}
	}
	if logs == nil {
		t.Fatal("logs command not found")
	}
	for _, name := range []string{"follow", "tail"} {
		if logs.Flags().Lookup(name) == nil {
			t.Errorf("expected logs flag --%s", name)
		}
	}
}

func TestCopyCommand_AssetFlag(t *testing.T) {
	root := NewRootCommand("test")
	var copyCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "copy" {
			copyCmd = c
			break
		}
	}
	if copyCmd == nil {
		t.Fatal("copy command not found")
	}
	if copyCmd.Flags().Lookup("asset") == nil {
		t.Fatal("expected copy flag --asset")
	}

	// Completion lists the copyable asset names.
	if len(assets.CopyableNames()) == 0 {
		t.Fatal("expected copyable asset names for completion")
	}

	// Args: without --asset, exactly two positional args are required.
	if err := copyCmd.Args(copyCmd, []string{"a", "b"}); err != nil {
		t.Errorf("two args should be valid in local mode: %v", err)
	}
	if err := copyCmd.Args(copyCmd, []string{"a"}); err == nil {
		t.Error("single arg should be invalid in local mode")
	}

	// With --asset set, zero or one positional arg is allowed.
	if err := copyCmd.Flags().Set("asset", "install-claude-code"); err != nil {
		t.Fatal(err)
	}
	if err := copyCmd.Args(copyCmd, nil); err != nil {
		t.Errorf("zero args should be valid with --asset: %v", err)
	}
	if err := copyCmd.Args(copyCmd, []string{"/tmp/x"}); err != nil {
		t.Errorf("one dest arg should be valid with --asset: %v", err)
	}
	if err := copyCmd.Args(copyCmd, []string{"a", "b"}); err == nil {
		t.Error("two args should be invalid with --asset")
	}
}

func TestCopyDirection(t *testing.T) {
	cases := []struct {
		name          string
		src, dst      string
		wantFrom      bool
		wantSrc, want string
		wantErr       bool
	}{
		{"host to container default", "./a", "/dest", false, "./a", "/dest", false},
		{"host to container explicit", "./a", ":/dest", false, "./a", "/dest", false},
		{"container to host", ":/src", "./out", true, "/src", "./out", false},
		{"container to container", ":/src", ":/dest", false, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, src, dst, err := copyDirection(c.src, c.dst)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if from != c.wantFrom || src != c.wantSrc || dst != c.want {
				t.Errorf("got (from=%v, src=%q, dst=%q), want (from=%v, src=%q, dst=%q)",
					from, src, dst, c.wantFrom, c.wantSrc, c.want)
			}
		})
	}
}

func TestResolveProjectComposeFile(t *testing.T) {
	tempDir := t.TempDir()
	// With no config, the workspace is derived from the sanitized directory name,
	// so the compose file lives under .dc_<dir>/build/.
	workspace := resolveWorkspace(tempDir)
	wsDir := filepath.Join(tempDir, ".dc_"+workspace, "build")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	compFile := filepath.Join(wsDir, "docker-compose.yml")
	if err := os.WriteFile(compFile, []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := resolveProjectComposeFile(tempDir)
	if err != nil {
		t.Fatalf("unexpected error resolving compose file: %v", err)
	}
	if res != compFile {
		t.Errorf("got %q, want %q", res, compFile)
	}
}

func TestCopyCommand_HasFlags(t *testing.T) {
	root := NewRootCommand("test")
	var copyCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "copy" {
			copyCmd = c
			break
		}
	}
	if copyCmd == nil {
		t.Fatal("copy command not found")
	}
	for _, name := range []string{"container", "asset"} {
		if copyCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected copy flag --%s", name)
		}
	}
}

func TestUpCommand_HasFlags(t *testing.T) {
	root := NewRootCommand("test")
	var upCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "up" {
			upCmd = c
			break
		}
	}
	if upCmd == nil {
		t.Fatal("up command not found")
	}
	for _, name := range []string{"build"} {
		if upCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected up flag --%s", name)
		}
	}
}

func TestStatusCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	var statusCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "status" {
			statusCmd = c
			break
		}
	}
	if statusCmd == nil {
		t.Fatal("status command not found")
	}
}

func TestLsCommand_HasFlags(t *testing.T) {
	root := NewRootCommand("test")
	var lsCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "ls" {
			lsCmd = c
			break
		}
	}
	if lsCmd == nil {
		t.Fatal("ls command not found")
	}
	for _, name := range []string{"all", "long"} {
		if lsCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected ls flag --%s", name)
		}
	}
}

func TestAllCommands_HaveContainerFlag(t *testing.T) {
	root := NewRootCommand("test")
	cmds := []string{"copy", "update", "ls", "logs", "status", "shell", "start", "stop", "restart", "info"}
	for _, name := range cmds {
		var target *cobra.Command
		for _, c := range root.Commands() {
			if c.Name() == name {
				target = c
				break
			}
		}
		if target == nil {
			t.Fatalf("command %q not found", name)
		}
		if target.Flags().Lookup("container") == nil {
			t.Errorf("expected command %q to have --container flag", name)
		}
	}
}

func TestOperationalCommands_HaveNoWorkspaceFlag(t *testing.T) {
	root := NewRootCommand("test")
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		byName[c.Name()] = c
	}
	// Operational commands act only on the current directory's project; none of
	// them expose a --workspace flag to target another one.
	for _, name := range []string{"up", "down", "start", "stop", "restart", "shell", "ssh", "copy", "info", "status", "logs", "ls"} {
		cmd := byName[name]
		if cmd == nil {
			t.Fatalf("command %q not found", name)
		}
		if cmd.Flags().Lookup("workspace") != nil {
			t.Errorf("command %q must not have a --workspace flag", name)
		}
		// -w is reserved so it can never read as a workspace selector. 'shell'
		// is the one holder, where it is --workdir: it changes the directory
		// *inside* the current project's container, which is the opposite of
		// targeting another project.
		if sh := cmd.Flags().ShorthandLookup("w"); sh != nil && sh.Name != "workdir" {
			t.Errorf("command %q binds -w to %q; -w must only ever be --workdir", name, sh.Name)
		}
	}

	// And pin that the one -w in the tree really is the workdir flag, so it
	// cannot quietly turn into a workspace selector later.
	shell := byName["shell"]
	sh := shell.Flags().ShorthandLookup("w")
	if sh == nil || sh.Name != "workdir" {
		t.Errorf("shell -w should be --workdir, got %v", sh)
	}
}

func TestFilterContainerNames_ByState(t *testing.T) {
	containers := []pick.Container{
		{Name: "ws-devcontainer-ssh", State: "running"},
		{Name: "ws-postgres", State: "exited"},
		{Name: "other", State: "created"},
	}

	all := filterContainerNames(containers, "", nil)
	if len(all) != 3 {
		t.Fatalf("nil keep: got %d names, want 3: %v", len(all), all)
	}

	running := filterContainerNames(containers, "", containerRunning)
	if len(running) != 1 || running[0] != "ws-devcontainer-ssh" {
		t.Errorf("running filter: got %v, want [ws-devcontainer-ssh]", running)
	}

	stopped := filterContainerNames(containers, "", func(c pick.Container) bool { return !containerRunning(c) })
	if len(stopped) != 2 {
		t.Errorf("stopped filter: got %v, want 2", stopped)
	}

	prefixed := filterContainerNames(containers, "ws-", nil)
	if len(prefixed) != 2 {
		t.Errorf("prefix filter: got %v, want 2 ws- names", prefixed)
	}
}

func TestStartStopCompletion_FilterByState(t *testing.T) {
	root := NewRootCommand("test")
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		byName[c.Name()] = c
	}
	// start completes stopped containers, stop completes running ones; both
	// register a flag completion func for --container.
	for _, name := range []string{"start", "stop"} {
		cmd := byName[name]
		if cmd == nil {
			t.Fatalf("command %q not found", name)
		}
		if _, ok := cmd.GetFlagCompletionFunc("container"); !ok {
			t.Errorf("expected %q to register --container completion", name)
		}
	}
}

func TestContainerChoiceLabel_ContainsFields(t *testing.T) {
	c := pick.Container{Name: "ws-devcontainer-ssh", Image: "img:latest", State: "running", Status: "Up", Workspace: "ws"}
	label := containerChoiceLabel(c)
	for _, want := range []string{"ws-devcontainer-ssh", "ws", "img:latest"} {
		if !strings.Contains(label, want) {
			t.Errorf("label %q missing %q", label, want)
		}
	}

	// Unmanaged container (no workspace) still renders a non-empty tag.
	if tag := workspaceTag(pick.Container{}); tag == "" {
		t.Error("workspaceTag for empty workspace should not be empty")
	}
}

func TestConfigExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	origCfg := &types.DevcontainerConfig{
		Mode:      types.BuildModeCustom,
		Image:     "devcontainer-cli/test:latest",
		Workspace: "my-test-workspace",
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{
				{ID: "golang", Options: map[string]any{}},
			},
		},
		Compose: types.ComposeConfig{
			Services: []types.SelectedService{{ID: "postgres"}},
			Subnet:   "172.28.0.0/24",
		},
		Env: map[string]string{"FOO": "BAR"},
	}

	if err := domain.SaveConfig(origCfg, tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Make sure we can load and serialize
	loaded, err := domain.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	data, err := yaml.Marshal(loaded)
	if err != nil {
		t.Fatalf("failed to marshal to yaml: %v", err)
	}

	var imported types.DevcontainerConfig
	if err := yaml.Unmarshal(data, &imported); err != nil {
		t.Fatalf("failed to unmarshal from yaml: %v", err)
	}

	if imported.Workspace != origCfg.Workspace {
		t.Errorf("imported.Workspace = %q, want %q", imported.Workspace, origCfg.Workspace)
	}
	if imported.Image != origCfg.Image {
		t.Errorf("imported.Image = %q, want %q", imported.Image, origCfg.Image)
	}
	if imported.Compose.Subnet != origCfg.Compose.Subnet {
		t.Errorf("imported.Compose.Subnet = %q, want %q", imported.Compose.Subnet, origCfg.Compose.Subnet)
	}
}

// findSubcommand returns the immediate child of parent with the given name.
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func TestConfigProfileCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	profileCmd := findSubcommand(configCmd, "profile")
	if profileCmd == nil {
		t.Fatal("expected 'config profile' command to be registered")
	}
	if findSubcommand(profileCmd, "list") == nil {
		t.Error("expected 'config profile list' subcommand")
	}
}

func TestConfigSkillCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	skillCmd := findSubcommand(configCmd, "skill")
	if skillCmd == nil {
		t.Fatal("expected 'config skill' command to be registered")
	}
	if findSubcommand(skillCmd, "list") == nil {
		t.Error("expected 'config skill list' subcommand")
	}
	if findSubcommand(skillCmd, "info") == nil {
		t.Error("expected 'config skill info' subcommand")
	}
	if findSubcommand(skillCmd, "add") == nil {
		t.Error("expected 'config skill add' subcommand")
	}
	if findSubcommand(skillCmd, "remove") == nil {
		t.Error("expected 'config skill remove' subcommand")
	}
}

func TestConfigProfileInfoCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	profileCmd := findSubcommand(configCmd, "profile")
	if profileCmd == nil {
		t.Fatal("expected 'config profile' command to be registered")
	}
	if findSubcommand(profileCmd, "info") == nil {
		t.Error("expected 'config profile info' subcommand")
	}
}

func TestRunProfileInfo_UnknownProfileFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := &cobra.Command{}
	if err := runProfileInfo(cmd, []string{"no-such-profile"}); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}

func TestRunProfileInfo_KnownProfileSucceeds(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := &cobra.Command{}
	if err := runProfileInfo(cmd, []string{"nodejs"}); err != nil {
		t.Fatalf("runProfileInfo: %v", err)
	}
}

func TestRunConfigSkillInfo_UnknownSkillFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := &cobra.Command{}
	if err := runConfigSkillInfo(cmd, []string{"no-such-skill"}); err == nil {
		t.Fatal("expected an error for an unknown skill")
	}
}

func TestRunConfigSkillInfo_KnownSkillSucceeds(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := &cobra.Command{}
	if err := runConfigSkillInfo(cmd, []string{"firecrawl"}); err != nil {
		t.Fatalf("runConfigSkillInfo: %v", err)
	}
}

// A user-defined skill must resolve through 'config skill info' the same way
// a built-in does.
func TestRunConfigSkillInfo_UserDefinedSkill(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := domain.SkillDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: my-skill\nlabel: My Skill\nref: me/my-skill\ncontext:\n  title: My Skill\n  body: Teaches X.\n"
	if err := os.WriteFile(filepath.Join(dir, "my-skill.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	if err := runConfigSkillInfo(cmd, []string{"my-skill"}); err != nil {
		t.Fatalf("runConfigSkillInfo: %v", err)
	}
}

// 'preset' was the old name of the group and stays as an alias, so an existing
// script or muscle-memory invocation keeps working.
func TestConfigProfileCommand_PresetAlias(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	aliased, _, err := configCmd.Find([]string{"preset", "list"})
	if err != nil {
		t.Fatalf("expected 'config preset list' to still resolve: %v", err)
	}
	if aliased.Name() != "list" || aliased.Parent().Name() != "profile" {
		t.Errorf("expected 'config preset' to alias 'config profile', got %s under %s", aliased.Name(), aliased.Parent().Name())
	}
}

func TestProfileCommand_NotTopLevel(t *testing.T) {
	root := NewRootCommand("test")
	if findSubcommand(root, "profile") != nil {
		t.Error("expected no top-level 'profile' command; it lives under 'config profile'")
	}
}

func TestVerboseFlagSetsDebug(t *testing.T) {
	orig := logger.Std().GetLevel()
	defer logger.SetLevel(orig)

	root := NewRootCommand("test")
	root.SetArgs([]string{"--verbose", "help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error executing root: %v", err)
	}

	if logger.Std().GetLevel() != log.DebugLevel {
		t.Errorf("expected log level to be debug, got %v", logger.Std().GetLevel())
	}
}

func TestLogLevelFlagInvalid(t *testing.T) {
	root := NewRootCommand("test")
	root.SetArgs([]string{"--log-level", "invalid-level-name", "help"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error with invalid log level, got nil")
	}
}

func TestMaybeUpdateGitignore_NoGitRepo(t *testing.T) {
	dir := t.TempDir() // no .git dir
	if err := maybeUpdateGitignore(dir, "test"); err != nil {
		t.Fatalf("expected no error for non-git dir, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error("expected .gitignore not to be created for non-git dir")
	}
}

func TestMaybeUpdateGitignore_AlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := ".dc_*/\ndevcontainer.config.json\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := maybeUpdateGitignore(dir, "test"); err != nil {
		t.Fatalf("expected no error when entries already present, got: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if string(data) != existing {
		t.Errorf(".gitignore should be unchanged; got: %q", string(data))
	}
}

func TestApplyGenFlags_ProfileWithoutServices(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{
				{ID: "golang"},
			},
		},
		Compose: types.ComposeConfig{
			Services: []types.SelectedService{{ID: "postgres"}},
		},
	}
	flags := &genFlags{
		profile: "nodejs",
	}

	applyGenFlags(config, flags)

	// The nodejs profile is github-cli + nodejs + pnpm (zellij is always-on and
	// no longer listed in profiles).
	if len(config.Dockerfile.Modules) != 3 {
		t.Errorf("expected 3 modules, got %d", len(config.Dockerfile.Modules))
	}
	if len(config.Compose.Services) != 0 {
		t.Errorf("expected 0 services after applying service-less profile, got %d: %v", len(config.Compose.Services), config.Compose.Services)
	}
}

// Under mode=profiles, --profile is the pull target (config.Remote), not a
// module bundle — applying it must not clear the project's existing DB
// services the way it does under mode=custom.
func TestApplyGenFlags_ProfileUnderModeProfilesKeepsServicesAndSetsRemote(t *testing.T) {
	config := &types.DevcontainerConfig{
		Mode: types.BuildModeProfiles,
		Compose: types.ComposeConfig{
			Services: []types.SelectedService{{ID: "postgres"}},
		},
	}
	flags := &genFlags{profile: "nodejs"}

	applyGenFlags(config, flags)

	if len(config.Compose.Services) != 1 || config.Compose.Services[0].ID != "postgres" {
		t.Errorf("expected postgres service to survive, got %v", config.Compose.Services)
	}
	if config.Remote == nil || config.Remote.Variant != "nodejs" {
		t.Errorf("expected Remote.Variant = nodejs, got %+v", config.Remote)
	}
}

// "ssh" is a pull target with no catalog entry, so under mode=custom it
// resolves to nothing. Applying it must leave the project alone rather than
// clear its DB services for a bundle that contributed no modules either;
// validateConfig is what rejects the combination outright.
func TestApplyGenFlags_SSHProfileUnderCustomModeKeepsServices(t *testing.T) {
	config := &types.DevcontainerConfig{
		Mode: types.BuildModeCustom,
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{{ID: "nodejs"}},
		},
		Compose: types.ComposeConfig{
			Services: []types.SelectedService{{ID: "postgres"}},
		},
	}

	applyGenFlags(config, &genFlags{profile: remoteVariantSSH})

	if len(config.Compose.Services) != 1 || config.Compose.Services[0].ID != "postgres" {
		t.Errorf("expected postgres service to survive, got %v", config.Compose.Services)
	}
	if len(config.Dockerfile.Modules) != 1 || config.Dockerfile.Modules[0].ID != "nodejs" {
		t.Errorf("expected the modules to survive, got %v", config.Dockerfile.Modules)
	}
}

// The same, but switching an existing custom-mode project to mode=profiles in
// one invocation (flags.mode set explicitly rather than inherited from
// config.Mode).
func TestApplyGenFlags_ProfileWithExplicitModeProfilesKeepsServices(t *testing.T) {
	config := &types.DevcontainerConfig{
		Mode: types.BuildModeCustom,
		Compose: types.ComposeConfig{
			Services: []types.SelectedService{{ID: "postgres"}},
		},
	}
	flags := &genFlags{profile: "nodejs", mode: string(types.BuildModeProfiles)}

	applyGenFlags(config, flags)

	if len(config.Compose.Services) != 1 || config.Compose.Services[0].ID != "postgres" {
		t.Errorf("expected postgres service to survive, got %v", config.Compose.Services)
	}
	if config.Remote == nil || config.Remote.Variant != "nodejs" {
		t.Errorf("expected Remote.Variant = nodejs, got %+v", config.Remote)
	}
}

// --profile is also what keeps this test safe to run headless: initAndConfigure
// asks about skills outside the full wizard too (see the block guarded by
// `flags.profile == ""` there), and that prompt goes through the real,
// non-injectable package-level `console` — there is no fake Prompter to hand
// it in a test. --profile short-circuits that question the same way it
// already short-circuits the build-mode/variant one, so this must keep
// setting profile whenever it exercises the !needsPrompts path with
// interactive: true.
func TestInitAndConfigure_ProfileSkipsPrompts(t *testing.T) {
	dir := t.TempDir()

	flags := &genFlags{
		profile:     "nodejs",
		interactive: true,
	}

	svc := service.GenerateService{}

	config, err := initAndConfigure(dir, flags, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config.Dockerfile.Modules) != 3 {
		t.Errorf("expected 3 modules from early-resolved profile, got %d", len(config.Dockerfile.Modules))
	}
}

// Supplying --skill must skip the interactive skills question even outside
// the full wizard — like --profile, it is one more way to avoid the real,
// non-injectable `console` prompt that block would otherwise trigger in a
// test binary.
func TestInitAndConfigure_ExplicitSkillsSkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	flags := &genFlags{
		interactive: true,
		withModules: []string{"nodejs"},
		skills:      []types.SkillID{types.SkillFirecrawl},
	}
	svc := service.GenerateService{}

	config, err := initAndConfigure(dir, flags, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(config.Skills.Skills) != 1 || config.Skills.Skills[0] != types.SkillFirecrawl {
		t.Errorf("expected the explicit skill to be applied, got %+v", config.Skills.Skills)
	}
}

func TestValidateConfig_DisambiguatesCollidingWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// A different project already claims the workspace name "api".
	if err := domain.SaveRegistry([]domain.ImageEntry{{ProjectDir: "/some/other/api", Workspace: "api"}}); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	config := &types.DevcontainerConfig{Workspace: "api", Mode: types.BuildModeCustom, Image: "api:local"}
	flags := &genFlags{interactive: true} // keepWorkspace false → auto-uniquify
	if err := validateConfig(cwd, config, flags, service.GenerateService{}); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	if !strings.HasPrefix(config.Workspace, "api-") || config.Workspace == "api" {
		t.Errorf("expected a disambiguated 'api-<hash>' workspace, got %q", config.Workspace)
	}
}

func TestValidateConfig_KeepsPinnedWorkspaceOnCollision(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := domain.SaveRegistry([]domain.ImageEntry{{ProjectDir: "/some/other/api", Workspace: "api"}}); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(t.TempDir(), "api")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	config := &types.DevcontainerConfig{Workspace: "api", Mode: types.BuildModeCustom, Image: "api:local"}
	flags := &genFlags{interactive: true, keepWorkspace: true} // pinned → warn, don't rewrite
	if err := validateConfig(cwd, config, flags, service.GenerateService{}); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	if config.Workspace != "api" {
		t.Errorf("pinned workspace must not be rewritten, got %q", config.Workspace)
	}
}

// mode=profiles rejects a [local]-only --profile (e.g. 'scraper'): it has no
// published image, so the pull would just fail — caught here instead, once
// the effective mode is known, not eagerly at flag-parse time (where the same
// --profile is valid for mode=custom).
func TestValidateConfig_ModeProfilesRejectsLocalOnlyProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd := t.TempDir()
	config := &types.DevcontainerConfig{
		Workspace: "ws",
		Mode:      types.BuildModeProfiles,
		Remote:    &types.RemoteConfig{Variant: "scraper"},
	}
	flags := &genFlags{interactive: true, keepWorkspace: true}
	err := validateConfig(cwd, config, flags, service.GenerateService{})
	if err == nil {
		t.Fatal("expected an error for a [local]-only profile under mode=profiles")
	}
}

// mode=profiles accepts "ssh", the hand-built full image, even though it is
// not itself a catalog profile.
func TestValidateConfig_ModeProfilesAcceptsSSH(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd := t.TempDir()
	config := &types.DevcontainerConfig{
		Workspace: "ws",
		Mode:      types.BuildModeProfiles,
		Remote:    &types.RemoteConfig{Variant: "ssh"},
	}
	flags := &genFlags{interactive: true, keepWorkspace: true}
	if err := validateConfig(cwd, config, flags, service.GenerateService{}); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
}

// The mirror of the case above: "ssh" only means anything as a pull target, so
// under mode=custom it is rejected rather than silently applied as an empty
// module bundle.
func TestValidateConfig_ModeCustomRejectsSSHProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cwd := t.TempDir()
	config := &types.DevcontainerConfig{
		Workspace: "ws",
		Mode:      types.BuildModeCustom,
		Image:     "devcontainer-cli/abc123:latest",
	}
	flags := &genFlags{interactive: true, keepWorkspace: true, profile: remoteVariantSSH}
	err := validateConfig(cwd, config, flags, service.GenerateService{})
	if err == nil {
		t.Fatal("expected an error for --profile ssh under mode=custom")
	}
	if !strings.Contains(err.Error(), "pull target") {
		t.Errorf("the error must say ssh is a pull target only, got: %v", err)
	}
}

func TestProfileCommand_CreateAndCopy(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	profileCmd := findSubcommand(configCmd, "profile")
	if profileCmd == nil {
		t.Fatal("expected 'config profile' command to be registered")
	}

	// Verify all subcommands exist
	subcommands := map[string]bool{}
	for _, sub := range profileCmd.Commands() {
		subcommands[sub.Name()] = true
	}

	for _, name := range []string{"list", "create", "copy", "remove"} {
		if !subcommands[name] {
			t.Errorf("expected profile subcommand %q to exist", name)
		}
	}
}

func TestProfileCreate_NoInteractiveFails(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "create", "--no-interactive"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: 'profile create' must fail in --no-interactive mode")
	}
}

func TestProfileCreate_RejectsPositionalArg(t *testing.T) {
	// create no longer takes a positional id; the id is prompted interactively.
	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "create", "some-id"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: 'profile create' no longer accepts a positional profile id")
	}
}

func TestProfileCopy(t *testing.T) {
	// Setup isolated XDG config home
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	// Copy 'nodejs' (builtin) to 'my-copied-nodejs' with --no-interactive
	root.SetArgs([]string{"config", "profile", "copy", "nodejs", "my-copied-nodejs", "--no-interactive"})

	// Run command
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error executing profile copy: %v", err)
	}

	// A profile with no scripts stays a flat <id>.yml.
	copiedPath := filepath.Join(tmpHome, "devcontainer-cli", "profiles", "my-copied-nodejs.yml")
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Fatalf("expected copied profile file to exist at %s, but it does not", copiedPath)
	}

	data, err := os.ReadFile(copiedPath)
	if err != nil {
		t.Fatalf("failed to read copied profile file: %v", err)
	}

	var p catalog.Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatalf("failed to unmarshal copied profile: %v", err)
	}

	if p.ID != "my-copied-nodejs" {
		t.Errorf("expected copied profile ID to be %q, got %q", "my-copied-nodejs", p.ID)
	}

	// It should copy modules from built-in nodejs profile
	if len(p.Modules) == 0 {
		t.Error("expected copied profile to have modules from 'nodejs' profile")
	}
}

// Copying a profile that carries scripts has to copy the scripts too, or the
// copy would reference files that only exist in the source profile.
func TestProfileCopy_CarriesScripts(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	srcDir := filepath.Join(tmpHome, "devcontainer-cli", "profiles", "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "id: src\nmodules: [github-cli]\nscripts:\n  - file: setup.sh\n    when: start\n"
	if err := os.WriteFile(filepath.Join(srcDir, "profile.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "setup.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "copy", "src", "dst", "--no-interactive"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error executing profile copy: %v", err)
	}

	dstDir := filepath.Join(tmpHome, "devcontainer-cli", "profiles", "dst")
	if _, err := os.Stat(filepath.Join(dstDir, "profile.yml")); err != nil {
		t.Fatalf("expected copied profile manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "setup.sh")); err != nil {
		t.Fatalf("expected the script to be copied alongside the manifest: %v", err)
	}
}

func TestProfileRemove_UserProfile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)
	profilesDir := filepath.Join(tmpHome, "devcontainer-cli", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(profilesDir, "scratch.yml")
	if err := os.WriteFile(target, []byte("id: scratch\nmodules: [github-cli]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "remove", "scratch", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error removing profile: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected profile file %s to be deleted", target)
	}
}

// A directory-shaped profile owns its scripts, so removing it takes the whole
// directory rather than leaving the scripts orphaned.
func TestProfileRemove_DirectoryShaped(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)
	dir := filepath.Join(tmpHome, "devcontainer-cli", "profiles", "withscripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile.yml"), []byte("id: withscripts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "remove", "withscripts", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error removing profile: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected profile directory %s to be deleted", dir)
	}
}

// Profiles saved under the pre-rename presets/ directory must keep resolving.
func TestProfileRemove_LegacyPresetsDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)
	legacy := filepath.Join(tmpHome, "devcontainer-cli", "presets")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(legacy, "old.yml")
	if err := os.WriteFile(target, []byte("id: old\nmodules: [github-cli]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "remove", "old", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error removing legacy profile: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected legacy profile file %s to be deleted", target)
	}
}

func TestProfileRemove_BuiltinIsRejected(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "remove", "nodejs", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: built-in profile 'nodejs' must not be removable")
	}
}

func TestProfileRemove_UnknownFails(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "profile", "remove", "does-not-exist", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error removing a non-existent user profile")
	}
}

// scriptedAliasPrompter is a fake aliasPrompter: it returns a canned Select
// choice and, for the rename branch, a canned alias from AskDefault.
type scriptedAliasPrompter struct {
	selectValue string
	renameTo    string
	selectErr   error
}

func (p scriptedAliasPrompter) Select(_ string, _ []service.Option, initial service.Option) (service.Option, error) {
	if p.selectErr != nil {
		return service.Option{}, p.selectErr
	}
	return service.Option{Value: p.selectValue}, nil
}

func (p scriptedAliasPrompter) AskDefault(_ string, initial string, validate func(string) error) (string, error) {
	val := p.renameTo
	if val == "" {
		val = initial
	}
	if validate != nil {
		if err := validate(val); err != nil {
			return "", err
		}
	}
	return val, nil
}

// In assume-yes mode an existing alias is overwritten without prompting,
// preserving the previous non-interactive default.
func TestResolveAliasConflict_AssumeYesOverwrites(t *testing.T) {
	f := &setupSshFlags{alias: "myws", assumeYes: true}
	// A prompter that would explode if consulted proves assume-yes short-circuits.
	got, err := resolveAliasConflict(scriptedAliasPrompter{selectErr: errProbe}, f)
	if err != nil {
		t.Fatalf("resolveAliasConflict: %v", err)
	}
	if got != aliasOverwrite {
		t.Errorf("assume-yes action = %v, want aliasOverwrite", got)
	}
	if f.alias != "myws" {
		t.Errorf("assume-yes must not rename alias, got %q", f.alias)
	}
}

var errProbe = fmtError("prompter should not be consulted")

type fmtError string

func (e fmtError) Error() string { return string(e) }

func TestResolveAliasConflict_Overwrite(t *testing.T) {
	f := &setupSshFlags{alias: "myws"}
	got, err := resolveAliasConflict(scriptedAliasPrompter{selectValue: "overwrite"}, f)
	if err != nil {
		t.Fatalf("resolveAliasConflict: %v", err)
	}
	if got != aliasOverwrite {
		t.Errorf("action = %v, want aliasOverwrite", got)
	}
	if f.alias != "myws" {
		t.Errorf("overwrite must keep alias, got %q", f.alias)
	}
}

func TestResolveAliasConflict_Skip(t *testing.T) {
	f := &setupSshFlags{alias: "myws"}
	got, err := resolveAliasConflict(scriptedAliasPrompter{selectValue: "skip"}, f)
	if err != nil {
		t.Fatalf("resolveAliasConflict: %v", err)
	}
	if got != aliasSkip {
		t.Errorf("action = %v, want aliasSkip", got)
	}
}

// Choosing "rename" prompts for a new alias and updates f.alias so the caller
// rebuilds (and re-checks) the block under the new name.
func TestResolveAliasConflict_RenameUpdatesAlias(t *testing.T) {
	f := &setupSshFlags{alias: "myws"}
	got, err := resolveAliasConflict(scriptedAliasPrompter{selectValue: "rename", renameTo: "myws-2"}, f)
	if err != nil {
		t.Fatalf("resolveAliasConflict: %v", err)
	}
	if got != aliasRename {
		t.Errorf("action = %v, want aliasRename", got)
	}
	if f.alias != "myws-2" {
		t.Errorf("rename should set alias to %q, got %q", "myws-2", f.alias)
	}
}

func TestValidateAliasName(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"myws", false},
		{"", true},
		{"   ", true},
	}
	for _, c := range cases {
		err := validateAliasName(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("validateAliasName(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

// The generated local block must scope host-key verification to the CLI-managed
// known_hosts: a devcontainer regenerates its host keys on every image rebuild
// while keeping the same IP, which otherwise makes ssh abort with "REMOTE HOST
// IDENTIFICATION HAS CHANGED" against the user's global known_hosts.
func TestBuildConfigBlock_LocalPinsManagedKnownHosts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSshCommand()
	f, err := collectSetupSshFlags(cmd)
	if err != nil {
		t.Fatalf("collectSetupSshFlags: %v", err)
	}
	if f.knownHosts != domain.ManagedKnownHostsPath() {
		t.Errorf("knownHosts = %q, want the managed path %q", f.knownHosts, domain.ManagedKnownHostsPath())
	}
	f.alias, f.user = "myws", "devuser"

	block, err := buildConfigBlock(sshdefaults.ModeLocal, f, installResult{hostname: "172.25.1.30"}, sshdefaults.KindWorkspace, "myws")
	if err != nil {
		t.Fatalf("buildConfigBlock(local): %v", err)
	}
	for _, want := range []string{
		"UserKnownHostsFile " + domain.ManagedKnownHostsPath(),
		"StrictHostKeyChecking accept-new",
		"HashKnownHosts no",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("local block missing %q:\n%s", want, block)
		}
	}
}

// The exported remote block lands in another machine's ~/.ssh/config, so — like
// IdentityFile — its known_hosts path must be ~-relative, never this host's.
func TestEmitRemoteConfig_KnownHostsIsTildeRelative(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_devcontainer")
	if err := os.WriteFile(keyPath, []byte("PRIV-KEY"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 PUB"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	f := &setupSshFlags{
		alias:      "myws",
		user:       "devuser",
		key:        keyPath,
		knownHosts: filepath.Join(dir, "known_hosts"),
		remote:     "user@host",
		container:  "myws-devcontainer-ssh",
	}

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	emitErr := emitRemoteConfig(service.SshService{Report: console}, f, sshdefaults.KindWorkspace, "myws")
	_ = w.Close()
	os.Stdout = orig
	if emitErr != nil {
		t.Fatalf("emitRemoteConfig: %v", emitErr)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "UserKnownHostsFile "+sshdefaults.RemoteKnownHostsPath) {
		t.Errorf("remote block should point at %s:\n%s", sshdefaults.RemoteKnownHostsPath, got)
	}
	if strings.Contains(got, dir) {
		t.Errorf("remote output must not leak this host's absolute paths:\n%s", got)
	}
	// The rewrite is local to the render; f keeps this host's managed path.
	if f.knownHosts == sshdefaults.RemoteKnownHostsPath {
		t.Error("emitRemoteConfig mutated f.knownHosts")
	}
}

func TestStaticCompletion(t *testing.T) {
	fn := staticCompletion("bash", "zsh", "sh")
	got, directive := fn(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive=%v, want NoFileComp", directive)
	}
	want := []string{"bash", "zsh", "sh"}
	if !slices.Equal(got, want) {
		t.Errorf("staticCompletion items=%v, want %v", got, want)
	}

	empty := staticCompletion()
	if items, _ := empty(nil, nil, "x"); len(items) != 0 {
		t.Errorf("empty staticCompletion returned %v", items)
	}
}

// A subnet clash is resolved the same way in both modes when the subnet is the
// built-in default: nothing is being decided for the user, so --no-interactive
// must not turn an automatic reassignment into an error.
func TestResolveSubnet(t *testing.T) {
	taken, ok := domain.ParseCidr(domain.DefaultSubnet)
	if !ok {
		t.Fatal("parsing the default subnet")
	}
	used := []domain.CidrRange{*taken}
	pinned := "10.42.0.0/28"
	pinnedTaken, _ := domain.ParseCidr(pinned)

	cases := []struct {
		name        string
		subnet      string
		used        []domain.CidrRange
		interactive bool
		wantMoved   bool
		wantErr     bool
	}{
		{name: "free subnet is kept", subnet: domain.DefaultSubnet, used: nil},
		{name: "default moves when interactive", subnet: domain.DefaultSubnet, used: used, interactive: true, wantMoved: true},
		{name: "default moves when non-interactive", subnet: domain.DefaultSubnet, used: used, wantMoved: true},
		{name: "pinned moves when interactive", subnet: pinned, used: []domain.CidrRange{*pinnedTaken}, interactive: true, wantMoved: true},
		{name: "pinned errors when non-interactive", subnet: pinned, used: []domain.CidrRange{*pinnedTaken}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveSubnet(c.subnet, c.used, c.interactive)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got subnet %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSubnet: %v", err)
			}
			if moved := got != c.subnet; moved != c.wantMoved {
				t.Errorf("subnet = %q (moved=%v), want moved=%v", got, moved, c.wantMoved)
			}
			if got == "" {
				t.Error("a resolved subnet must never be empty")
			}
		})
	}
}
