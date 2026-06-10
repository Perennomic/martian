//go:build !windows
// +build !windows

package main

import (
	"syscall"
	"testing"

	"github.com/martian-lang/martian/martian/util"
)

func TestExternalSignal(t *testing.T) {
	for _, sig := range util.HANDLED_SIGNALS {
		sysSig, ok := sig.(syscall.Signal)
		if !ok {
			t.Fatalf("expected syscall.Signal, got %T", sig)
		}
		if !externalSignal(sysSig) {
			t.Fatalf("expected handled signal %v to be external", sig)
		}
	}
	if !externalSignal(syscall.SIGKILL) {
		t.Fatal("expected SIGKILL to be external")
	}
	if externalSignal(syscall.SIGSEGV) {
		t.Fatal("unexpected external classification for SIGSEGV")
	}
}
