//go:build !windows

package service

import (
	"os"
	"syscall"
)

// detachAttr makes a spawned ssh process survive the CLI exit by starting it in
// its own session (no controlling terminal, own process group).
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// processAliveImpl reports whether pid names a live process. On Unix
// os.FindProcess always succeeds, so liveness is probed with signal 0.
func processAliveImpl(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func killProcessImpl(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
