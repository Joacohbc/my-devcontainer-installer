package commands

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/docker"
	"github.com/spf13/cobra"
)

func TestForwardViaSubcommandRegistration(t *testing.T) {
	accessCmd := newPortForwardCommand()
	viaCmd, _, err := accessCmd.Find([]string{"via"})
	if err != nil || viaCmd == nil || viaCmd.Name() != "via" {
		t.Fatalf("access forward must register 'via' subcommand, got err=%v", err)
	}

	agentCmd := newAgentForwardCommand()
	agentViaCmd, _, err := agentCmd.Find([]string{"via"})
	if err != nil || agentViaCmd == nil || agentViaCmd.Name() != "via" {
		t.Fatalf("agent forward must register 'via' subcommand, got err=%v", err)
	}
}

func TestForwardViaArgValidation(t *testing.T) {
	accessVia := newPortForwardViaCommand()
	if err := accessVia.Args(accessVia, []string{}); err == nil {
		t.Error("access forward via: expected error with 0 args")
	}
	if err := accessVia.Args(accessVia, []string{"host"}); err == nil {
		t.Error("access forward via: expected error with 1 arg")
	}
	if err := accessVia.Args(accessVia, []string{"host", "container"}); err != nil {
		t.Errorf("access forward via: unexpected error with 2 args: %v", err)
	}
	if err := accessVia.Args(accessVia, []string{"host", "container", "3000"}); err != nil {
		t.Errorf("access forward via: unexpected error with 3 args: %v", err)
	}
	if err := accessVia.Args(accessVia, []string{"host", "container", "3000", "extra"}); err == nil {
		t.Error("access forward via: expected error with 4 args")
	}

	agentVia := newAgentForwardViaCommand()
	if err := agentVia.Args(agentVia, []string{"host", "container"}); err == nil {
		t.Error("agent forward via: expected error with 2 args")
	}
	if err := agentVia.Args(agentVia, []string{"host", "container", "3000"}); err != nil {
		t.Errorf("agent forward via: unexpected error with 3 args: %v", err)
	}
	if err := agentVia.Args(agentVia, []string{"host", "container", "3000", "extra"}); err == nil {
		t.Error("agent forward via: expected error with 4 args")
	}
}

func TestForwardViaCompletionAndAgentDefaults(t *testing.T) {
	accessCmd := newPortForwardCommand()
	if accessCmd.ValidArgsFunction == nil {
		t.Error("access forward must register ValidArgsFunction")
	}

	agentCmd := newAgentForwardCommand()
	if agentCmd.ValidArgsFunction == nil {
		t.Error("agent forward must register ValidArgsFunction")
	}

	accessVia := newPortForwardViaCommand()
	if accessVia.ValidArgsFunction == nil {
		t.Error("access forward via must register ValidArgsFunction")
	}

	agentVia := newAgentForwardViaCommand()
	if agentVia.ValidArgsFunction == nil {
		t.Error("agent forward via must register ValidArgsFunction")
	}
	if err := agentVia.PreRunE(agentVia, []string{"host", "container", "3000"}); err != nil {
		t.Fatalf("agent forward via PreRunE: %v", err)
	}
	for _, name := range []string{"ephemeral", "yes", flagNoInteractive} {
		v, err := agentVia.Flags().GetBool(name)
		if err != nil || !v {
			t.Errorf("agent forward via flag %q = (%v, %v), want (true, nil)", name, v, err)
		}
	}
}

func TestExtractContainerPorts(t *testing.T) {
	cases := []struct {
		name  string
		ports string
		want  []string
	}{
		{
			name:  "exposed tcp excluding ssh",
			ports: "22/tcp, 3000/tcp",
			want:  []string{"3000"},
		},
		{
			name:  "published port mappings use container side",
			ports: "127.0.0.1:8080->80/tcp, 0.0.0.0:5432->5432/tcp",
			want:  []string{"80", "5432"},
		},
		{
			name:  "deduplicates ipv4 and ipv6, ignores udp, invalid, and ssh",
			ports: "0.0.0.0:2222->22/tcp, 53/udp, 0/tcp, 70000/tcp, 8000-8002/tcp, 0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp",
			want:  []string{"8080"},
		},
		{
			name:  "empty",
			ports: "",
			want:  nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractContainerPorts(c.ports)
			if !slices.Equal(got, c.want) {
				t.Errorf("extractContainerPorts(%q) = %v, want %v", c.ports, got, c.want)
			}
		})
	}
}

