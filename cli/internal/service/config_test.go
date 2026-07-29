package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain/types"
)

func TestConfigGlobalDefaultsRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	value, customized, err := svc.GetGlobalDefault("registry")
	if err != nil || customized || value != domain.DefaultRegistry {
		t.Fatalf("fresh registry = %q customized=%v err=%v; want default uncustomized", value, customized, err)
	}

	if err := svc.SetGlobalDefault("registry", "ghcr.io/me/"); err != nil {
		t.Fatalf("SetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("registry")
	if value != "ghcr.io/me/" || !customized {
		t.Errorf("after set: value=%q customized=%v", value, customized)
	}

	if err := svc.UnsetGlobalDefault("registry"); err != nil {
		t.Fatalf("UnsetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("registry")
	if value != domain.DefaultRegistry || customized {
		t.Errorf("after unset: value=%q customized=%v", value, customized)
	}
}

func TestConfigSSHKeyRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	value, customized, err := svc.GetGlobalDefault("ssh-key")
	if err != nil || customized || value != domain.DefaultManagedSSHKeyPath() {
		t.Fatalf("fresh ssh-key = %q customized=%v err=%v; want managed default uncustomized", value, customized, err)
	}

	if err := svc.SetGlobalDefault("ssh-key", "/custom/key"); err != nil {
		t.Fatalf("SetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("ssh-key")
	if value != "/custom/key" || !customized {
		t.Errorf("after set: value=%q customized=%v", value, customized)
	}

	if err := svc.UnsetGlobalDefault("ssh-key"); err != nil {
		t.Fatalf("UnsetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("ssh-key")
	if value != domain.DefaultManagedSSHKeyPath() || customized {
		t.Errorf("after unset: value=%q customized=%v", value, customized)
	}
}

func TestConfigSSHConfigFileRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	value, customized, err := svc.GetGlobalDefault("ssh-config-file")
	if err != nil || customized || value != domain.DefaultManagedSSHConfigPath() {
		t.Fatalf("fresh ssh-config-file = %q customized=%v err=%v; want managed default uncustomized", value, customized, err)
	}

	if err := svc.SetGlobalDefault("ssh-config-file", "/custom/ssh.config"); err != nil {
		t.Fatalf("SetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("ssh-config-file")
	if value != "/custom/ssh.config" || !customized {
		t.Errorf("after set: value=%q customized=%v", value, customized)
	}
	// The resolver must honour the stored value, not just the getter.
	if got := domain.ResolveSSHConfigPath(""); got != "/custom/ssh.config" {
		t.Errorf("ResolveSSHConfigPath = %q, want the configured value", got)
	}
	// An explicit override still wins over the stored value.
	if got := domain.ResolveSSHConfigPath("/flag/path"); got != "/flag/path" {
		t.Errorf("ResolveSSHConfigPath(flag) = %q, want the flag override", got)
	}

	if err := svc.UnsetGlobalDefault("ssh-config-file"); err != nil {
		t.Fatalf("UnsetGlobalDefault: %v", err)
	}
	value, customized, _ = svc.GetGlobalDefault("ssh-config-file")
	if value != domain.DefaultManagedSSHConfigPath() || customized {
		t.Errorf("after unset: value=%q customized=%v", value, customized)
	}
}

func TestConfigAliasFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	svc := ConfigService{Report: nopReporter{}}

	// The host path must be the host side of the shared-config entry, so
	// `config shared sync alias.sh` pushes exactly this file.
	entry, ok := types.SharedConfigEntryByID(types.SharedConfigAliasID)
	if !ok {
		t.Fatal("the alias.sh shared-config entry must exist")
	}
	wantPath := filepath.Join(home, entry.Target)
	if got := svc.AliasFilePath(); got != wantPath {
		t.Errorf("AliasFilePath = %q, want %q", got, wantPath)
	}

	// Missing file: no error, exists=false.
	path, content, exists, err := svc.ReadAliasFile()
	if err != nil || exists || content != "" || path != wantPath {
		t.Fatalf("fresh ReadAliasFile = (%q, %q, %v, %v)", path, content, exists, err)
	}

	// EnsureAliasFile seeds the template.
	path, created, err := svc.EnsureAliasFile()
	if err != nil || !created || path != wantPath {
		t.Fatalf("EnsureAliasFile = (%q, %v, %v)", path, created, err)
	}
	_, content, exists, _ = svc.ReadAliasFile()
	if !exists || !strings.Contains(content, "devcontainer-cli config shared sync alias.sh") {
		t.Errorf("seeded alias file should document the sync command, got:\n%s", content)
	}

	// An existing file is never overwritten by EnsureAliasFile.
	if err := os.WriteFile(wantPath, []byte("alias mine=yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, created, err = svc.EnsureAliasFile(); err != nil || created {
		t.Errorf("EnsureAliasFile on an existing file: created=%v err=%v", created, err)
	}
	if _, content, _, _ = svc.ReadAliasFile(); content != "alias mine=yes\n" {
		t.Errorf("EnsureAliasFile must not touch existing content, got %q", content)
	}

	// ResetAliasFile does overwrite it.
	if _, err := svc.ResetAliasFile(); err != nil {
		t.Fatalf("ResetAliasFile: %v", err)
	}
	if _, content, _, _ = svc.ReadAliasFile(); strings.Contains(content, "alias mine=yes") {
		t.Errorf("ResetAliasFile must discard the old content, got:\n%s", content)
	}
}

func TestConfigUnknownKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}
	if _, _, err := svc.GetGlobalDefault("nope"); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestConfigParseYAMLValidation(t *testing.T) {
	svc := ConfigService{Report: nopReporter{}}

	if _, err := svc.ParseConfigYAML([]byte("workspace: myws\n")); err != nil {
		t.Errorf("valid yaml rejected: %v", err)
	}
	if _, err := svc.ParseConfigYAML([]byte("workspace: \"bad name!!\"\n")); err == nil {
		t.Error("expected invalid workspace name to be rejected")
	}
	if _, err := svc.ParseConfigYAML([]byte("::: not yaml :::")); err == nil {
		t.Error("expected invalid yaml to be rejected")
	}
}

func TestConfigExportYAML(t *testing.T) {
	tmp := t.TempDir()
	svc := ConfigService{Report: nopReporter{}}

	cfg := &types.DevcontainerConfig{Workspace: "myws", Mode: types.BuildModeLocalCached}
	if err := svc.SaveProjectConfig(tmp, cfg); err != nil {
		t.Fatalf("SaveProjectConfig: %v", err)
	}
	data, err := svc.ExportConfigYAML(tmp)
	if err != nil {
		t.Fatalf("ExportConfigYAML: %v", err)
	}
	if !strings.Contains(string(data), "myws") {
		t.Errorf("exported yaml missing workspace: %s", data)
	}

	if _, err := svc.ExportConfigYAML(t.TempDir()); err == nil {
		t.Error("expected error exporting from a dir with no config")
	}
}
