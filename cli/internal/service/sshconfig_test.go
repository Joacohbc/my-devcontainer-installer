package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// setHomeDir points os.UserHomeDir() at dir via the HOME env var.
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
}

const sampleConfig = `Host devcontainer
    HostName 172.18.0.2
    User devuser

Host other
    HostName example.com

Host *.internal
    User admin
`

func TestHasHostAlias(t *testing.T) {
	if !HasHostAlias(sampleConfig, "devcontainer") {
		t.Error("expected devcontainer alias to be present")
	}
	if HasHostAlias(sampleConfig, "missing") {
		t.Error("did not expect missing alias to be present")
	}
}

func TestExtractHostBlock(t *testing.T) {
	block := ExtractHostBlock(sampleConfig, "devcontainer")
	if !strings.HasPrefix(block, "Host devcontainer") || !strings.Contains(block, "172.18.0.2") {
		t.Errorf("unexpected block:\n%s", block)
	}
	if strings.Contains(block, "example.com") {
		t.Errorf("block leaked into the next Host:\n%s", block)
	}
	if ExtractHostBlock(sampleConfig, "missing") != "" {
		t.Error("expected empty block for missing alias")
	}
}

func TestStripHostBlock(t *testing.T) {
	stripped := StripHostBlock(sampleConfig, "devcontainer")
	if strings.Contains(stripped, "172.18.0.2") {
		t.Errorf("devcontainer block not removed:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Host other") || !strings.Contains(stripped, "Host *.internal") {
		t.Errorf("stripping removed unrelated blocks:\n%s", stripped)
	}
}

func TestConfigHostAliases(t *testing.T) {
	aliases := ConfigHostAliases(sampleConfig)
	want := []string{"devcontainer", "other"}
	if len(aliases) != len(want) {
		t.Fatalf("ConfigHostAliases = %v, want %v", aliases, want)
	}
	for i, a := range want {
		if aliases[i] != a {
			t.Errorf("ConfigHostAliases[%d] = %q, want %q (wildcards must be skipped)", i, aliases[i], a)
		}
	}
}

func TestReadAndWriteSSHConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)

	svc := SshService{Report: nopReporter{}}

	// ReadSSHConfig creates ~/.ssh/config when absent.
	path, content, err := svc.ReadSSHConfig()
	if err != nil {
		t.Fatalf("ReadSSHConfig: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty config, got %q", content)
	}
	if path != filepath.Join(home, ".ssh", "config") {
		t.Errorf("unexpected config path: %s", path)
	}

	// AppendHostBlock writes the first block.
	if err := svc.AppendHostBlock(path, content, "Host devcontainer\n    HostName 1.2.3.4"); err != nil {
		t.Fatalf("AppendHostBlock: %v", err)
	}
	_, content, _ = svc.ReadSSHConfig()
	if !HasHostAlias(content, "devcontainer") {
		t.Fatalf("appended block missing:\n%s", content)
	}

	// ReplaceHostBlock swaps the block and leaves a backup.
	backup, err := svc.ReplaceHostBlock(path, content, "devcontainer", "Host devcontainer\n    HostName 5.6.7.8")
	if err != nil {
		t.Fatalf("ReplaceHostBlock: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	_, content, _ = svc.ReadSSHConfig()
	if strings.Contains(content, "1.2.3.4") || !strings.Contains(content, "5.6.7.8") {
		t.Errorf("block not replaced:\n%s", content)
	}
}

const managedConfig = `# devcontainer-cli:managed workspace=proj
Host proj
    HostName 172.18.0.2
    User devuser

Host other
    HostName example.com
`

func TestHasManagedBlock(t *testing.T) {
	if !HasManagedBlock(managedConfig, "proj") {
		t.Error("expected managed block for workspace 'proj'")
	}
	if HasManagedBlock(managedConfig, "missing") {
		t.Error("did not expect managed block for workspace 'missing'")
	}
	if HasManagedBlock(sampleConfig, "devcontainer") {
		t.Error("untagged config must not report a managed block")
	}
}

func TestStripManagedBlock(t *testing.T) {
	stripped := StripManagedBlock(managedConfig, "proj")
	if strings.Contains(stripped, "172.18.0.2") || strings.Contains(stripped, "Host proj") {
		t.Errorf("managed Host block not removed:\n%s", stripped)
	}
	if strings.Contains(stripped, "devcontainer-cli:managed") {
		t.Errorf("managed marker comment not removed:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Host other") || !strings.Contains(stripped, "example.com") {
		t.Errorf("unrelated block was removed:\n%s", stripped)
	}
}

func TestStripHostBlockRemovesPrecedingMarker(t *testing.T) {
	// Stripping a managed block by alias must also drop its marker comment so no
	// orphaned marker is left when setup-ssh replaces an existing block.
	stripped := StripHostBlock(managedConfig, "proj")
	if strings.Contains(stripped, "devcontainer-cli:managed") {
		t.Errorf("orphaned marker left behind:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Host other") {
		t.Errorf("unrelated block was removed:\n%s", stripped)
	}
}

func TestRemoveManagedBlock(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(managedConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	removed, backup, err := svc.RemoveManagedBlock("proj")
	if err != nil {
		t.Fatalf("RemoveManagedBlock: %v", err)
	}
	if !removed {
		t.Fatal("expected removed=true for tagged workspace")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "Host proj") || strings.Contains(got, "devcontainer-cli:managed") {
		t.Errorf("managed block not removed from disk:\n%s", got)
	}
	if !strings.Contains(got, "Host other") {
		t.Errorf("unrelated block lost:\n%s", got)
	}
}

func TestRemoveManagedBlockNoMatch(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}

	// Missing ~/.ssh/config is a no-op and must not create the file.
	removed, _, err := svc.RemoveManagedBlock("proj")
	if err != nil {
		t.Fatalf("RemoveManagedBlock (missing file): %v", err)
	}
	if removed {
		t.Error("expected removed=false when config is absent")
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "config")); !os.IsNotExist(err) {
		t.Error("RemoveManagedBlock must not create ~/.ssh/config")
	}
}

const managedContainerConfig = `# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh
Host dc-ssh
    HostName 172.18.0.5
    User devuser

Host other
    HostName example.com
`

func TestRemoveManagedBlockByRef(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(managedContainerConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}

	// A workspace-kind lookup must not match a container-kind block.
	removed, _, err := svc.RemoveManagedBlockByRef(sshdefaults.KindWorkspace, "dc-ssh")
	if err != nil {
		t.Fatalf("RemoveManagedBlockByRef (workspace kind): %v", err)
	}
	if removed {
		t.Error("expected removed=false for a kind that does not match the block")
	}

	removed, backup, err := svc.RemoveManagedBlockByRef(sshdefaults.KindContainer, "dc-ssh")
	if err != nil {
		t.Fatalf("RemoveManagedBlockByRef (container kind): %v", err)
	}
	if !removed {
		t.Fatal("expected removed=true for tagged container")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "Host dc-ssh") || strings.Contains(got, "devcontainer-cli:managed") {
		t.Errorf("managed block not removed from disk:\n%s", got)
	}
	if !strings.Contains(got, "Host other") {
		t.Errorf("unrelated block lost:\n%s", got)
	}
}

// newFormatConfig mixes a workspace block and a container block in the modern
// structured marker format, plus an untagged block.
const newFormatConfig = `# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api
Host api
    HostName 172.18.0.2
    User devuser

# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh
Host dc-ssh
    HostName 172.18.0.9
    User devuser

Host other
    HostName example.com
`

func TestParseManagedMarker(t *testing.T) {
	cases := []struct {
		name         string
		line         string
		wantOK       bool
		kind, ref, a string
	}{
		{"new workspace", "# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api", true, "workspace", "api-3f9a", "api"},
		{"new container", "# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh", true, "container", "dc-ssh", "dc-ssh"},
		{"legacy workspace", "# devcontainer-cli:managed workspace=proj", true, "workspace", "proj", ""},
		{"indented", "   #  devcontainer-cli:managed kind=workspace ref=x alias=y", true, "workspace", "x", "y"},
		{"not a comment", "Host devcontainer", false, "", "", ""},
		{"other comment", "# just a note", false, "", "", ""},
		{"missing ref", "# devcontainer-cli:managed kind=workspace alias=y", false, "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, ok := parseManagedMarker(c.line)
			if ok != c.wantOK {
				t.Fatalf("parseManagedMarker(%q) ok=%v, want %v", c.line, ok, c.wantOK)
			}
			if !ok {
				return
			}
			if m.Kind != c.kind || m.Ref != c.ref || m.Alias != c.a {
				t.Errorf("parseManagedMarker(%q) = %+v, want {Kind:%q Ref:%q Alias:%q}", c.line, m, c.kind, c.ref, c.a)
			}
		})
	}
}

func TestStripManagedBlockByRef_Container(t *testing.T) {
	stripped := StripManagedBlockByRef(newFormatConfig, "container", "dc-ssh")
	if strings.Contains(stripped, "Host dc-ssh") || strings.Contains(stripped, "172.18.0.9") {
		t.Errorf("container block not removed:\n%s", stripped)
	}
	if strings.Contains(stripped, "kind=container") {
		t.Errorf("container marker not removed:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Host api") || !strings.Contains(stripped, "Host other") {
		t.Errorf("unrelated blocks were removed:\n%s", stripped)
	}
}

func TestListManagedBlocks(t *testing.T) {
	blocks := ListManagedBlocks(newFormatConfig)
	if len(blocks) != 2 {
		t.Fatalf("ListManagedBlocks returned %d blocks, want 2: %+v", len(blocks), blocks)
	}
	want := map[string]ManagedMarker{
		"api-3f9a": {Kind: "workspace", Ref: "api-3f9a", Alias: "api"},
		"dc-ssh":   {Kind: "container", Ref: "dc-ssh", Alias: "dc-ssh"},
	}
	for _, b := range blocks {
		if w, ok := want[b.Ref]; !ok || b != w {
			t.Errorf("unexpected block %+v", b)
		}
	}
}

// ListManagedBlocks recovers the alias from the Host line beneath a legacy marker
// that carries no alias= field.
func TestListManagedBlocksLegacyAlias(t *testing.T) {
	blocks := ListManagedBlocks(managedConfig)
	if len(blocks) != 1 {
		t.Fatalf("want 1 block, got %+v", blocks)
	}
	if blocks[0].Kind != "workspace" || blocks[0].Ref != "proj" || blocks[0].Alias != "proj" {
		t.Errorf("legacy block = %+v, want {workspace proj proj}", blocks[0])
	}
}

func TestAliasForManagedRef(t *testing.T) {
	alias, ok := AliasForManagedRef(newFormatConfig, "workspace", "api-3f9a")
	if !ok || alias != "api" {
		t.Errorf("AliasForManagedRef(workspace,api-3f9a) = %q,%v, want api,true", alias, ok)
	}
	if _, ok := AliasForManagedRef(newFormatConfig, "workspace", "missing"); ok {
		t.Error("did not expect a match for an unknown ref")
	}
	if _, ok := AliasForManagedRef(newFormatConfig, "container", "api-3f9a"); ok {
		t.Error("kind must be matched, not just ref")
	}
}

func TestManagedAlias(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	alias, ok, err := svc.ManagedAlias(sshdefaults.KindWorkspace, "api-3f9a")
	if err != nil || !ok || alias != "api" {
		t.Errorf("ManagedAlias(workspace,api-3f9a) = %q,%v,%v, want api,true,nil", alias, ok, err)
	}
	if _, ok, err := svc.ManagedAlias(sshdefaults.KindWorkspace, "missing"); err != nil || ok {
		t.Errorf("ManagedAlias(missing) = %v,%v, want false,nil", ok, err)
	}
}

func TestManagedAliasMissingConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}
	alias, ok, err := svc.ManagedAlias(sshdefaults.KindWorkspace, "api-3f9a")
	if err != nil || ok || alias != "" {
		t.Errorf("ManagedAlias with no config = %q,%v,%v, want \"\",false,nil", alias, ok, err)
	}
}

