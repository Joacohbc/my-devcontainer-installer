package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProgressWriter_TalliesAndRenders(t *testing.T) {
	var buf bytes.Buffer
	pw := &progressWriter{total: 100, out: &buf}
	n, err := pw.Write(make([]byte, 40))
	if err != nil || n != 40 {
		t.Fatalf("Write returned (%d, %v)", n, err)
	}
	if _, err := pw.Write(make([]byte, 60)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if pw.written != 100 {
		t.Errorf("written = %d, want 100", pw.written)
	}
	pw.finish()
	if !strings.Contains(buf.String(), "100.0%") {
		t.Errorf("expected final render to show 100.0%%, got %q", buf.String())
	}
}

func TestProgressWriter_UnknownTotal(t *testing.T) {
	var buf bytes.Buffer
	pw := &progressWriter{total: -1, out: &buf}
	if _, err := pw.Write(make([]byte, 2048)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	pw.finish()
	if !strings.Contains(buf.String(), "2.0 KiB") {
		t.Errorf("expected byte count without percentage, got %q", buf.String())
	}
}

func TestSwapBinary_Unix(t *testing.T) {
	dir := t.TempDir()
	exec := filepath.Join(dir, "cli")
	tmp := filepath.Join(dir, "new")
	if err := os.WriteFile(exec, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary(exec, tmp); err != nil {
		t.Fatalf("swapBinary: %v", err)
	}
	got, _ := os.ReadFile(exec)
	if string(got) != "new" {
		t.Errorf("exec content = %q, want %q", got, "new")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("expected tmp file to be consumed by rename")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.2.0", "1.1.9", 1},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.0-rc1", 1},
		{"1.0.0-rc1", "1.0.0", -1},
		{"1.0.0-rc1", "1.0.0-rc2", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPickTargets(t *testing.T) {
	releases := []Release{
		{Tag: "v1.5.0-vt.2", Prerelease: true},
		{Tag: "v1.5.0-vt.1", Prerelease: true},
		{Tag: "v1.4.0"},
		{Tag: "v1.3.0"},
		{Tag: "v1.6.0-draft", Prerelease: true, Draft: true}, // ignored
	}
	top, stable := pickTargets(releases)
	if top == nil || top.Tag != "v1.5.0-vt.2" {
		t.Errorf("top = %v, want v1.5.0-vt.2", top)
	}
	if stable == nil || stable.Tag != "v1.4.0" {
		t.Errorf("stable = %v, want v1.4.0", stable)
	}
}

func TestPickTargets_OnlyStable(t *testing.T) {
	top, stable := pickTargets([]Release{{Tag: "v2.0.0"}, {Tag: "v2.1.0"}})
	if top == nil || stable == nil || top.Tag != "v2.1.0" || stable.Tag != "v2.1.0" {
		t.Errorf("top=%v stable=%v, want both v2.1.0", top, stable)
	}
}

func TestPickTargets_OnlyTesting(t *testing.T) {
	top, stable := pickTargets([]Release{{Tag: "v0.1.0-vt.1", Prerelease: true}})
	if top == nil || top.Tag != "v0.1.0-vt.1" {
		t.Errorf("top = %v, want v0.1.0-vt.1", top)
	}
	if stable != nil {
		t.Errorf("stable = %v, want nil (only testing releases)", stable)
	}
}

func TestPreReleaseAvailable(t *testing.T) {
	var svc UpgradeService
	stable := &Release{Tag: "v1.4.0"}
	newerPre := &Release{Tag: "v1.5.0-vt.2", Prerelease: true}
	cases := []struct {
		name    string
		current string
		top     *Release
		want    bool
	}{
		{"newer pre-release", "v1.4.0", newerPre, true},
		{"already on pre-release", "v1.5.0-vt.2", newerPre, false},
		{"older pre-release", "v1.6.0", newerPre, false},
		{"top is stable", "v1.3.0", stable, false},
		{"dev always sees pre-release", "dev", newerPre, true},
		{"nil top", "v1.0.0", nil, false},
	}
	for _, c := range cases {
		if got := svc.PreReleaseAvailable(c.current, c.top); got != c.want {
			t.Errorf("%s: PreReleaseAvailable(%q) = %v, want %v", c.name, c.current, got, c.want)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	var svc UpgradeService
	top := &Release{Tag: "v1.5.0-vt.2", Prerelease: true}
	stable := &Release{Tag: "v1.4.0"}
	cases := []struct {
		name           string
		top, stable    *Release
		wantPreRelease bool
		want           *Release
	}{
		// Opting in targets top even when it is NOT newer than current — the
		// case that reproduced the --force/--pre-release bug.
		{"opt-in targets top", top, stable, true, top},
		{"default targets stable", top, stable, false, stable},
		{"no stable falls back to top", top, nil, false, top},
		{"opt-in with no top falls back to stable", nil, stable, true, stable},
	}
	for _, c := range cases {
		if got := svc.ResolveTarget(c.top, c.stable, c.wantPreRelease); got != c.want {
			t.Errorf("%s: ResolveTarget() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGetTargetTriplet(t *testing.T) {
	triplet, err := getTargetTriplet()
	if err != nil {
		t.Skipf("unsupported platform for this test: %v", err)
	}
	if triplet == "" {
		t.Error("expected a non-empty triplet")
	}
}

func TestResolveAssetURL(t *testing.T) {
	rel := &Release{
		Tag: "v1.0.0",
		Assets: []ReleaseAsset{
			{Name: "devcontainer-cli-linux-x64", DownloadURL: "https://example.com/bin"},
			{Name: "devcontainer-cli-linux-x64.sha256", DownloadURL: "https://example.com/sum"},
		},
	}
	bin, sum, err := resolveAssetURL(rel, "linux-x64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bin != "https://example.com/bin" || sum != "https://example.com/sum" {
		t.Errorf("got bin=%q sum=%q", bin, sum)
	}
	if _, _, err := resolveAssetURL(rel, "darwin-arm64"); err == nil {
		t.Error("expected error for missing asset")
	}
}

func TestIsAllowedHost(t *testing.T) {
	allowed := []string{"github.com", "api.github.com", "objects.githubusercontent.com"}
	for _, h := range allowed {
		if !isAllowedHost(h) {
			t.Errorf("expected %q to be allowed", h)
		}
	}
	for _, h := range []string{"evil.com", "github.com.evil.com"} {
		if isAllowedHost(h) {
			t.Errorf("expected %q to be denied", h)
		}
	}
}
