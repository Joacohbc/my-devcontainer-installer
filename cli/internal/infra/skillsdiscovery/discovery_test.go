package skillsdiscovery

import (
	"strings"
	"testing"
)

func TestIsSupportedNodeVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"v24.14.0", true},
		{"v22.20.0", true},
		{"v22.20.1", true},
		{"v22.21.0", true},
		{"v23.0.0", true},
		{"22.20.0", true},
		{"  v24.0.0\n", true},
		{"v22.19.0", false},
		{"v22.0.0", false},
		{"v20.18.0", false},
		{"v18.12.0", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		got := isSupportedNodeVersion(tt.version)
		if got != tt.want {
			t.Errorf("isSupportedNodeVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestErrNodeRequirementMessage(t *testing.T) {
	expectedSub := "requires Node >= 22.20 on the host; or add a single skill with 'config skill add <id> --ref <source> --skill <name>'"
	if !strings.Contains(ErrNodeRequirement.Error(), expectedSub) {
		t.Errorf("ErrNodeRequirement = %q, want substring %q", ErrNodeRequirement.Error(), expectedSub)
	}
}

func TestDiscoverSkills_SourceStartingWithDash(t *testing.T) {
	runner := NewRunner()
	_, err := runner.DiscoverSkills("-flag-source")
	if err == nil {
		t.Fatal("expected error for source starting with '-'")
	}
	if !strings.Contains(err.Error(), "cannot start with '-'") {
		t.Errorf("unexpected error: %v", err)
	}
}
