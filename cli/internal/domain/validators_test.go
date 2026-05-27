package domain_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

func TestIsValidImageName(t *testing.T) {
	valid := []string{
		"foo",
		"foo:latest",
		"ghcr.io/joacohbc/devcontainer-ssh",
		"devcontainer-ssh:local",
	}
	for _, name := range valid {
		if !domain.IsValidImageName(name) {
			t.Errorf("expected %q to be a valid image name", name)
		}
	}
}

func TestIsValidImageName_Invalid(t *testing.T) {
	invalid := []string{
		"FOO",
		"foo bar",
		"",
	}
	for _, name := range invalid {
		if domain.IsValidImageName(name) {
			t.Errorf("expected %q to be an invalid image name", name)
		}
	}
}

func TestIsValidCidr(t *testing.T) {
	valid := []string{
		"172.25.0.0/24",
		"10.0.0.0/8",
	}
	for _, cidr := range valid {
		if !domain.IsValidCidr(cidr) {
			t.Errorf("expected %q to be a valid CIDR", cidr)
		}
	}
}

func TestIsValidCidr_Invalid(t *testing.T) {
	invalid := []string{
		"172.25.0.0",
		"999.0.0.0/24",
		"foo",
		"2001:db8::/32",
	}
	for _, cidr := range invalid {
		if domain.IsValidCidr(cidr) {
			t.Errorf("expected %q to be an invalid CIDR", cidr)
		}
	}
}

func TestIsValidDockerName(t *testing.T) {
	valid := []string{
		"myproject",
		"my-project",
		"My.Project",
		"a1",
	}
	for _, name := range valid {
		if !domain.IsValidDockerName(name) {
			t.Errorf("expected %q to be a valid docker name", name)
		}
	}
}

func TestIsValidDockerName_Invalid(t *testing.T) {
	invalid := []string{
		"-starts-with-dash",
		"",
	}
	for _, name := range invalid {
		if domain.IsValidDockerName(name) {
			t.Errorf("expected %q to be an invalid docker name", name)
		}
	}
}

func TestSanitizeDockerName(t *testing.T) {
	cases := []struct {
		raw      string
		fallback string
		want     string
	}{
		{"My Project!", "devcontainer", "my-project"},
		{"  ", "devcontainer", "devcontainer"},
		{"valid-name", "devcontainer", "valid-name"},
		{"UPPER", "devcontainer", "upper"},
	}
	for _, tc := range cases {
		got := domain.SanitizeDockerName(tc.raw, tc.fallback)
		if got != tc.want {
			t.Errorf("SanitizeDockerName(%q, %q) = %q, want %q", tc.raw, tc.fallback, got, tc.want)
		}
	}
}