func TestPruneManagedBlocks(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	// Workspace api-3f9a still exists; container dc-ssh is gone → only dc-ssh is stale.
	existsWorkspace := func(ref string) bool { return ref == "api-3f9a" }
	existsContainer := func(string) bool { return false }

	// Dry-run reports the stale block but leaves the file untouched.
	stale, backup, err := svc.PruneManagedBlocks(existsWorkspace, existsContainer, true)
	if err != nil {
		t.Fatalf("PruneManagedBlocks(dryRun): %v", err)
	}
	if len(stale) != 1 || stale[0].Ref != "dc-ssh" {
		t.Fatalf("dry-run stale = %+v, want [dc-ssh]", stale)
	}
	if backup != "" {
		t.Errorf("dry-run must not write a backup, got %q", backup)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "Host dc-ssh") {
		t.Error("dry-run must not modify the config")
	}

	// Real run strips the stale container block, keeps the live workspace block.
	stale, backup, err = svc.PruneManagedBlocks(existsWorkspace, existsContainer, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 1 || stale[0].Ref != "dc-ssh" {
		t.Fatalf("stale = %+v, want [dc-ssh]", stale)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "Host dc-ssh") || strings.Contains(got, "kind=container") {
		t.Errorf("stale container block not removed:\n%s", got)
	}
	if !strings.Contains(got, "Host api") || !strings.Contains(got, "Host other") {
		t.Errorf("live/unrelated blocks lost:\n%s", got)
	}
}

func TestPruneManagedBlocksNoStale(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := SshService{Report: nopReporter{}}
	alive := func(string) bool { return true }
	stale, backup, err := svc.PruneManagedBlocks(alive, alive, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 0 || backup != "" {
		t.Errorf("expected no-op when nothing stale, got stale=%+v backup=%q", stale, backup)
	}
}

func TestPruneManagedBlocksMissingConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}
	alive := func(string) bool { return true }
	stale, _, err := svc.PruneManagedBlocks(alive, alive, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks (missing file): %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected empty result for missing config, got %+v", stale)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "config")); !os.IsNotExist(err) {
		t.Error("PruneManagedBlocks must not create ~/.ssh/config")
	}
}

