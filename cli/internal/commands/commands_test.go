package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRootCommand_RegistersAllSubcommands(t *testing.T) {
	root := NewRootCommand("test")
	want := []string{
		"setup-ssh", "port-forward", "run", "down", "destroy",
		"start", "stop", "restart", "prune", "update",
		"upgrade-cli", "config", "cleanup-tips",
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
	for _, name := range []string{"mode", "variant", "with", "service", "image", "workspace", "force", "build", "no-build", "version"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("expected root flag --%s", name)
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
	got := splitCSV("nodejs, golang ,, dod")
	want := []string{"nodejs", "golang", "dod"}
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
