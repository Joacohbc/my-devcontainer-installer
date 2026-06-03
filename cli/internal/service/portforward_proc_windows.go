//go:build windows

package service

import (
	"os"
	"syscall"
)

// detachAttr starts the ssh process in a new process group so it is not killed
// when the CLI's console is closed.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// processAliveImpl reports whether pid names a live process. On Windows
// os.FindProcess opens the process and fails for a dead pid.
func processAliveImpl(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = proc.Release()
	return true
}

func killProcessImpl(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
