//go:build windows
// +build windows

package core

import "testing"

func TestNewWindowsLocalProcessSupervisor(t *testing.T) {
	if _, ok := newLocalProcessSupervisor().(*windowsLocalProcessSupervisor); !ok {
		t.Fatalf("expected Windows local process supervisor, got %T", newLocalProcessSupervisor())
	}
}
