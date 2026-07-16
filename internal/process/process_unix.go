//go:build darwin || linux

// Package process provides process-group lifecycle controls shared by agent
// and verifier execution.
package process

import (
	"errors"
	"os/exec"
	"syscall"
)

// ConfigureGroup starts cmd in a dedicated process group and makes context
// cancellation terminate the entire group rather than only the direct child.
func ConfigureGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return KillGroup(cmd)
	}
}

// KillGroup terminates cmd and every descendant that remains in its process
// group. It is safe to call after the direct child has already exited.
func KillGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
