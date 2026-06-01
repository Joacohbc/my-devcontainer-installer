package assets

import "testing"

func TestRegistryEntriesResolve(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatal("registry must not be empty")
	}
	for _, a := range Registry {
		if a.Name == "" || a.File == "" {
			t.Errorf("asset has empty Name or File: %+v", a)
		}
		if !AssetExists(a.File) {
			t.Errorf("registry asset %q references missing embedded file %q", a.Name, a.File)
		}
	}
}

func TestRegistryNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Registry {
		if seen[a.Name] {
			t.Errorf("duplicate asset name %q", a.Name)
		}
		seen[a.Name] = true
	}
}

func TestCopyableAssetsAreScripts(t *testing.T) {
	copyable := CopyableAssets()
	for _, a := range copyable {
		if a.Kind != KindScript {
			t.Errorf("copyable asset %q must be KindScript, got %q", a.Name, a.Kind)
		}
	}
	if len(CopyableNames()) != len(copyable) {
		t.Errorf("CopyableNames length %d != CopyableAssets length %d", len(CopyableNames()), len(copyable))
	}
}

func TestBuildOnlyAssetsAreNotCopyable(t *testing.T) {
	// Build-time-only scripts must never be offered for runtime copying.
	buildOnly := []string{"entrypoint", "golang-utils", "update-golang", "zsh-installer"}
	for _, name := range buildOnly {
		if _, ok := LookupCopyable(name); ok {
			t.Errorf("build-only asset %q must not be copyable", name)
		}
	}
	// The copyable set is exactly the runtime installers + the gh login helper.
	want := []string{
		"install-antigravity", "install-claude-code", "install-codex-cli",
		"install-copilot", "install-opencode", "login-github-cli",
	}
	if len(CopyableNames()) != len(want) {
		t.Fatalf("expected %d copyable assets, got %v", len(want), CopyableNames())
	}
	for _, name := range want {
		if _, ok := LookupCopyable(name); !ok {
			t.Errorf("expected %q to be copyable", name)
		}
	}
}

func TestLookupCopyable(t *testing.T) {
	a, ok := LookupCopyable("install-claude-code")
	if !ok {
		t.Fatal("expected to resolve install-claude-code")
	}
	if a.File != "install-claude-code.sh" {
		t.Errorf("expected install-claude-code.sh, got %q", a.File)
	}
	if _, ok := LookupCopyable("does-not-exist"); ok {
		t.Error("expected lookup of unknown asset to fail")
	}
}

func TestReadAsset(t *testing.T) {
	data, err := ReadAsset("install-claude-code.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty asset content")
	}
	if _, err := ReadAsset("nope.sh"); err == nil {
		t.Error("expected error reading missing asset")
	}
}
