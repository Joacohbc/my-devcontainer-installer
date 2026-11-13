package service

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestUpdateCheckService_DetectsOutdatedCLIAndSkillAndUsesCache(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "update-check.json")
	skillFile := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# old skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 13, 3, 0, 0, 0, time.UTC)
	fetchCount := 0
	svc := UpdateCheckService{
		Now:       func() time.Time { return now },
		CachePath: func() string { return cacheFile },
		FetchLatestCLI: func() (string, error) {
			fetchCount++
			return "v1.5.0", nil
		},
		FetchRemoteSkill: func() ([]byte, error) {
			fetchCount++
			return []byte("# new skill\r\n"), nil
		},
		FindSkills: func() []SkillLocation {
			data, _ := os.ReadFile(skillFile)
			return []SkillLocation{{
				Path:        skillFile,
				DisplayPath: "~/.agents/skills/devcontainer-cli",
				SHA256:      sha256NormalizedHex(data),
			}}
		},
	}

	status := svc.Check("v1.4.0", false)
	if !status.CLIOutdated || status.LatestCLI != "v1.5.0" {
		t.Fatalf("expected outdated CLI v1.4.0 -> v1.5.0, got %+v", status)
	}
	if len(status.OutdatedPaths) != 1 || status.OutdatedPaths[0] != "~/.agents/skills/devcontainer-cli" {
		t.Fatalf("expected outdated skill path, got %+v", status.OutdatedPaths)
	}
	if fetchCount != 2 {
		t.Fatalf("expected 2 initial fetches, got %d", fetchCount)
	}

	// Within 24h TTL, updating local skill clears the warning immediately without network calls.
	if err := os.WriteFile(skillFile, []byte("# new skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Hour)
	status = svc.Check("v1.5.0", false)
	if fetchCount != 2 {
		t.Fatalf("expected cached check with 0 new fetches, got %d", fetchCount)
	}
	if status.CLIOutdated || len(status.OutdatedPaths) != 0 {
		t.Fatalf("expected clean up-to-date status after update, got %+v", status)
	}

	// After 24h TTL expires, fetches run again.
	now = now.Add(24 * time.Hour)
	_ = svc.Check("v1.5.0", false)
	if fetchCount != 4 {
		t.Fatalf("expected refresh after 24h TTL, got fetchCount=%d", fetchCount)
	}
}

func TestUpdateCheckService_OfflineBackoffAndPreservesPriorCache(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "update-check.json")
	now := time.Date(2026, 11, 13, 3, 0, 0, 0, time.UTC)

	attempts := 0
	online := false
	svc := UpdateCheckService{
		Now:       func() time.Time { return now },
		CachePath: func() string { return cacheFile },
		FetchLatestCLI: func() (string, error) {
			attempts++
			if !online {
				return "", os.ErrDeadlineExceeded
			}
			return "v2.0.0", nil
		},
		FetchRemoteSkill: func() ([]byte, error) {
			attempts++
			if !online {
				return nil, os.ErrDeadlineExceeded
			}
			return []byte("latest"), nil
		},
		FindSkills: func() []SkillLocation { return nil },
	}

	status := svc.Check("v1.9.0", false)
	if status.CLIOutdated || status.SkillChecked {
		t.Fatalf("expected empty status when initial offline fetch fails, got %+v", status)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}

	// Inside the 1h retry backoff, does not hammer GitHub.
	now = now.Add(15 * time.Minute)
	_ = svc.Check("v1.9.0", false)
	if attempts != 2 {
		t.Fatalf("expected no retry inside 1h backoff, got %d attempts", attempts)
	}

	// After 1h backoff, retries and detects update.
	online = true
	now = now.Add(1 * time.Hour)
	status = svc.Check("v1.9.0", false)
	if !status.CLIOutdated || status.LatestCLI != "v2.0.0" {
		t.Fatalf("expected CLI update after recovery, got %+v", status)
	}
}

func TestFindInstalledHostSkills_DeduplicatesSymlinks(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWd) })

	agentsDir := filepath.Join(home, ".agents", "skills", "devcontainer-cli")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "SKILL.md"), []byte("shared"), 0o644); err != nil {
		t.Fatal(err)
	}

	claudeSkills := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeSkills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(agentsDir, filepath.Join(claudeSkills, "devcontainer-cli")); err != nil {
		t.Fatal(err)
	}

	geminiDir := filepath.Join(home, ".gemini", "config", "skills", "devcontainer-cli")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(geminiDir, "SKILL.md"), []byte("separate"), 0o644); err != nil {
		t.Fatal(err)
	}

	found := FindInstalledHostSkills()
	if len(found) != 2 {
		t.Fatalf("expected 2 unique skill installs (symlink deduplicated), got %d: %+v", len(found), found)
	}

	displays := []string{found[0].DisplayPath, found[1].DisplayPath}
	if !slices.Contains(displays, filepath.Join("~", ".agents", "skills", "devcontainer-cli")) {
		t.Errorf("expected ~/.agents/skills/devcontainer-cli in %v", displays)
	}
	if !slices.Contains(displays, filepath.Join("~", ".gemini", "config", "skills", "devcontainer-cli")) {
		t.Errorf("expected ~/.gemini/config/skills/devcontainer-cli in %v", displays)
	}
}

func TestFormatUpdateWarnings(t *testing.T) {
	warnings := FormatUpdateWarnings(UpdateStatus{
		CurrentCLI:    "v1.4.0",
		LatestCLI:     "v1.5.0",
		CLIOutdated:   true,
		OutdatedPaths: []string{"~/.agents/skills/devcontainer-cli"},
	})
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "devcontainer-cli config upgrade-cli") {
		t.Errorf("missing upgrade hint in %q", warnings[0])
	}
	if !strings.Contains(warnings[1], "npx skills add") {
		t.Errorf("missing skill update hint in %q", warnings[1])
	}
}