func TestFilterPortSuggestions(t *testing.T) {
	ports := []string{"80", "3000", "5432", "8080"}
	cases := []struct {
		name       string
		ports      []string
		toComplete string
		want       []string
	}{
		{name: "empty prefix", ports: ports, toComplete: "", want: ports},
		{name: "numeric prefix", ports: ports, toComplete: "80", want: []string{"80", "8080"}},
		{name: "reverse prefix", ports: ports, toComplete: "reverse:", want: []string{"reverse:80", "reverse:3000", "reverse:5432", "reverse:8080"}},
		{name: "case-insensitive reverse prefix", ports: ports, toComplete: "REVERSE:5", want: []string{"REVERSE:5432"}},
		{name: "short r prefix", ports: ports, toComplete: "r:3", want: []string{"r:3000"}},
		{name: "does not double prefix configured reverse spec", ports: []string{"3000", "reverse:5432"}, toComplete: "reverse:", want: []string{"reverse:3000", "reverse:5432"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := filterPortSuggestions(c.ports, c.toComplete)
			if !slices.Equal(got, c.want) {
				t.Errorf("filterPortSuggestions(%v, %q) = %v, want %v", c.ports, c.toComplete, got, c.want)
			}
		})
	}
}

type completionCmdMockRunner struct {
	sawVersion  bool
	hasDeadline bool
	calls       [][]string
	stdoutByCmd map[string]string
	handleFn    func(args []string) (int, string, string)
}

func (m *completionCmdMockRunner) Run(ctx context.Context, args []string, _ string, _ string, _ map[string]string) (int, string, string) {
	m.calls = append(m.calls, args)
	if _, ok := ctx.Deadline(); ok {
		m.hasDeadline = true
	}
	if len(args) >= 2 && args[1] == "version" {
		m.sawVersion = true
		return 1, "", "version skipped in completion"
	}
	if m.handleFn != nil {
		return m.handleFn(args)
	}
	if len(args) >= 2 {
		if out, ok := m.stdoutByCmd[args[1]]; ok {
			return 0, out, ""
		}
	}
	return 1, "", ""
}

func TestCompleteForwardViaArgs_PortArg(t *testing.T) {
	docker.SetCompletionCacheDir(t.TempDir())
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	runner := &completionCmdMockRunner{
		stdoutByCmd: map[string]string{
			"ps": `{"Names":"dc-app","Image":"img","Status":"Up","State":"running","Labels":"","Ports":"22/tcp, 127.0.0.1:8080->80/tcp, 3000/tcp"}`,
		},
	}
	docker.SetRunner(runner)
	docker.ResetDockerCache()

	got, directive := completeForwardViaArgs(nil, []string{"me@remote", "dc-app"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"80", "3000"}
	if !slices.Equal(got, want) {
		t.Errorf("completeForwardViaArgs = %v, want %v", got, want)
	}
	if runner.sawVersion {
		t.Error("expected EnsureDocker (docker version) to be skipped during completion")
	}

	gotRev, _ := completeForwardViaArgs(nil, []string{"me@remote", "dc-app"}, "reverse:3")
	if !slices.Equal(gotRev, []string{"reverse:3000"}) {
		t.Errorf("reverse completion = %v, want [reverse:3000]", gotRev)
	}
	// Cache hit must not trigger a second docker runner call.
	if len(runner.calls) != 1 {
		t.Errorf("expected 1 docker call (second cached), got %d", len(runner.calls))
	}

	gotExtra, _ := completeForwardViaArgs(nil, []string{"me@remote", "dc-app", "3000"}, "")
	if gotExtra != nil {
		t.Errorf("expected nil completions past arg 2, got %v", gotExtra)
	}
}

func TestCompleteForwardArgs_MergesContainerAndConfigPorts(t *testing.T) {
	docker.SetCompletionCacheDir(t.TempDir())
	t.Cleanup(func() {
		docker.SetCompletionCacheDir("")
		docker.ResetRunner()
		docker.ResetDockerCache()
	})

	projDir := filepath.Join(t.TempDir(), "myproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := domain.DefaultConfig(projDir)
	cfg.Workspace = "myproj"
	cfg.ForwardPorts = []string{"5432", "3000", "reverse:9000"}
	if err := domain.SaveConfig(cfg, projDir); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projDir)

	runner := &completionCmdMockRunner{
		stdoutByCmd: map[string]string{
			"ps": `{"Names":"myproj-devcontainer-ssh","Image":"img","Status":"Up","State":"running","Labels":"dev.devcontainer-installer.managed=true","Ports":"22/tcp, 3000/tcp, 0.0.0.0:8080->80/tcp"}`,
		},
	}
	docker.SetRunner(runner)
	docker.ResetDockerCache()

	cmd := newPortForwardCommand()
	got, directive := completeForwardArgs(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := []string{"3000", "80", "5432", "reverse:9000"}
	if !slices.Equal(got, want) {
		t.Errorf("completeForwardArgs = %v, want %v", got, want)
	}
	if runner.sawVersion {
		t.Error("expected EnsureDocker (docker version) to be skipped during local forward completion")
	}
	if !runner.hasDeadline {
		t.Error("expected completion timeout deadline to be applied")
	}
}
