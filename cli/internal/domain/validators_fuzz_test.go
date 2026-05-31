package domain_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

// FuzzSanitizeDockerName asserts the core invariant: whatever junk goes in, the
// sanitized result is always a valid Docker name (or the fallback, which is
// itself valid). This guards against slicing/normalization edge cases.
func FuzzSanitizeDockerName(f *testing.F) {
	seeds := []string{
		"", " ", "My Project!", "----", "...", "_._",
		"ünîçödé", "a/b:c@d", "0", "Z",
		"this-is-an-extremely-long-name-that-should-be-truncated-to-sixty-three-characters-or-fewer",
		"\t\n\x00weird",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got := domain.SanitizeDockerName(raw, "")
		if got == "" {
			t.Fatalf("SanitizeDockerName(%q) returned empty string", raw)
		}
		if len(got) > 63 {
			t.Errorf("SanitizeDockerName(%q) = %q exceeds 63 chars (%d)", raw, got, len(got))
		}
		if !domain.IsValidDockerName(got) {
			t.Errorf("SanitizeDockerName(%q) = %q is not a valid docker name", raw, got)
		}
	})
}

// FuzzIsValidDockerName ensures the validator never panics on arbitrary input.
func FuzzIsValidDockerName(f *testing.F) {
	for _, s := range []string{"", "ok", "-bad", "a.b_c-d", "WAY", "x/y"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		_ = domain.IsValidDockerName(name)
	})
}

// FuzzIsValidImageName ensures the image-name validator never panics.
func FuzzIsValidImageName(f *testing.F) {
	for _, s := range []string{"", "foo", "foo:latest", "ghcr.io/o/r:tag", "BAD NAME"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		_ = domain.IsValidImageName(name)
	})
}

// FuzzIsValidCidr ensures the CIDR validator never panics.
func FuzzIsValidCidr(f *testing.F) {
	for _, s := range []string{"", "10.0.0.0/8", "172.16.0.0/12", "::1/128", "garbage"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cidr string) {
		_ = domain.IsValidCidr(cidr)
	})
}
