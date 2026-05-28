package domain_test

import (
	"os"
	"testing"
)

// TestMain establishes a clean XDG_CONFIG_HOME for all domain tests so that
// real config files on the developer's machine never leak into tests.
// Individual tests that need per-test isolation use withTempXDGDir on top of
// this baseline.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "dc-cli-domain-tests-")
	if err != nil {
		panic("failed to create test base dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)
	os.Setenv("XDG_CONFIG_HOME", tmp) //nolint:errcheck
	os.Setenv("APPDATA", tmp)         //nolint:errcheck
	os.Exit(m.Run())
}

// withTempXDGDir creates a fresh, empty directory and points XDG_CONFIG_HOME
// at it for the duration of fn. This isolates tests that read or write the
// global config / image registry from one another.
func withTempXDGDir(t *testing.T, fn func()) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "dc-cli-xdg-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmp) })
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)
	fn()
}
