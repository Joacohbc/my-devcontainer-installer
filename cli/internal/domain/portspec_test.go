package domain_test

import (
	"testing"

	"github.com/joacohbc/my-devcontainer-installer/cli/internal/domain"
)

// A profile's ports are not typed at a prompt, so a typo has to surface at
// generate time rather than as a compose error when the stack comes up.
func TestValidatePortSpecs(t *testing.T) {
	cases := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"bare container port", "3000", false},
		{"explicit mapping", "3000:3000", false},
		{"different host port", "8080:80", false},
		{"with host ip", "0.0.0.0:8080:80", false},
		{"loopback ip", "127.0.0.1:8080:80", false},
		{"protocol suffix", "5353:53/udp", false},
		{"ephemeral host port", ":80", false},
		{"highest port", "65535", false},
		{"empty", "", true},
		{"blank", "   ", true},
		{"not a number", "web", true},
		{"zero", "0", true},
		{"above range", "65536", true},
		{"negative", "-1", true},
		{"too many fields", "1:2:3:4", true},
		{"empty container port", "8080:", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.ValidatePortSpecs([]string{c.spec})
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePortSpecs([%q]) error = %v, wantErr %v", c.spec, err, c.wantErr)
			}
		})
	}

	if err := domain.ValidatePortSpecs(nil); err != nil {
		t.Errorf("no ports is not an error, got %v", err)
	}
	if err := domain.ValidatePortSpecs([]string{"3000", "bad"}); err == nil {
		t.Error("expected one bad entry to fail the whole list")
	}
}

// Both accepted shapes must survive into a compose spec, since the profile is
// what chooses between a free host port and a fixed one.
func TestBindLoopbackKeepsBothProfileShapes(t *testing.T) {
	if got := domain.BindLoopback("3000"); got != "127.0.0.1::3000" {
		t.Errorf("a bare port must leave the host port to Docker, got %q", got)
	}
	if got := domain.BindLoopback("3000:3000"); got != "127.0.0.1:3000:3000" {
		t.Errorf("an explicit mapping must be pinned, got %q", got)
	}
	if got := domain.BindLoopback("0.0.0.0:3000:3000"); got != "0.0.0.0:3000:3000" {
		t.Errorf("an explicit host IP must be respected, got %q", got)
	}
}
