package domain_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestLoadGlobalConfig_returnsDefaultWhenMissing(t *testing.T) {
	withTempXDGDir(t, func() {
		cfg := domain.LoadGlobalConfig()
		if cfg.Registry != "" {
			t.Errorf("expected empty registry, got %q", cfg.Registry)
		}
	})
}

func TestSaveAndLoadGlobalConfig_roundtrips(t *testing.T) {
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
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
	withTempXDGDir(t, func() {
		dir := domain.GlobalConfigDir()
		path := domain.GlobalConfigPath()
		expected := filepath.Join(dir, "config.json")
		if path != expected {
			t.Errorf("expected path %q, got %q", expected, path)
		}
	})
}

func TestResolveRegistry_flagOverrideTakesPriority(t *testing.T) {
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io/"})
		result := domain.ResolveRegistry("flag-override.io", "per-project.io")
		if result != "flag-override.io/" {
			t.Errorf("expected flag override to win, got %q", result)
		}
	})
}

func TestResolveRegistry_perProjectFallsBackFromFlag(t *testing.T) {
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io/"})
		result := domain.ResolveRegistry("", "per-project.io")
		if result != "per-project.io/" {
			t.Errorf("expected per-project to win over global, got %q", result)
		}
	})
}

func TestResolveRegistry_globalConfigFallback(t *testing.T) {
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io"})
		result := domain.ResolveRegistry("", "")
		if result != "global.io/" {
			t.Errorf("expected global config registry with trailing slash, got %q", result)
		}
	})
}

func TestResolveRegistry_defaultWhenNothingSet(t *testing.T) {
	withTempXDGDir(t, func() {
		result := domain.ResolveRegistry("", "")
		if result != domain.DefaultRegistry {
			t.Errorf("expected default registry %q, got %q", domain.DefaultRegistry, result)
		}
	})
}

func TestResolveRegistry_appendsTrailingSlash(t *testing.T) {
	withTempXDGDir(t, func() {
		result := domain.ResolveRegistry("example.io/prefix", "")
		if result != "example.io/prefix/" {
			t.Errorf("expected trailing slash appended, got %q", result)
		}
	})
}

func TestResolveDBCredentials(t *testing.T) {
	// Case 1: Empty / no config
	withTempXDGDir(t, func() {
		user, pass := domain.ResolveDBCredentials()
		if user != "devuser" || pass != "devpass" {
			t.Errorf("expected devuser/devpass, got %s/%s", user, pass)
		}
	})

	// Case 2: Config without defaults
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{Registry: "global.io/"})
		user, pass := domain.ResolveDBCredentials()
		if user != "devuser" || pass != "devpass" {
			t.Errorf("expected devuser/devpass, got %s/%s", user, pass)
		}
	})

	// Case 3: Partial config (only user)
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{
			Defaults: &domain.Defaults{DBUser: "alice"},
		})
		user, pass := domain.ResolveDBCredentials()
		if user != "alice" || pass != "devpass" {
			t.Errorf("expected alice/devpass, got %s/%s", user, pass)
		}
	})

	// Case 4: Full config
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{
			Defaults: &domain.Defaults{DBUser: "alice", DBPassword: "password123"},
		})
		user, pass := domain.ResolveDBCredentials()
		if user != "alice" || pass != "password123" {
			t.Errorf("expected alice/password123, got %s/%s", user, pass)
		}
	})
}

func TestDefaultManagedSSHKeyPath(t *testing.T) {
	withTempXDGDir(t, func() {
		got := domain.DefaultManagedSSHKeyPath()
		want := filepath.Join(domain.GlobalConfigDir(), "ssh", "id_devcontainer")
		if got != want {
			t.Errorf("DefaultManagedSSHKeyPath() = %q, want %q", got, want)
		}
	})
}

