package domain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func withTempConfigDir(t *testing.T, fn func()) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "dc-cli-global-config-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })

	prev, hasPrev := os.LookupEnv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() {
		if hasPrev {
			os.Setenv("XDG_CONFIG_HOME", prev)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		xdg.Reload()
	})

	fn()
}

func TestLoadGlobalConfig_returnsDefaultWhenMissing(t *testing.T) {
	withTempConfigDir(t, func() {
		cfg := domain.LoadGlobalConfig()
		if cfg.Registry != "" {
			t.Errorf("expected empty registry, got %q", cfg.Registry)
		}
	})
}

func TestSaveAndLoadGlobalConfig_roundtrips(t *testing.T) {
	withTempConfigDir(t, func() {
		original := domain.GlobalConfig{Registry: "ghcr.io/example/"}
		if err := domain.SaveGlobalConfig(original); err != nil {
			t.Fatalf("SaveGlobalConfig failed: %v", err)
		}

		loaded := domain.LoadGlobalConfig()
		if loaded.Registry != original.Registry {
			t.Errorf("expected registry %q, got %q", original.Registry, loaded.Registry)
		}
	})
}

func TestSaveGlobalConfig_createsDirectoryIfMissing(t *testing.T) {
	withTempConfigDir(t, func() {
		cfg := domain.GlobalConfig{Registry: "example.io/"}
		if err := domain.SaveGlobalConfig(cfg); err != nil {
			t.Fatalf("SaveGlobalConfig failed: %v", err)
		}

		configPath := domain.GlobalConfigPath()
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Errorf("expected config file at %s to exist", configPath)
		}
	})
}

func TestGlobalConfigPath_isInsideConfigDir(t *testing.T) {
	withTempConfigDir(t, func() {
		dir := domain.GlobalConfigDir()
		path := domain.GlobalConfigPath()
		expected := filepath.Join(dir, "config.json")
		if path != expected {
			t.Errorf("expected path %q, got %q", expected, path)
		}
	})
}

func TestResolveRegistry_flagOverrideTakesPriority(t *testing.T) {
	withTempConfigDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io/"})
		result := domain.ResolveRegistry("flag-override.io", "per-project.io")
		if result != "flag-override.io/" {
			t.Errorf("expected flag override to win, got %q", result)
		}
	})
}

func TestResolveRegistry_perProjectFallsBackFromFlag(t *testing.T) {
	withTempConfigDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io/"})
		result := domain.ResolveRegistry("", "per-project.io")
		if result != "per-project.io/" {
			t.Errorf("expected per-project to win over global, got %q", result)
		}
	})
}

func TestResolveRegistry_globalConfigFallback(t *testing.T) {
	withTempConfigDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io"})
		result := domain.ResolveRegistry("", "")
		if result != "global.io/" {
			t.Errorf("expected global config registry with trailing slash, got %q", result)
		}
	})
}

func TestResolveRegistry_defaultWhenNothingSet(t *testing.T) {
	withTempConfigDir(t, func() {
		result := domain.ResolveRegistry("", "")
		if result != domain.DefaultRegistry {
			t.Errorf("expected default registry %q, got %q", domain.DefaultRegistry, result)
		}
	})
}

func TestResolveRegistry_appendsTrailingSlash(t *testing.T) {
	withTempConfigDir(t, func() {
		result := domain.ResolveRegistry("example.io/prefix", "")
		if result != "example.io/prefix/" {
			t.Errorf("expected trailing slash appended, got %q", result)
		}
	})
}
