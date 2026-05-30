package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/goccy/go-yaml"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/cli/logger"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/spf13/cobra"
)

func TestNewRootCommand_RegistersAllSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	want := []string{
		"setup-ssh", "port-forward", "run", "down", "destroy",
		"start", "stop", "restart", "prune", "update",
		"upgrade-cli", "config", "cleanup-tips", "shell", "logs", "copy",
		"up", "status", "ls",
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

func TestRootCommand_HasGenerateFlags(t *testing.T) {
	root := NewRootCommand("test")
	for _, name := range []string{"mode", "variant", "with", "service", "image", "workspace", "persist", "force", "build", "no-build", "version"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("expected root flag --%s", name)
		}
	}
}

func TestParseGenFlags_Persist(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantNil bool
		want    []string
		wantErr bool
	}{
		{name: "unset", args: nil, wantNil: true},
		{name: "all", args: []string{"--persist", "all"}, want: []string{"etc", "root", "home"}},
		{name: "none", args: []string{"--persist", "none"}, want: []string{}},
		{name: "empty", args: []string{"--persist", ""}, want: []string{}},
		{name: "subset", args: []string{"--persist", "etc,home"}, want: []string{"etc", "home"}},
		{name: "invalid", args: []string{"--persist", "etc,bogus"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "generate"}
			addGenerateFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			flags, err := parseGenFlags(cmd)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for args %v", tc.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGenFlags: %v", err)
			}
			if tc.wantNil {
				if flags.persist != nil {
					t.Errorf("expected nil persist, got %v", *flags.persist)
				}
				return
			}
			if flags.persist == nil {
				t.Fatalf("expected non-nil persist for args %v", tc.args)
			}
			if len(*flags.persist) != len(tc.want) {
				t.Fatalf("persist = %v, want %v", *flags.persist, tc.want)
			}
			for i := range tc.want {
				if (*flags.persist)[i] != tc.want[i] {
					t.Errorf("persist[%d] = %q, want %q", i, (*flags.persist)[i], tc.want[i])
				}
			}
		})
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
	wants := []string{"db-user", "db-password", "ssh-port"}
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

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.2.0", "1.1.9", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.0-rc1", 1},
		{"1.0.0-rc1", "1.0.0", -1},
		{"1.0.0-rc1", "1.0.0-rc2", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestGetTargetTriplet(t *testing.T) {
	triplet, _, err := getTargetTriplet()
	if err != nil {
		t.Skipf("unsupported platform for this test: %v", err)
	}
	if triplet == "" {
		t.Error("expected a non-empty triplet")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("nodejs, golang ,, tmux")
	want := []string{"nodejs", "golang", "tmux"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestResolveAssetURL(t *testing.T) {
	rel := &release{
		TagName: "v1.0.0",
		Assets: []releaseAsset{
			{Name: "devcontainer-cli-linux-x64", DownloadURL: "https://example.com/bin"},
			{Name: "devcontainer-cli-linux-x64.sha256", DownloadURL: "https://example.com/sum"},
		},
	}
	bin, sum, err := resolveAssetURL(rel, "linux-x64", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bin != "https://example.com/bin" || sum != "https://example.com/sum" {
		t.Errorf("got bin=%q sum=%q", bin, sum)
	}
	if _, _, err := resolveAssetURL(rel, "darwin-arm64", ""); err == nil {
		t.Error("expected error for missing asset")
	}
}

func TestIsAllowedHost(t *testing.T) {
	allowed := []string{"github.com", "api.github.com", "objects.githubusercontent.com"}
	for _, h := range allowed {
		if !isAllowedHost(h) {
			t.Errorf("expected %q to be allowed", h)
		}
	}
	for _, h := range []string{"evil.com", "github.com.evil.com"} {
		if isAllowedHost(h) {
			t.Errorf("expected %q to be denied", h)
		}
	}
}

func TestCompleteCSV(t *testing.T) {
	allModules := []string{"nodejs", "python", "golang", "tmux", "bun"}
	cases := []struct {
		toComplete string
		want       []string
	}{
		{toComplete: "no", want: []string{"nodejs"}},
		{toComplete: "nodejs,go", want: []string{"nodejs,golang"}},
		{toComplete: "nodejs, python, g", want: []string{"nodejs, python,golang"}},
		{toComplete: "nodejs,python,tmux,bun,c", want: []string{}},
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
	origUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserProfile)
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
	for _, name := range []string{"workspace", "user"} {
		if shell.Flags().Lookup(name) == nil {
			t.Errorf("expected shell flag --%s", name)
		}
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
	cmds := []string{"copy", "up", "down", "update", "ls", "logs", "status", "shell"}
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

func TestPresetCommand_Exists(t *testing.T) {
	root := NewRootCommand("test")
	var presetCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "preset" {
			presetCmd = c
			break
		}
	}
	if presetCmd == nil {
		t.Fatal("expected 'preset' command to be registered")
	}
	found := false
	for _, sub := range presetCmd.Commands() {
		if sub.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'preset list' subcommand")
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
