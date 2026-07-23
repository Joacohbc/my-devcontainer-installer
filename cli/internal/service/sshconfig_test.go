package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
