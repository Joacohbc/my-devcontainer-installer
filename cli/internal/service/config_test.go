package service

import (
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

func TestConfigSSHPortValidation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	svc := ConfigService{Report: nopReporter{}}

	if err := svc.SetGlobalDefault("ssh-port", "9000"); err != nil {
		t.Fatalf("valid port: %v", err)
	}
	if v, _, _ := svc.GetGlobalDefault("ssh-port"); v != "9000" {
		t.Errorf("ssh-port = %q, want 9000", v)
	}
	for _, bad := range []string{"abc", "0", "70000"} {
		if err := svc.SetGlobalDefault("ssh-port", bad); err == nil {
			t.Errorf("expected error for invalid port %q", bad)
		}
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
