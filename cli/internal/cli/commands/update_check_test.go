package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/service"
	"github.com/spf13/cobra"
)

func TestShouldSkipStartupUpdateCheck(t *testing.T) {
	normalCmd := &cobra.Command{Use: "status"}
	cases := []struct {
		name    string
		cmd     *cobra.Command
		version string
		env     string
		want    bool
	}{
		{"release build normal cmd", normalCmd, "v1.4.0", "", false},
		{"dev version skipped", normalCmd, "dev", "", true},
		{"test version skipped", normalCmd, "test", "", true},
		{"empty version skipped", normalCmd, "", "", true},
		{"opt-out env skipped", normalCmd, "v1.4.0", "1", true},
		{"upgrade-cli skipped", &cobra.Command{Use: "upgrade-cli"}, "v1.4.0", "", true},
		{"completion skipped", &cobra.Command{Use: "__complete"}, "v1.4.0", "", true},
		{"help skipped", &cobra.Command{Use: "help"}, "v1.4.0", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(envNoUpdateCheck, c.env)
			if got := shouldSkipStartupUpdateCheck(c.cmd, c.version); got != c.want {
				t.Errorf("shouldSkipStartupUpdateCheck = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCheckStartupUpdates_WritesWarningsToStderrOnly(t *testing.T) {
	t.Setenv(envNoUpdateCheck, "")
	cacheFile := filepath.Join(t.TempDir(), "update-check.json")
	skillFile := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := service.UpdateCheckService{
		Now:              func() time.Time { return time.Date(2026, 11, 13, 3, 0, 0, 0, time.UTC) },
		CachePath:        func() string { return cacheFile },
		FetchLatestCLI:   func() (string, error) { return "v2.0.0", nil },
		FetchRemoteSkill: func() ([]byte, error) { return []byte("new"), nil },
		FindSkills: func() []service.SkillLocation {
			return []service.SkillLocation{{
				Path:        skillFile,
				DisplayPath: "~/.agents/skills/devcontainer-cli",
				SHA256:      "0000",
			}}
		},
	}

	var stderr bytes.Buffer
	checkStartupUpdates(&cobra.Command{Use: "status"}, "v1.9.0", svc, &stderr)

	out := stderr.String()
	if !strings.Contains(out, "v1.9.0 -> v2.0.0") {
		t.Errorf("expected CLI update notice on stderr, got %q", out)
	}
	if !strings.Contains(out, "Host skill devcontainer-cli is outdated") {
		t.Errorf("expected skill update notice on stderr, got %q", out)
	}
}
