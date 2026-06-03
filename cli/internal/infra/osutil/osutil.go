package osutil

import "os/exec"

// CommandExists reports whether bin is on PATH.
func CommandExists(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
