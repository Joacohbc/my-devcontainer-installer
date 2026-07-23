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
		"ssh", "setup-ssh", "clean-ssh", "port-forward", "run", "down", "destroy",
		"start", "stop", "restart", "prune", "update",
		"upgrade-cli", "config", "cleanup-tips", "shell", "logs", "copy",
		"up", "status", "ls", "info", "remove-container", "remove-image",
		"network",
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

func TestRemoveCommands_HaveShortAliases(t *testing.T) {
	root := NewRootCommand("test")
	want := map[string]string{"remove-container": "rm", "remove-image": "rmi"}
	for _, c := range root.Commands() {
		alias, ok := want[c.Name()]
		if !ok {
			continue
		}
		if !slices.Contains(c.Aliases, alias) {
			t.Errorf("expected command %q to have alias %q, got %v", c.Name(), alias, c.Aliases)
		}
		delete(want, c.Name())
	}
	for name := range want {
		t.Errorf("command %q not registered", name)
	}
}

func TestSelectByNames(t *testing.T) {
	images := []service.LocalImage{
		{Ref: "devcontainer-cli/a:latest", ID: "ID1"},
		{Ref: "devcontainer-cli/b:latest", ID: "ID2"},
		{Ref: "ghcr.io/o/devcontainer-go:latest", ID: "ID3"},
	}
	nameOf := func(i service.LocalImage) string { return i.Ref }

	selected, missing := selectByNames(images, []string{"devcontainer-cli/b:latest", "ghcr.io/o/devcontainer-go:latest"}, nameOf)
	if len(missing) != 0 {
		t.Fatalf("unexpected missing: %v", missing)
	}
	if len(selected) != 2 || selected[0].Ref != "devcontainer-cli/b:latest" || selected[1].Ref != "ghcr.io/o/devcontainer-go:latest" {
		t.Fatalf("selected mismatch (order should follow names): %+v", selected)
	}

	selected, missing = selectByNames(images, []string{"devcontainer-cli/a:latest", "nope:latest"}, nameOf)
	if len(selected) != 1 || selected[0].Ref != "devcontainer-cli/a:latest" {
		t.Errorf("expected only the matching image, got %+v", selected)
	}
	if len(missing) != 1 || missing[0] != "nope:latest" {
		t.Errorf("expected missing [nope:latest], got %v", missing)
	}
}

func TestRemoveCommands_AcceptArgsAndComplete(t *testing.T) {
	root := NewRootCommand("test")
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		byName[c.Name()] = c
	}
	for _, name := range []string{"remove-container", "remove-image"} {
		cmd := byName[name]
		if cmd == nil {
			t.Fatalf("command %q not registered", name)
		}
		// Positional args must be accepted (default arbitrary args, no validator
		// that rejects them).
		if cmd.Args != nil {
			if err := cmd.Args(cmd, []string{"some-name"}); err != nil {
				t.Errorf("%s should accept a positional arg: %v", name, err)
			}
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%s should register positional-arg completion", name)
		}
	}
}