func TestRemoveManagedBlocksSubset(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	// Both blocks are stale, but the caller (clean-ssh's picker) only asks to
	// remove the container one — the workspace block must survive untouched.
	toRemove := []ManagedMarker{{Kind: "container", Ref: "dc-ssh", Alias: "dc-ssh"}}
	removed, backup, err := svc.RemoveManagedBlocks(toRemove)
	if err != nil {
		t.Fatalf("RemoveManagedBlocks: %v", err)
	}
	if len(removed) != 1 || removed[0].Ref != "dc-ssh" {
		t.Fatalf("removed = %+v, want [dc-ssh]", removed)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "Host dc-ssh") {
		t.Errorf("selected block not removed:\n%s", got)
	}
	if !strings.Contains(got, "Host api") || !strings.Contains(got, "Host other") {
		t.Errorf("unselected/unrelated blocks lost:\n%s", got)
	}
}

func TestRemoveManagedBlocksEmpty(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := SshService{Report: nopReporter{}}
	removed, backup, err := svc.RemoveManagedBlocks(nil)
	if err != nil {
		t.Fatalf("RemoveManagedBlocks(nil): %v", err)
	}
	if len(removed) != 0 || backup != "" {
		t.Errorf("expected no-op for empty blocks, got removed=%+v backup=%q", removed, backup)
	}
}

func TestRemoveManagedBlocksMissingConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}
	toRemove := []ManagedMarker{{Kind: "workspace", Ref: "api-3f9a", Alias: "api"}}
	removed, backup, err := svc.RemoveManagedBlocks(toRemove)
	if err != nil {
		t.Fatalf("RemoveManagedBlocks (missing file): %v", err)
	}
	if len(removed) != 0 || backup != "" {
		t.Errorf("expected no-op when config is missing, got removed=%+v backup=%q", removed, backup)
	}
}

func TestConfigHostAliasesFromDisk(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := SshService{Report: nopReporter{}}
	aliases := svc.ConfigHostAliasesFromDisk()
	if len(aliases) != 2 || aliases[0] != "devcontainer" {
		t.Errorf("ConfigHostAliasesFromDisk = %v, want [devcontainer other]", aliases)
	}
}

func TestHostNameForAlias(t *testing.T) {
	cases := []struct {
		name    string
		content string
		alias   string
		want    string
	}{
		{name: "reads the block's HostName", content: sampleConfig, alias: "devcontainer", want: "172.18.0.2"},
		{name: "does not leak a neighbour's HostName", content: sampleConfig, alias: "other", want: "example.com"},
		{name: "unknown alias", content: sampleConfig, alias: "nope", want: ""},
		{
			// A ProxyCommand block sets no HostName; ssh dials the alias itself,
			// so there is no local address to pin a host key against.
			name:    "proxycommand block has none",
			content: "Host jump\n    User devuser\n    ProxyCommand ssh host \"nc -q0 1.2.3.4 22\"\n",
			alias:   "jump",
			want:    "",
		},
		{
			name:    "case-insensitive keyword",
			content: "Host x\n    hostname 10.0.0.9\n",
			alias:   "x",
			want:    "10.0.0.9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HostNameForAlias(c.content, c.alias); got != c.want {
				t.Errorf("HostNameForAlias(%q) = %q, want %q", c.alias, got, c.want)
			}
		})
	}
}

