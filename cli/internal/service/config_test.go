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

// Aliases are stored in the CLI config, set and unset by command, and never
// touch a file in the user's home.
func TestConfigAliasesRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	if got := svc.Aliases(); len(got) != 0 {
		t.Fatalf("fresh config must have no aliases, got %v", got)
	}

	if err := svc.SetAlias("ll", "ls -la"); err != nil {
		t.Fatalf("SetAlias: %v", err)
	}
	if err := svc.SetAlias("gs", "git status"); err != nil {
		t.Fatalf("SetAlias: %v", err)
	}
	// Overwriting an existing name keeps a single entry.
	if err := svc.SetAlias("ll", "ls -lah"); err != nil {
		t.Fatalf("SetAlias overwrite: %v", err)
	}

	got := svc.Aliases()
	if len(got) != 2 || got["ll"] != "ls -lah" || got["gs"] != "git status" {
		t.Fatalf("aliases = %v, want ll=ls -lah, gs=git status", got)
	}

	// The rendered script is deterministic (sorted) and single-quotes commands.
	rendered := svc.RenderedAliases()
	for _, frag := range []string{"alias gs='git status'", "alias ll='ls -lah'"} {
		if !strings.Contains(rendered, frag) {
			t.Errorf("rendered aliases missing %q:\n%s", frag, rendered)
		}
	}
	if strings.Index(rendered, "alias gs=") > strings.Index(rendered, "alias ll=") {
		t.Errorf("aliases must render in sorted order:\n%s", rendered)
	}

	// Nothing was written to the user's home.
	if _, err := os.Stat(filepath.Join(home, ".alias.sh")); !os.IsNotExist(err) {
		t.Errorf("aliases must not create ~/.alias.sh on the host, stat err = %v", err)
	}

	existed, err := svc.UnsetAlias("gs")
	if err != nil || !existed {
		t.Fatalf("UnsetAlias(gs) = (%v, %v), want (true, nil)", existed, err)
	}
	if existed, _ := svc.UnsetAlias("nope"); existed {
		t.Error("UnsetAlias on an absent name must report existed=false")
	}
	if got := svc.Aliases(); len(got) != 1 || got["ll"] == "" {
		t.Errorf("after unset, aliases = %v, want just ll", got)
	}
}

// An invalid alias name is rejected before anything is persisted.
func TestConfigSetAliasRejectsBadNames(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}
	for _, bad := range []string{"", "1abc", "has space", "a=b", "no/slash"} {
		if err := svc.SetAlias(bad, "echo hi"); err == nil {
			t.Errorf("SetAlias(%q) must be rejected", bad)
		}
	}
	if err := svc.SetAlias("ok", "   "); err == nil {
		t.Error("SetAlias with an empty command must be rejected")
	}
	if got := svc.Aliases(); len(got) != 0 {
		t.Errorf("no invalid alias should have been stored, got %v", got)
	}
}

// A command containing a single quote must survive the round-trip into the
// rendered POSIX script.
func TestRenderedAliasesEscapesSingleQuotes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}
	if err := svc.SetAlias("say", "echo 'hi there'"); err != nil {
		t.Fatalf("SetAlias: %v", err)
	}
	rendered := svc.RenderedAliases()
	if !strings.Contains(rendered, `alias say='echo '\''hi there'\'''`) {
		t.Errorf("single quotes must be POSIX-escaped:\n%s", rendered)
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
