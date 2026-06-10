//go:build !windows
// +build !windows

package main

import (
	"syscall"
	"testing"
)

func TestDefaultProcessSupervisorTerminationReason(t *testing.T) {
	supervisor := &defaultProcessSupervisor{}
	supervisor.kill(syscall.SIGKILL, "monitor kill")
	if got := supervisor.terminationReason(); got != "monitor kill" {
		t.Fatalf("expected termination reason %q, got %q", "monitor kill", got)
	}
	supervisor.kill(syscall.SIGKILL, "")
	if got := supervisor.terminationReason(); got != "monitor kill" {
		t.Fatalf("empty reason should preserve previous reason, got %q", got)
	}
}
