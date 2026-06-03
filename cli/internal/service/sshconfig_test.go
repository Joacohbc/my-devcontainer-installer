package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setHomeDir points os.UserHomeDir() at dir, using the env var honored on the
// current platform (HOME on Unix/macOS, USERPROFILE on Windows).
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		return
	}
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