func TestEnsureHostKeyOptions(t *testing.T) {
	const knownHosts = "/cfg/ssh/known_hosts"

	t.Run("upgrades a legacy block in place", func(t *testing.T) {
		content := "Host api\n    HostName 172.25.1.30\n    User devuser\n\nHost other\n    HostName example.com\n"
		got, changed := EnsureHostKeyOptions(content, "api", knownHosts)
		if !changed {
			t.Fatal("expected the legacy block to change")
		}
		want := "Host api\n" +
			"    HostName 172.25.1.30\n" +
			"    User devuser\n" +
			"    UserKnownHostsFile " + knownHosts + "\n" +
			"    StrictHostKeyChecking accept-new\n" +
			"    HashKnownHosts no\n" +
			"\n" +
			"Host other\n    HostName example.com\n"
		if got != want {
			t.Errorf("EnsureHostKeyOptions =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("replaces a stale known_hosts path", func(t *testing.T) {
		content := "Host api\n    HostName 1.2.3.4\n    UserKnownHostsFile /old/known_hosts\n    StrictHostKeyChecking yes\n    HashKnownHosts no\n"
		got, changed := EnsureHostKeyOptions(content, "api", knownHosts)
		if !changed {
			t.Fatal("expected the stale options to change")
		}
		if strings.Contains(got, "/old/known_hosts") || strings.Contains(got, "StrictHostKeyChecking yes") {
			t.Errorf("stale options survived:\n%s", got)
		}
		if strings.Count(got, "UserKnownHostsFile") != 1 {
			t.Errorf("expected exactly one UserKnownHostsFile:\n%s", got)
		}
	})

	t.Run("is a no-op when already pinned", func(t *testing.T) {
		content := "Host api\n    HostName 1.2.3.4\n    UserKnownHostsFile " + knownHosts + "\n    StrictHostKeyChecking accept-new\n    HashKnownHosts no\n"
		got, changed := EnsureHostKeyOptions(content, "api", knownHosts)
		if changed || got != content {
			t.Errorf("expected no change, got changed=%v:\n%s", changed, got)
		}
	})

	t.Run("leaves other blocks alone", func(t *testing.T) {
		got, changed := EnsureHostKeyOptions(sampleConfig, "missing", knownHosts)
		if changed || got != sampleConfig {
			t.Errorf("expected no change for an unknown alias, got changed=%v:\n%s", changed, got)
		}
	})

	t.Run("keeps the marker comment above the block", func(t *testing.T) {
		content := "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\nHost api\n    HostName 1.2.3.4\n"
		got, _ := EnsureHostKeyOptions(content, "api", knownHosts)
		if !strings.HasPrefix(got, "# devcontainer-cli:managed v=1 kind=workspace ref=api alias=api\nHost api\n") {
			t.Errorf("marker comment lost or moved:\n%s", got)
		}
	})
}

func TestEnsureHostKeyPinning(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sshDir, "config")
	legacy := "Host api\n    HostName 172.25.1.30\n    User devuser\n"
	if err := os.WriteFile(configPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	updated, err := svc.EnsureHostKeyPinning("api", "/cfg/ssh/known_hosts")
	if err != nil || !updated {
		t.Fatalf("EnsureHostKeyPinning = %v,%v, want true,nil", updated, err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "UserKnownHostsFile /cfg/ssh/known_hosts") {
		t.Errorf("config not upgraded:\n%s", data)
	}
	backup, err := os.ReadFile(configPath + ".bak")
	if err != nil || string(backup) != legacy {
		t.Errorf("expected the original to be backed up, got %q (%v)", backup, err)
	}

	// A second run has nothing to do and must not churn the backup.
	updated, err = svc.EnsureHostKeyPinning("api", "/cfg/ssh/known_hosts")
	if err != nil || updated {
		t.Errorf("second EnsureHostKeyPinning = %v,%v, want false,nil", updated, err)
	}
}

func TestEnsureHostKeyPinningMissingConfig(t *testing.T) {
	setHomeDir(t, t.TempDir())
	svc := SshService{Report: nopReporter{}}
	updated, err := svc.EnsureHostKeyPinning("api", "/cfg/ssh/known_hosts")
	if err != nil || updated {
		t.Errorf("EnsureHostKeyPinning with no config = %v,%v, want false,nil", updated, err)
	}
}

func TestAliasHostName(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := SshService{Report: nopReporter{}}
	got, err := svc.AliasHostName("devcontainer")
	if err != nil || got != "172.18.0.2" {
		t.Errorf("AliasHostName = %q,%v, want 172.18.0.2,nil", got, err)
	}
}

func TestAliasHostNameMissingConfig(t *testing.T) {
	setHomeDir(t, t.TempDir())
	svc := SshService{Report: nopReporter{}}
	got, err := svc.AliasHostName("devcontainer")
	if err != nil || got != "" {
		t.Errorf("AliasHostName with no config = %q,%v, want \"\",nil", got, err)
	}
}
