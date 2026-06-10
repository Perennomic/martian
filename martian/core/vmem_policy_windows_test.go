//go:build windows
// +build windows

package core

import (
	"strings"
	"testing"
)

func TestWindowsShouldCheckVMemRequiresExplicitRequest(t *testing.T) {
	if ShouldCheckVMem(&JobInfo{VMemGB: 4}) {
		t.Fatal("expected default Windows vmem reservation to skip monitor enforcement")
	}
	if !ShouldCheckVMem(&JobInfo{VMemGB: 4, VMemGBExplicit: true}) {
		t.Fatal("expected explicit Windows vmem request to enable monitor threshold")
	}
	if ShouldCheckVMem(&JobInfo{VMemGBExplicit: true}) {
		t.Fatal("expected zero Windows vmem request to skip monitor threshold")
	}
}

func TestWindowsShouldNotSetVMemRLimit(t *testing.T) {
	if ShouldSetVMemRLimit(&JobInfo{VMemGB: 4, VMemGBExplicit: true}) {
		t.Fatal("expected explicit Windows vmem request to skip RLIMIT_AS enforcement")
	}
}

func TestWindowsSetVMemRLimitNoOpContract(t *testing.T) {
	if oldAmount, err := SetVMemRLimit(4 * 1024 * 1024 * 1024); err != nil {
		t.Fatalf("expected Windows SetVMemRLimit no-op to succeed, got %v", err)
	} else if oldAmount != 0 {
		t.Fatalf("expected unavailable previous Windows limit to be 0, got %d", oldAmount)
	}
}

func TestWindowsVMemQuotaMessageExplainsObservedMemory(t *testing.T) {
	msg := VMemQuotaMessage(8.5, 4)
	for _, want := range []string{
		"Windows observed-memory threshold",
		"working set",
		"private bytes",
		"commit",
		"not Linux-style virtual address-space enforcement",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected Windows VMem message to contain %q, got %q", want, msg)
		}
	}
}
