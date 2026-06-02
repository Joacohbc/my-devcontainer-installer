package osutil

import "testing"

func TestCommandExists(t *testing.T) {
	// "go" should always be available in the development/test environment.
	if !CommandExists("go") {
		t.Error("expected 'go' to be on PATH in the test environment")
	}

	// This is a bogus binary name that shouldn't exist anywhere.
	if CommandExists("definitely-not-a-real-binary-xyz") {
		t.Error("did not expect a bogus binary to exist on PATH")
	}
}
