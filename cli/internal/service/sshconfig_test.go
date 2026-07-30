package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/infra/sshdefaults"
)

// setHomeDir points os.UserHomeDir() at dir via the HOME env var. It also
// isolates the global config (which resolves the managed SSH config path) so a
// developer's real ~/.config/devcontainer-cli never leaks into a test.
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
}

// managedSSHConfig is the CLI-owned SSH config file for a fake HOME — where
// every managed Host block lives.
func managedSSHConfig(home string) string {
	return filepath.Join(home, ".ssh", types.SSHConfigName)
}

// userSSHConfig is the user's own ~/.ssh/config for a fake HOME — the file the
// CLI only ever adds an Include to.
func userSSHConfig(home string) string {
	return filepath.Join(home, ".ssh", "config")
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

	// ReadManagedConfig creates the managed config when absent.
	path, content, err := svc.ReadManagedConfig()
	if err != nil {
		t.Fatalf("ReadManagedConfig: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty config, got %q", content)
	}
	if path != managedSSHConfig(home) {
		t.Errorf("unexpected config path: %s", path)
	}

	// AppendHostBlock writes the first block.
	if err := svc.AppendHostBlock(path, content, "Host devcontainer\n    HostName 1.2.3.4"); err != nil {
		t.Fatalf("AppendHostBlock: %v", err)
	}
	_, content, _ = svc.ReadManagedConfig()
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
	_, content, _ = svc.ReadManagedConfig()
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
	path := managedSSHConfig(home)
	if err := os.WriteFile(path, []byte(managedConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	removed, backup, err := svc.RemoveManagedBlockByRef(sshdefaults.KindWorkspace, "proj")
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
	removed, _, err := svc.RemoveManagedBlockByRef(sshdefaults.KindWorkspace, "proj")
	if err != nil {
		t.Fatalf("RemoveManagedBlock (missing file): %v", err)
	}
	if removed {
		t.Error("expected removed=false when config is absent")
	}
	if _, err := os.Stat(managedSSHConfig(home)); !os.IsNotExist(err) {
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
	path := managedSSHConfig(home)
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
		name            string
		line            string
		wantOK          bool
		kind, ref, a, h string
	}{
		{"new workspace", "# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api", true, "workspace", "api-3f9a", "api", ""},
		{"new container", "# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh", true, "container", "dc-ssh", "dc-ssh", ""},
		{"legacy workspace", "# devcontainer-cli:managed workspace=proj", true, "workspace", "proj", "", ""},
		{"indented", "   #  devcontainer-cli:managed kind=workspace ref=x alias=y", true, "workspace", "x", "y", ""},
		{"via host", "# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh host=me@remote-host", true, "container", "dc-ssh", "dc-ssh", "me@remote-host"},
		{"not a comment", "Host devcontainer", false, "", "", "", ""},
		{"other comment", "# just a note", false, "", "", "", ""},
		{"missing ref", "# devcontainer-cli:managed kind=workspace alias=y", false, "", "", "", ""},
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
			if m.Kind != c.kind || m.Ref != c.ref || m.Alias != c.a || m.Host != c.h {
				t.Errorf("parseManagedMarker(%q) = %+v, want {Kind:%q Ref:%q Alias:%q Host:%q}", c.line, m, c.kind, c.ref, c.a, c.h)
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
	if err := os.WriteFile(managedSSHConfig(home), []byte(newFormatConfig), 0o600); err != nil {
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

func TestListManagedSSHBlocks(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedSSHConfig(home), []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	blocks, err := svc.ListManagedSSHBlocks()
	if err != nil {
		t.Fatalf("ListManagedSSHBlocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("ListManagedSSHBlocks = %+v, want 2 blocks (api workspace + dc-ssh container)", blocks)
	}
}

func TestListManagedSSHBlocks_DedupesDuplicateMarkers(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	content := "# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"\n" +
		"# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"Host myproj\n    HostName 172.20.0.2\n"
	if err := os.WriteFile(managedSSHConfig(home), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	blocks, err := svc.ListManagedSSHBlocks()
	if err != nil {
		t.Fatalf("ListManagedSSHBlocks: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("ListManagedSSHBlocks = %+v, want the duplicate collapsed to 1", blocks)
	}
}

func TestListManagedSSHBlocks_MissingConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}
	blocks, err := svc.ListManagedSSHBlocks()
	if err != nil || len(blocks) != 0 {
		t.Errorf("ListManagedSSHBlocks with no config = %+v,%v, want empty,nil", blocks, err)
	}
}

func TestPruneManagedBlocks(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	// Workspace api-3f9a still exists; container dc-ssh is gone → only dc-ssh is stale.
	existsWorkspace := func(ref string) bool { return ref == "api-3f9a" }
	existsContainer := func(string) bool { return false }
	existsRemoteContainer := func(string, string) (bool, bool) { return true, true }

	// Dry-run reports the stale block but leaves the file untouched.
	stale, _, backup, err := svc.PruneManagedBlocks(existsWorkspace, existsContainer, existsRemoteContainer, true)
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
	stale, _, backup, err = svc.PruneManagedBlocks(existsWorkspace, existsContainer, existsRemoteContainer, false)
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

func TestFindOrphanedMarkers(t *testing.T) {
	// Reproduces the exact shape reported in production: a marker with no
	// Host stanza directly beneath it (left behind by an incomplete
	// append/replace), immediately followed by a second, well-formed copy of
	// the same block.
	content := "# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"\n" +
		"# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"Host myproj\n" +
		"    HostName 172.20.0.2\n"
	orphans := FindOrphanedMarkers(content)
	if len(orphans) != 1 {
		t.Fatalf("FindOrphanedMarkers = %+v, want exactly 1 orphan", orphans)
	}
	if orphans[0].Kind != "workspace" || orphans[0].Ref != "myproj" {
		t.Errorf("orphan = %+v, want kind=workspace ref=myproj", orphans[0])
	}
}

func TestFindOrphanedMarkers_WellFormedBlockIsNotOrphaned(t *testing.T) {
	if orphans := FindOrphanedMarkers(newFormatConfig); len(orphans) != 0 {
		t.Errorf("expected no orphans in a well-formed config, got %+v", orphans)
	}
}

// A duplicated/orphaned marker must be reported and removable by clean-ssh
// even when the workspace/container it names is still perfectly alive — the
// bug this guards against is exactly that: an orphan for a live target was
// silently ignored because staleness was judged purely by target liveness.
func TestPruneManagedBlocks_OrphanedDuplicateRemovedEvenWhenTargetAlive(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
	content := "# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"\n" +
		"# devcontainer-cli:managed v=1 kind=workspace ref=myproj alias=myproj\n" +
		"Host myproj\n" +
		"    HostName 172.20.0.2\n" +
		"    User devuser\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	alwaysAlive := func(string) bool { return true }
	alwaysAliveRemote := func(string, string) (bool, bool) { return true, true }

	stale, _, backup, err := svc.PruneManagedBlocks(alwaysAlive, alwaysAlive, alwaysAliveRemote, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 1 || stale[0].Ref != "myproj" {
		t.Fatalf("stale = %+v, want exactly one entry for myproj", stale)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("expected backup at %s: %v", backup, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "myproj") {
		t.Errorf("expected the whole duplicated/orphaned entry gone, got:\n%s", data)
	}
}

// A --via block's container lives on a different Docker daemon: it must be
// checked with existsRemoteContainer (keyed by marker.Host), not the local
// existsContainer, or clean-ssh treats every --via entry as stale on sight.
func TestPruneManagedBlocks_ViaBlockCheckedAgainstRemoteHost(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
	content := "# devcontainer-cli:managed v=1 kind=container ref=remote-ctr alias=remote-ctr host=rbpi\n" +
		"Host remote-ctr\n" +
		"    HostName 172.20.0.5\n" +
		"    ProxyJump rbpi\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	localNeverHasIt := func(string) bool { return false }
	remoteHasIt := func(host, name string) (bool, bool) { return host == "rbpi" && name == "remote-ctr", true }

	// The container is invisible locally but alive on rbpi: must not be stale.
	stale, unverified, _, err := svc.PruneManagedBlocks(localNeverHasIt, localNeverHasIt, remoteHasIt, true)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("--via block wrongly flagged stale despite existing on its remote host: %+v", stale)
	}
	if len(unverified) != 0 {
		t.Errorf("expected no unverified blocks when the remote host is reachable: %+v", unverified)
	}

	// Once it's gone from rbpi too, it must be reported stale.
	remoteGone := func(string, string) (bool, bool) { return false, true }
	stale, _, _, err = svc.PruneManagedBlocks(localNeverHasIt, localNeverHasIt, remoteGone, true)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 1 || stale[0].Ref != "remote-ctr" {
		t.Fatalf("stale = %+v, want [remote-ctr] once gone from its remote host too", stale)
	}
}

// A --via block whose remote host can't be reached at all must not be reported
// stale (it might still be perfectly valid) nor silently disappear from
// clean-ssh's output: it comes back as unverified so the user can still choose
// to remove it, e.g. after they've renamed or deleted the underlying ssh
// connection to that host.
func TestPruneManagedBlocks_UnreachableViaHostIsUnverifiedNotStale(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
	content := "# devcontainer-cli:managed v=1 kind=container ref=remote-ctr alias=remote-ctr host=rbpi\n" +
		"Host remote-ctr\n" +
		"    User devuser\n" +
		"    ProxyCommand ssh rbpi \"nc -q0 172.20.0.5 22\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	localNeverHasIt := func(string) bool { return false }
	unreachable := func(string, string) (bool, bool) { return true, false }

	stale, unverified, backup, err := svc.PruneManagedBlocks(localNeverHasIt, localNeverHasIt, unreachable, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected nothing auto-removed for an unreachable --via host, got %+v", stale)
	}
	if backup != "" {
		t.Errorf("expected no write/backup when nothing was removed, got %q", backup)
	}
	if len(unverified) != 1 || unverified[0].Ref != "remote-ctr" || unverified[0].Host != "rbpi" {
		t.Fatalf("unverified = %+v, want [remote-ctr host=rbpi]", unverified)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Host remote-ctr") {
		t.Error("the unverified block must not have been removed from the file")
	}
}

func TestPruneManagedBlocksNoStale(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
	if err := os.WriteFile(path, []byte(newFormatConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := SshService{Report: nopReporter{}}
	alive := func(string) bool { return true }
	aliveRemote := func(string, string) (bool, bool) { return true, true }
	stale, _, backup, err := svc.PruneManagedBlocks(alive, alive, aliveRemote, false)
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
	aliveRemote := func(string, string) (bool, bool) { return true, true }
	stale, _, _, err := svc.PruneManagedBlocks(alive, alive, aliveRemote, false)
	if err != nil {
		t.Fatalf("PruneManagedBlocks (missing file): %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected empty result for missing config, got %+v", stale)
	}
	if _, err := os.Stat(managedSSHConfig(home)); !os.IsNotExist(err) {
		t.Error("PruneManagedBlocks must not create ~/.ssh/config")
	}
}

func TestRemoveManagedBlocksSubset(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := managedSSHConfig(home)
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
	path := managedSSHConfig(home)
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
	if err := os.WriteFile(managedSSHConfig(home), []byte(sampleConfig), 0o600); err != nil {
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
	configPath := managedSSHConfig(home)
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
	if err := os.WriteFile(managedSSHConfig(home), []byte(sampleConfig), 0o600); err != nil {
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

func TestConfigHostTargets(t *testing.T) {
	// Aliases and HostNames both count: a block without HostName dials its alias.
	// Wildcard patterns name no single host and are skipped.
	want := []string{"devcontainer", "172.18.0.2", "other", "example.com"}
	got := ConfigHostTargets(sampleConfig)
	if len(got) != len(want) {
		t.Fatalf("ConfigHostTargets() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("ConfigHostTargets()[%d] = %q, want %q", i, got[i], w)
		}
	}
	for _, unwanted := range []string{"*.internal"} {
		if slices.Contains(got, unwanted) {
			t.Errorf("ConfigHostTargets() should skip wildcard %q: %v", unwanted, got)
		}
	}
}

func TestConfigHostTargetsDeduplicates(t *testing.T) {
	content := "Host a\n    HostName 1.2.3.4\n\nHost b\n    HostName 1.2.3.4\n"
	if got, want := len(ConfigHostTargets(content)), 3; got != want {
		t.Errorf("ConfigHostTargets() returned %d targets, want %d (a, b, 1.2.3.4)", got, want)
	}
}

func TestManagedHostName(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := "# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh\n" +
		"Host dc-ssh\n    HostName 172.25.2.30\n"
	if err := os.WriteFile(managedSSHConfig(home), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	cases := []struct {
		kind sshdefaults.Kind
		ref  string
		want string
	}{
		// The alias is not the ref, so the lookup must go through the marker.
		{sshdefaults.KindWorkspace, "api-3f9a", "172.25.1.30"},
		{sshdefaults.KindContainer, "dc-ssh", "172.25.2.30"},
		{sshdefaults.KindWorkspace, "gone", ""},
	}
	for _, c := range cases {
		got, err := svc.ManagedHostName(c.kind, c.ref)
		if err != nil || got != c.want {
			t.Errorf("ManagedHostName(%s,%s) = %q,%v, want %q,nil", c.kind, c.ref, got, err, c.want)
		}
	}
}

func TestManagedHostNameMissingConfig(t *testing.T) {
	setHomeDir(t, t.TempDir())
	svc := SshService{Report: nopReporter{}}
	got, err := svc.ManagedHostName(sshdefaults.KindWorkspace, "api")
	if err != nil || got != "" {
		t.Errorf("ManagedHostName with no config = %q,%v, want \"\",nil", got, err)
	}
}

func TestManagedViaHost(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A local block (records no host=) and a --via block (host=rbpi) whose
	// ProxyCommand stanza has no HostName.
	cfg := "# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh host=rbpi\n" +
		"Host dc-ssh\n    User devuser\n    ProxyCommand ssh rbpi \"nc -q0 172.25.2.30 22\"\n"
	if err := os.WriteFile(managedSSHConfig(home), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	cases := []struct {
		alias string
		want  string
	}{
		{"dc-ssh", "rbpi"}, // --via block: its jump target is recovered from the marker
		{"api", ""},        // local block: no host= recorded
		{"gone", ""},       // unknown alias
	}
	for _, c := range cases {
		got, err := svc.ManagedViaHost(c.alias)
		if err != nil || got != c.want {
			t.Errorf("ManagedViaHost(%s) = %q,%v, want %q,nil", c.alias, got, err, c.want)
		}
	}
}

func TestManagedViaHostMissingConfig(t *testing.T) {
	setHomeDir(t, t.TempDir())
	svc := SshService{Report: nopReporter{}}
	got, err := svc.ManagedViaHost("dc-ssh")
	if err != nil || got != "" {
		t.Errorf("ManagedViaHost with no config = %q,%v, want \"\",nil", got, err)
	}
}

func TestSetManagedHostName(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A local block that dials by IP, and a --via block that dials by ProxyCommand
	// (no HostName). Only the first is refreshable.
	cfg := "# devcontainer-cli:managed v=1 kind=workspace ref=api-3f9a alias=api\n" +
		"Host api\n    HostName 172.25.1.30\n    User devuser\n\n" +
		"# devcontainer-cli:managed v=1 kind=container ref=dc-ssh alias=dc-ssh host=rbpi\n" +
		"Host dc-ssh\n    User devuser\n    ProxyCommand ssh rbpi \"nc -q0 172.25.2.30 22\"\n"
	path := managedSSHConfig(home)
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}

	changed, err := svc.SetManagedHostName("api", "172.25.1.99")
	if err != nil || !changed {
		t.Fatalf("SetManagedHostName(api) = %v,%v, want true,nil", changed, err)
	}
	if got := HostNameForAlias(readFile(t, path), "api"); got != "172.25.1.99" {
		t.Errorf("HostName after update = %q, want 172.25.1.99", got)
	}

	// Idempotent: setting the same address changes nothing.
	if changed, err := svc.SetManagedHostName("api", "172.25.1.99"); err != nil || changed {
		t.Errorf("SetManagedHostName(api, same) = %v,%v, want false,nil", changed, err)
	}

	// A ProxyCommand block has no HostName line; it must not gain one.
	if changed, err := svc.SetManagedHostName("dc-ssh", "10.0.0.9"); err != nil || changed {
		t.Errorf("SetManagedHostName(dc-ssh) = %v,%v, want false,nil (no HostName to update)", changed, err)
	}
	if got := HostNameForAlias(readFile(t, path), "dc-ssh"); got != "" {
		t.Errorf("dc-ssh gained a HostName %q, want none", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHostIsReferenced(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedSSHConfig(home), []byte(sampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	cases := []struct {
		name string
		host string
		want bool
	}{
		{name: "a HostName", host: "172.18.0.2", want: true},
		{name: "an alias, which a block without HostName dials", host: "devcontainer", want: true},
		{name: "an address no block reaches", host: "172.99.0.1", want: false},
		{name: "no host at all", host: "", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := svc.HostIsReferenced(c.host)
			if err != nil || got != c.want {
				t.Errorf("HostIsReferenced(%q) = %v,%v, want %v,nil", c.host, got, err, c.want)
			}
		})
	}
}

// ── Separate managed config file ────────────────────────────────────────────

func TestEnsureIncludeCreatesUserConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}

	added, err := svc.EnsureInclude()
	if err != nil || !added {
		t.Fatalf("EnsureInclude = %v,%v, want true,nil", added, err)
	}
	data, err := os.ReadFile(userSSHConfig(home))
	if err != nil {
		t.Fatalf("expected ~/.ssh/config to be created: %v", err)
	}
	// A managed file inside ~/.ssh is referenced relatively, the way OpenSSH
	// resolves relative includes.
	if !strings.Contains(string(data), "Include "+types.SSHConfigName) {
		t.Errorf("missing Include directive:\n%s", data)
	}
	if !strings.Contains(string(data), includeMarker) {
		t.Errorf("Include must carry the managed marker:\n%s", data)
	}

	// Idempotent: a second run changes nothing.
	added, err = svc.EnsureInclude()
	if err != nil || added {
		t.Errorf("second EnsureInclude = %v,%v, want false,nil", added, err)
	}
	again, _ := os.ReadFile(userSSHConfig(home))
	if string(again) != string(data) {
		t.Errorf("second EnsureInclude rewrote the config:\n%s", again)
	}
}

func TestEnsureIncludeGoesAboveHostBlocks(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	existing := "ServerAliveInterval 30\n\nHost mine\n    HostName example.com\n"
	if err := os.WriteFile(userSSHConfig(home), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	if added, err := svc.EnsureInclude(); err != nil || !added {
		t.Fatalf("EnsureInclude = %v,%v, want true,nil", added, err)
	}

	data, _ := os.ReadFile(userSSHConfig(home))
	got := string(data)
	// OpenSSH keeps the FIRST value obtained for each keyword, so the Include
	// must precede every Host/Match block or it would be shadowed.
	inc, host := strings.Index(got, "Include "), strings.Index(got, "Host mine")
	if inc == -1 || host == -1 || inc > host {
		t.Errorf("Include (%d) must come before the first Host block (%d):\n%s", inc, host, got)
	}
	// The user's own content survives untouched, and is backed up.
	if !strings.Contains(got, "ServerAliveInterval 30") || !strings.Contains(got, "HostName example.com") {
		t.Errorf("user config content was lost:\n%s", got)
	}
	backup, err := os.ReadFile(userSSHConfig(home) + ".bak")
	if err != nil || string(backup) != existing {
		t.Errorf("expected the original to be backed up, got %q (%v)", backup, err)
	}
}

func TestEnsureIncludeAcceptsExistingSpellings(t *testing.T) {
	for _, spelling := range []string{
		"Include " + types.SSHConfigName,
		"Include ~/.ssh/" + types.SSHConfigName,
		"include " + types.SSHConfigName,
	} {
		t.Run(spelling, func(t *testing.T) {
			home := t.TempDir()
			setHomeDir(t, home)
			if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(userSSHConfig(home), []byte(spelling+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			svc := SshService{Report: nopReporter{}}
			added, err := svc.EnsureInclude()
			if err != nil || added {
				t.Errorf("EnsureInclude = %v,%v, want false,nil (already included as %q)", added, err, spelling)
			}
		})
	}
}

func TestEnsureIncludeUsesAbsolutePathOutsideSSHDir(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	custom := filepath.Join(home, "elsewhere", "dc.config")
	svc := ConfigService{Report: nopReporter{}}
	if err := svc.SetGlobalDefault("ssh-config-file", custom); err != nil {
		t.Fatal(err)
	}

	ssh := SshService{Report: nopReporter{}}
	if added, err := ssh.EnsureInclude(); err != nil || !added {
		t.Fatalf("EnsureInclude = %v,%v, want true,nil", added, err)
	}
	data, _ := os.ReadFile(userSSHConfig(home))
	if !strings.Contains(string(data), "Include "+custom) {
		t.Errorf("a managed file outside ~/.ssh needs an absolute Include:\n%s", data)
	}
}

// insertBeforeFirstStanza carries EnsureInclude's whole placement rule, so it is
// tested directly on strings rather than only through the filesystem.
func TestInsertBeforeFirstStanza(t *testing.T) {
	const block = "# marker\nInclude devcontainer-cli.config\n"
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no stanza at all appends at the end",
			content: "# just a comment\nServerAliveInterval 60\n",
			want:    "# just a comment\nServerAliveInterval 60\n" + block + "\n",
		},
		{
			name:    "stanza on the first line stays below the block",
			content: "Host example\n    HostName example.com\n",
			want:    block + "\nHost example\n    HostName example.com\n",
		},
		{
			// The blank separator line becomes the newline terminating the option
			// above it, so the block still lands between the two.
			name:    "leading options are preserved above the block",
			content: "AddKeysToAgent yes\n\nHost example\n    HostName example.com\n",
			want:    "AddKeysToAgent yes\n" + block + "\nHost example\n    HostName example.com\n",
		},
		{
			name:    "Match opens a stanza just like Host",
			content: "Match host example\n    User dev\n",
			want:    block + "\nMatch host example\n    User dev\n",
		},
		{
			name:    "content without a trailing newline still gets separated",
			content: "AddKeysToAgent yes",
			want:    "AddKeysToAgent yes\n" + block + "\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := insertBeforeFirstStanza(c.content, block); got != c.want {
				t.Errorf("insertBeforeFirstStanza:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

// planBlockMigration is the pure core of MigrateManagedBlocks: it decides both
// files' new content, which is what lets the write ordering above it stay a
// three-line concern.
func TestPlanBlockMigration(t *testing.T) {
	userContent := "Host handwritten\n    HostName example.com\n\n" + managedContainerConfig
	stale := ListManagedBlocks(userContent)
	if len(stale) == 0 {
		t.Fatalf("fixture has no managed blocks to move")
	}

	plan := planBlockMigration(userContent, "", stale)

	if len(plan.moved) != 1 || plan.moved[0].Ref != "dc-ssh" {
		t.Fatalf("moved = %v, want the dc-ssh block", plan.moved)
	}
	if strings.Contains(plan.userContent, "dc-ssh") {
		t.Errorf("managed block left in the user content:\n%s", plan.userContent)
	}
	if !strings.Contains(plan.userContent, "Host handwritten") {
		t.Errorf("plan dropped a block it does not own:\n%s", plan.userContent)
	}
	if !strings.Contains(plan.managedContent, "Host dc-ssh") {
		t.Errorf("block did not land in the managed content:\n%s", plan.managedContent)
	}
	// The marker is re-rendered in the current spelling rather than carried over.
	if !strings.Contains(plan.managedContent, "kind=container") || !strings.Contains(plan.managedContent, "ref=dc-ssh") {
		t.Errorf("managed content lacks a current-form marker:\n%s", plan.managedContent)
	}
	if !strings.HasSuffix(plan.managedContent, "\n") {
		t.Errorf("managed content must end with a newline, got %q", plan.managedContent)
	}
}

// An existing managed file is appended to, not overwritten.
func TestPlanBlockMigrationKeepsExistingManagedBlocks(t *testing.T) {
	existing := "# devcontainer-cli:managed v=1 kind=workspace ref=other alias=other\nHost other-ws\n    HostName 172.18.0.9\n"
	userContent := managedContainerConfig
	plan := planBlockMigration(userContent, existing, ListManagedBlocks(userContent))

	if !strings.Contains(plan.managedContent, "Host other-ws") {
		t.Errorf("migration dropped an already-managed block:\n%s", plan.managedContent)
	}
	if !strings.Contains(plan.managedContent, "Host dc-ssh") {
		t.Errorf("migration did not append the moved block:\n%s", plan.managedContent)
	}
}

// A marker whose stanza is gone is dropped rather than written back as an empty
// block.
func TestPlanBlockMigrationDropsOrphanedMarker(t *testing.T) {
	orphan := "# devcontainer-cli:managed v=1 kind=workspace ref=gone alias=gone\n"
	plan := planBlockMigration(orphan, "", []ManagedMarker{{Kind: "workspace", Ref: "gone", Alias: "gone"}})

	if len(plan.moved) != 0 {
		t.Errorf("moved = %v, want nothing for an orphaned marker", plan.moved)
	}
	if strings.Contains(plan.managedContent, "gone") {
		t.Errorf("orphaned marker was written into the managed config:\n%s", plan.managedContent)
	}
}

func TestMigrateManagedBlocks(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := "Host handwritten\n    HostName example.com\n\n" + managedContainerConfig
	if err := os.WriteFile(userSSHConfig(home), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}
	moved, err := svc.MigrateManagedBlocks()
	if err != nil {
		t.Fatalf("MigrateManagedBlocks: %v", err)
	}
	if len(moved) != 1 || moved[0].Ref != "dc-ssh" {
		t.Fatalf("MigrateManagedBlocks = %v, want the dc-ssh block", moved)
	}

	user, _ := os.ReadFile(userSSHConfig(home))
	if strings.Contains(string(user), "dc-ssh") || strings.Contains(string(user), "devcontainer-cli:managed") {
		t.Errorf("managed block left behind in the user config:\n%s", user)
	}
	// Blocks the user wrote by hand are never touched.
	if !strings.Contains(string(user), "Host handwritten") || !strings.Contains(string(user), "Host other") {
		t.Errorf("migration removed blocks it does not own:\n%s", user)
	}
	// The original is backed up before the rewrite.
	if backup, err := os.ReadFile(userSSHConfig(home) + ".bak"); err != nil || string(backup) != legacy {
		t.Errorf("expected the original user config to be backed up, got %q (%v)", backup, err)
	}

	managed, err := os.ReadFile(managedSSHConfig(home))
	if err != nil {
		t.Fatalf("expected the managed config to be written: %v", err)
	}
	if !strings.Contains(string(managed), "Host dc-ssh") || !strings.Contains(string(managed), "172.18.0.5") {
		t.Errorf("block did not land in the managed config:\n%s", managed)
	}
	// The block stays findable by its marker after the move.
	if alias, ok, _ := svc.ManagedAlias(sshdefaults.KindContainer, "dc-ssh"); !ok || alias != "dc-ssh" {
		t.Errorf("ManagedAlias after migration = %q,%v, want dc-ssh,true", alias, ok)
	}

	// Idempotent: nothing left to move.
	moved, err = svc.MigrateManagedBlocks()
	if err != nil || len(moved) != 0 {
		t.Errorf("second MigrateManagedBlocks = %v,%v, want empty,nil", moved, err)
	}
}

func TestMigrateManagedBlocksNoConfig(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	svc := SshService{Report: nopReporter{}}
	moved, err := svc.MigrateManagedBlocks()
	if err != nil || len(moved) != 0 {
		t.Fatalf("MigrateManagedBlocks with no config = %v,%v, want empty,nil", moved, err)
	}
	// A no-op migration must not create either file.
	if _, err := os.Stat(managedSSHConfig(home)); !os.IsNotExist(err) {
		t.Error("migration must not create the managed config when there is nothing to move")
	}
}

func TestFindHostAliasConflict(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedSSHConfig(home), []byte("Host proj\n    HostName 10.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userSSHConfig(home), []byte("Host mine\n    HostName example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}

	got, err := svc.FindHostAliasConflict("proj")
	if err != nil || got == nil || !got.Managed || got.Path != managedSSHConfig(home) {
		t.Fatalf("FindHostAliasConflict(proj) = %+v,%v, want the managed config", got, err)
	}
	if !strings.Contains(got.Block, "10.0.0.1") {
		t.Errorf("expected the existing stanza for display, got %q", got.Block)
	}

	// A collision with the user's own config must be reported too — otherwise
	// the CLI would silently shadow a Host they maintain themselves.
	got, err = svc.FindHostAliasConflict("mine")
	if err != nil || got == nil || got.Managed || got.Path != userSSHConfig(home) {
		t.Fatalf("FindHostAliasConflict(mine) = %+v,%v, want the user config", got, err)
	}

	if got, err := svc.FindHostAliasConflict("free"); err != nil || got != nil {
		t.Errorf("FindHostAliasConflict(free) = %+v,%v, want nil,nil", got, err)
	}
}

// The read-only queries that guard destructive work must see the user's own
// hand-written blocks, not just the managed file.
func TestDualConfigReads(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedSSHConfig(home), []byte("Host proj\n    HostName 10.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userSSHConfig(home), []byte("Host mine\n    HostName 10.0.0.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := SshService{Report: nopReporter{}}

	// A key still dialed by the user's own block must not be treated as orphaned.
	for _, host := range []string{"10.0.0.1", "10.0.0.2"} {
		if got, err := svc.HostIsReferenced(host); err != nil || !got {
			t.Errorf("HostIsReferenced(%s) = %v,%v, want true,nil", host, got, err)
		}
	}

	if got, err := svc.AliasHostName("mine"); err != nil || got != "10.0.0.2" {
		t.Errorf("AliasHostName(mine) = %q,%v, want 10.0.0.2,nil", got, err)
	}

	aliases := svc.ConfigHostAliasesFromDisk()
	for _, want := range []string{"proj", "mine"} {
		if !slices.Contains(aliases, want) {
			t.Errorf("completion must offer %q, got %v", want, aliases)
		}
	}
}