func TestResolveSSHKeyPath(t *testing.T) {
	// Default managed path when nothing is set.
	withTempXDGDir(t, func() {
		got := domain.ResolveSSHKeyPath("")
		if got != domain.DefaultManagedSSHKeyPath() {
			t.Errorf("ResolveSSHKeyPath(\"\") = %q, want managed default %q", got, domain.DefaultManagedSSHKeyPath())
		}
	})

	// Configured override wins over the default.
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{
			Defaults: &domain.Defaults{SSHKeyPath: "/custom/key"},
		})
		if got := domain.ResolveSSHKeyPath(""); got != "/custom/key" {
			t.Errorf("ResolveSSHKeyPath(\"\") = %q, want configured /custom/key", got)
		}
	})

	// Flag override wins over everything.
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{
			Defaults: &domain.Defaults{SSHKeyPath: "/custom/key"},
		})
		if got := domain.ResolveSSHKeyPath("/flag/key"); got != "/flag/key" {
			t.Errorf("ResolveSSHKeyPath(\"/flag/key\") = %q, want flag override", got)
		}
	})
}

func TestManagedKnownHostsPath(t *testing.T) {
	withTempXDGDir(t, func() {
		got := domain.ManagedKnownHostsPath()
		want := filepath.Join(domain.GlobalConfigDir(), "ssh", "known_hosts")
		if got != want {
			t.Errorf("ManagedKnownHostsPath() = %q, want %q", got, want)
		}
	})
}

// The known_hosts file is CLI bookkeeping, not user key material: unlike the key
// path it must not follow a configured SSHKeyPath override, or the CLI would
// re-pin keys in a file the generated Host blocks never reference.
func TestManagedKnownHostsPathIgnoresKeyOverride(t *testing.T) {
	withTempXDGDir(t, func() {
		_ = domain.SaveGlobalConfig(domain.GlobalConfig{
			Defaults: &domain.Defaults{SSHKeyPath: "/custom/key"},
		})
		if got, want := domain.ManagedKnownHostsPath(), filepath.Join(domain.GlobalConfigDir(), "ssh", "known_hosts"); got != want {
			t.Errorf("ManagedKnownHostsPath() = %q, want %q", got, want)
		}
	})
}

func TestIsValidAliasName(t *testing.T) {
	valid := []string{"ll", "g", "gco", "k8s", "my_alias", "my-alias", "a.b", "_x"}
	for _, name := range valid {
		if !domain.IsValidAliasName(name) {
			t.Errorf("IsValidAliasName(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "1abc", "has space", "a=b", "a/b", "a$b", "a'b", "-x"}
	for _, name := range invalid {
		if domain.IsValidAliasName(name) {
			t.Errorf("IsValidAliasName(%q) = true, want false", name)
		}
	}
}

func TestRenderUserAliases(t *testing.T) {
	// Empty map still yields a valid, sourceable header-only script (comments
	// only, no actual `alias` statement).
	empty := domain.RenderUserAliases(nil)
	if !strings.HasPrefix(empty, "#!/bin/sh") {
		t.Errorf("rendered script must start with a shebang:\n%s", empty)
	}
	for _, line := range strings.Split(empty, "\n") {
		if strings.HasPrefix(line, "alias ") {
			t.Errorf("empty aliases must render no alias statements, got %q", line)
		}
	}

	out := domain.RenderUserAliases(map[string]string{
		"ll":  "ls -la",
		"gs":  "git status",
		"say": "echo 'hi'",
	})
	// Sorted order: gs, ll, say.
	gs, ll, say := strings.Index(out, "alias gs="), strings.Index(out, "alias ll="), strings.Index(out, "alias say=")
	if gs == -1 || ll == -1 || say == -1 || !(gs < ll && ll < say) {
		t.Errorf("aliases must render in sorted order (gs<ll<say), got %d/%d/%d:\n%s", gs, ll, say, out)
	}
	if !strings.Contains(out, "alias ll='ls -la'") {
		t.Errorf("plain command must be single-quoted verbatim:\n%s", out)
	}
	// POSIX single-quote escaping: ' becomes '\''.
	if !strings.Contains(out, `alias say='echo '\''hi'\'''`) {
		t.Errorf("embedded single quotes must be escaped the POSIX way:\n%s", out)
	}
}