func TestPruneCommand_HasResourceSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	var prune *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "prune" {
			prune = c
			break
		}
	}
	if prune == nil {
		t.Fatal("prune command not registered")
	}
	have := map[string]bool{}
	for _, c := range prune.Commands() {
		have[c.Name()] = true
	}
	for _, name := range []string{"images", "network", "volume"} {
		if !have[name] {
			t.Errorf("expected prune subcommand %q", name)
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
		if sub.Flags().Lookup("workspace") == nil {
			t.Errorf("network %s missing --workspace flag", name)
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

func TestPruneAndRemovalCommands_HaveAllFlag(t *testing.T) {
	root := NewRootCommand("test")
	byName := map[string]*cobra.Command{}
	for _, c := range root.Commands() {
		byName[c.Name()] = c
	}
	// Top-level commands carrying --all.
	for _, name := range []string{"prune", "remove-container", "remove-image"} {
		cmd := byName[name]
		if cmd == nil {
			t.Errorf("command %q not registered", name)
			continue
		}
		if cmd.Flags().Lookup("all") == nil {
			t.Errorf("expected command %q to have --all flag", name)
		}
	}
	// prune subcommands carrying --all.
	prune := byName["prune"]
	for _, sub := range prune.Commands() {
		if sub.Flags().Lookup("all") == nil {
			t.Errorf("expected prune subcommand %q to have --all flag", sub.Name())
		}
	}
}

func TestRootCommand_HasGenerateFlags(t *testing.T) {
	root := NewRootCommand("test")
	for _, name := range []string{"mode", "variant", "with", "service", "image", "workspace", "force", "build", "no-build", "version"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("expected root flag --%s", name)
		}
	}
}

func TestCleanSshCommand_Flags(t *testing.T) {
	root := NewRootCommand("test")
	var clean *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "clean-ssh" {
			clean = c
			break
		}
	}
	if clean == nil {
		t.Fatal("clean-ssh command not registered")
	}
	for _, name := range []string{"dry-run", "yes", "no-interactive"} {
		if clean.Flags().Lookup(name) == nil {
			t.Errorf("expected clean-ssh flag --%s", name)
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
		"devcontainer-cli remove-container",
		"devcontainer-cli remove-image",
		"devcontainer-cli prune",
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

func TestPruneVolumeCommand_HasSharedFlag(t *testing.T) {
	root := NewRootCommand("test")
	var prune *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "prune" {
			prune = c
		}
	}
	if prune == nil {
		t.Fatal("prune command not found")
	}
	var vol *cobra.Command
	for _, c := range prune.Commands() {
		if c.Name() == "volume" {
			vol = c
		}
	}
	if vol == nil {
		t.Fatal("prune volume subcommand not found")
	}
	if vol.Flags().Lookup("shared") == nil {
		t.Error("expected prune volume --shared flag")
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

func TestResolveSshTargetContainerExplicit(t *testing.T) {
	target, err := resolveSshTarget("/ignored", "", "myws-postgres", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := sshTarget{
		containerName:     "myws-postgres",
		containerExplicit: true,
		markerKind:        sshdefaults.KindContainer,
		markerRef:         "myws-postgres",
	}
	if target != want {
		t.Errorf("resolveSshTarget(explicit) = %+v, want %+v", target, want)
	}
}

func TestResolveSshTargetWorkspace(t *testing.T) {
	cwd := t.TempDir()
	target, err := resolveSshTarget(cwd, "myws", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.containerExplicit {
		t.Error("workspace mode must not set containerExplicit")
	}
	if target.markerKind != sshdefaults.KindWorkspace || target.markerRef != "myws" {
		t.Errorf("marker = %s/%s, want workspace/myws", target.markerKind, target.markerRef)
	}
	if target.containerName != "myws-devcontainer-ssh" {
		t.Errorf("containerName = %q, want myws-devcontainer-ssh", target.containerName)
	}
}

func TestBootstrapSetupSshFlagsContainerExplicit(t *testing.T) {
	target := sshTarget{
		containerName:     "myws-postgres",
		containerExplicit: true,
		markerKind:        sshdefaults.KindContainer,
		markerRef:         "myws-postgres",
	}
	f, workspace := bootstrapSetupSshFlags(target, true)
	if workspace != "(none)" {
		t.Errorf("workspace label = %q, want (none)", workspace)
	}
	if !f.containerExplicit || f.container != "myws-postgres" || f.alias != "myws-postgres" || f.composeFile != "" {
		t.Errorf("unexpected flags: %+v", f)
	}
}

func TestBootstrapSetupSshFlagsWorkspace(t *testing.T) {
	target := sshTarget{
		containerName: "myws-devcontainer-ssh",
		markerKind:    sshdefaults.KindWorkspace,
		markerRef:     "myws",
	}
	f, workspace := bootstrapSetupSshFlags(target, false)
	if workspace != "myws" {
		t.Errorf("workspace label = %q, want myws", workspace)
	}
	if f.containerExplicit || f.container != "myws-devcontainer-ssh" || f.alias != "myws" || f.composeFile == "" {
		t.Errorf("unexpected flags: %+v", f)
	}
}

func TestSshCommandRegistered(t *testing.T) {
	root := NewRootCommand("test")
	sshCmd := findSubcommand(root, "ssh")
	if sshCmd == nil {
		t.Fatal("expected 'ssh' command to be registered")
	}
	for _, name := range []string{"workspace", "container", "yes", "no-interactive", "forward", "ports"} {
		if sshCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected 'ssh' to register --%s", name)
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

// -c is the shorthand for --container, matching every other command that
// targets a container.
func TestSetupSshCommand_ContainerShorthand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSetupSshCommand()
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

// With no --key, setup-ssh defaults to the shared managed key under the CLI
// config dir, not a per-host ~/.ssh path.
func TestCollectSetupSshFlags_DefaultsToManagedKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := newSetupSshCommand()
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
	for _, name := range []string{"workspace", "user", "no-tty"} {
		if shell.Flags().Lookup(name) == nil {
			t.Errorf("expected shell flag --%s", name)
		}
	}
	if shell.Flags().ShorthandLookup("T") == nil {
		t.Error("expected shell flag shorthand -T for --no-tty")
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
	for _, name := range []string{"workspace", "follow", "tail"} {
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

func TestResolveProjectComposeFileWithWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	wsDir := filepath.Join(tempDir, ".dc_my-ws", "build")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	compFile := filepath.Join(wsDir, "docker-compose.yml")
	if err := os.WriteFile(compFile, []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := resolveProjectComposeFileWithWorkspace(tempDir, "my-ws")
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
	for _, name := range []string{"workspace"} {
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
	for _, name := range []string{"workspace", "build"} {
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
	for _, name := range []string{"workspace", "all", "long"} {
		if lsCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected ls flag --%s", name)
		}
	}
}

func TestAllCommands_HaveContainerFlag(t *testing.T) {
	root := NewRootCommand("test")
	cmds := []string{"copy", "update", "ls", "logs", "status", "shell", "start", "stop", "info"}
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
		Mode:      types.BuildModeLocalCached,
		Image:     "devcontainer-cli/test:latest",
		Workspace: "my-test-workspace",
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{
				{ID: "golang", Options: map[string]any{}},
			},
		},
		Compose: types.ComposeConfig{
			Services: []any{"postgres"},
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

func TestConfigPresetCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	presetCmd := findSubcommand(configCmd, "preset")
	if presetCmd == nil {
		t.Fatal("expected 'config preset' command to be registered")
	}
	if findSubcommand(presetCmd, "list") == nil {
		t.Error("expected 'config preset list' subcommand")
	}
}

func TestPresetCommand_NotTopLevel(t *testing.T) {
	root := NewRootCommand("test")
	if findSubcommand(root, "preset") != nil {
		t.Error("expected no top-level 'preset' command; it now lives under 'config preset'")
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

func TestApplyGenFlags_PresetWithoutServices(t *testing.T) {
	config := &types.DevcontainerConfig{
		Dockerfile: types.DockerfileConfig{
			Modules: []types.SelectedModule{
				{ID: "golang"},
			},
		},
		Compose: types.ComposeConfig{
			Services: []any{
				types.SelectedModule{ID: "postgres"},
			},
		},
	}
	flags := &genFlags{
		preset: "nodejs",
	}

	applyGenFlags(config, flags)

	if len(config.Dockerfile.Modules) != 4 {
		t.Errorf("expected 4 modules, got %d", len(config.Dockerfile.Modules))
	}
	if len(config.Compose.Services) != 0 {
		t.Errorf("expected 0 services after applying service-less preset, got %d: %v", len(config.Compose.Services), config.Compose.Services)
	}
}

func TestInitAndConfigure_PresetSkipsPrompts(t *testing.T) {
	dir := t.TempDir()

	flags := &genFlags{
		preset:      "nodejs",
		interactive: true,
	}

	svc := service.GenerateService{}

	config, err := initAndConfigure(dir, flags, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config.Dockerfile.Modules) != 4 {
		t.Errorf("expected 4 modules from early-resolved preset, got %d", len(config.Dockerfile.Modules))
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
	config := &types.DevcontainerConfig{Workspace: "api", Mode: types.BuildModeLocalCached, Image: "api:local"}
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
	config := &types.DevcontainerConfig{Workspace: "api", Mode: types.BuildModeLocalCached, Image: "api:local"}
	flags := &genFlags{interactive: true, keepWorkspace: true} // pinned → warn, don't rewrite
	if err := validateConfig(cwd, config, flags, service.GenerateService{}); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
	if config.Workspace != "api" {
		t.Errorf("pinned workspace must not be rewritten, got %q", config.Workspace)
	}
}

func TestPresetCommand_CreateAndCopy(t *testing.T) {
	root := NewRootCommand("test")
	configCmd := findSubcommand(root, "config")
	if configCmd == nil {
		t.Fatal("expected 'config' command to be registered")
	}
	presetCmd := findSubcommand(configCmd, "preset")
	if presetCmd == nil {
		t.Fatal("expected 'config preset' command to be registered")
	}

	// Verify all subcommands exist
	subcommands := map[string]bool{}
	for _, sub := range presetCmd.Commands() {
		subcommands[sub.Name()] = true
	}

	for _, name := range []string{"list", "create", "copy", "remove"} {
		if !subcommands[name] {
			t.Errorf("expected preset subcommand %q to exist", name)
		}
	}
}

func TestPresetCreate_NoInteractiveFails(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "preset", "create", "--no-interactive"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: 'preset create' must fail in --no-interactive mode")
	}
}

func TestPresetCreate_RejectsPositionalArg(t *testing.T) {
	// create no longer takes a positional id; the id is prompted interactively.
	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "preset", "create", "some-id"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: 'preset create' no longer accepts a positional preset id")
	}
}

func TestPresetCopy(t *testing.T) {
	// Setup isolated XDG config home
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	// Copy 'nodejs' (builtin) to 'my-copied-nodejs' with --no-interactive
	root.SetArgs([]string{"config", "preset", "copy", "nodejs", "my-copied-nodejs", "--no-interactive"})

	// Run command
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error executing preset copy: %v", err)
	}

	// Verify the file was created and contains the copied details
	presetsDir := filepath.Join(tmpHome, "devcontainer-cli", "presets")
	copiedPath := filepath.Join(presetsDir, "my-copied-nodejs.yml")
	if _, err := os.Stat(copiedPath); os.IsNotExist(err) {
		t.Fatalf("expected copied preset file to exist at %s, but it does not", copiedPath)
	}

	data, err := os.ReadFile(copiedPath)
	if err != nil {
		t.Fatalf("failed to read copied preset file: %v", err)
	}

	var p catalog.Preset
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatalf("failed to unmarshal copied preset: %v", err)
	}

	if p.ID != "my-copied-nodejs" {
		t.Errorf("expected copied preset ID to be %q, got %q", "my-copied-nodejs", p.ID)
	}

	// It should copy modules from built-in nodejs preset
	if len(p.Modules) == 0 {
		t.Error("expected copied preset to have modules from 'nodejs' preset")
	}
}

func TestPresetRemove_UserPreset(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)
	presetsDir := filepath.Join(tmpHome, "devcontainer-cli", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(presetsDir, "scratch.yml")
	if err := os.WriteFile(target, []byte("id: scratch\nmodules: [github-cli]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "preset", "remove", "scratch", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error removing preset: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected preset file %s to be deleted", target)
	}
}

func TestPresetRemove_BuiltinIsRejected(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "preset", "remove", "nodejs", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error: built-in preset 'nodejs' must not be removable")
	}
}

func TestPresetRemove_UnknownFails(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "preset", "remove", "does-not-exist", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error removing a non-existent user preset")
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
