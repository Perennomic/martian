//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/martian-lang/martian/martian/util"
)

// Returns true if sig is a signal which we expect is not due to a
// bug in the stage code.
func externalSignal(sig syscall.Signal) bool {
	for _, handled := range util.HANDLED_SIGNALS {
		if sig == handled {
			return true
		}
	}
	// SIGKLL isn't in the handled set because it can't be handled, but
	// should be treated equivalently to SIGTERM for these purposes.
	if sig == syscall.SIGKILL {
		return true
	}
	return false
}

// Convert an exec.ExitError to a stageReturnedError if the failure was due to
// one of the signals that we choose to handle. This allows restart logic to
// work correctly on Unix-style process supervisors.
func sigToErr(err error) error {
	if err == nil {
		return err
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if state, ok := exitErr.Sys().(*syscall.WaitStatus); ok &&
			state.Signaled() && externalSignal(state.Signal()) {
			return &stageReturnedError{
				message: fmt.Sprintf(
					"stage code received signal: %v", state.Signal()),
			}
		}
	}
	return err
}
