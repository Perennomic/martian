//go:build darwin
// +build darwin

package core

import (
	"strings"
	"testing"
)

func TestDarwinShouldCheckVMemRequiresExplicitRequest(t *testing.T) {
	if ShouldCheckVMem(&JobInfo{VMemGB: 4}) {
		t.Fatal("expected default Darwin vmem reservation to skip monitor enforcement")
	}
	if !ShouldCheckVMem(&JobInfo{VMemGB: 4, VMemGBExplicit: true}) {
		t.Fatal("expected explicit Darwin vmem request to enable monitor enforcement")
	}
	if ShouldCheckVMem(&JobInfo{VMemGBExplicit: true}) {
		t.Fatal("expected zero Darwin vmem request to skip monitor enforcement")
	}
}

func TestDarwinShouldNotSetVMemRLimit(t *testing.T) {
	if ShouldSetVMemRLimit(&JobInfo{VMemGB: 4, VMemGBExplicit: true}) {
		t.Fatal("expected explicit Darwin vmem request to skip RLIMIT_AS enforcement")
	}
}

func TestDarwinVMemQuotaMessageExplainsFootprint(t *testing.T) {
	msg := VMemQuotaMessage(8.5, 4)
	if !strings.Contains(msg, "macOS physical-footprint") {
		t.Fatalf("expected macOS physical-footprint wording, got %q", msg)
	}
	if !strings.Contains(msg, "does not provide Linux-style") {
		t.Fatalf("expected Linux hard-limit distinction, got %q", msg)
	}
}
