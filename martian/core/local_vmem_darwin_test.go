//go:build darwin
// +build darwin

package core

import (
	"strings"
	"testing"
)

func newDarwinLocalVMemTestManager() *LocalJobManager {
	return &LocalJobManager{
		jobSettings: &JobManagerSettings{
			ThreadsPerJob: 1,
			MemGBPerJob:   1,
			ExtraVmemGB:   1,
		},
		maxCores: 4,
		maxMemGB: 4,
	}
}

func TestDarwinLocalVMemDefaultDisabled(t *testing.T) {
	jm := newDarwinLocalVMemTestManager()
	jm.setMaxVMem(0)
	if jm.maxVmemMB != 0 {
		t.Fatalf("expected default Darwin local vmem budget to be disabled, got %d MB", jm.maxVmemMB)
	}

	reqs := jm.GetSystemReqs(JobResources{MemGB: 1})
	if reqs.VMemGB != 2 {
		t.Fatalf("expected default vmem request to remain diagnostic at 2GB, got %g", reqs.VMemGB)
	}
	if reqs.VMemGBExplicit {
		t.Fatal("expected default Darwin local vmem request to remain implicit")
	}
}

func TestDarwinLocalVMemOptionEnablesPhysicalFootprintBudget(t *testing.T) {
	jm := newDarwinLocalVMemTestManager()
	jm.setMaxVMem(6)
	if jm.maxVmemMB != 6*1024 {
		t.Fatalf("expected explicit Darwin local vmem budget of 6144 MB, got %d", jm.maxVmemMB)
	}
	if !strings.Contains(formatLocalVMemMB(2*1024), "physical-footprint vmem") {
		t.Fatalf("expected Darwin local vmem formatter to describe physical footprint, got %q",
			formatLocalVMemMB(2*1024))
	}
	if !strings.Contains(localVMemLimitName(), "physical-footprint") {
		t.Fatalf("expected Darwin local vmem limit name to describe physical footprint, got %q",
			localVMemLimitName())
	}

	reqs := jm.GetSystemReqs(JobResources{
		MemGB:          2,
		VMemGB:         10,
		VMemGBExplicit: true,
	})
	if reqs.VMemGB != 6 {
		t.Fatalf("expected explicit vmem request to be capped to --localvmem budget, got %g", reqs.VMemGB)
	}
	if !reqs.VMemGBExplicit {
		t.Fatal("expected capped explicit vmem request to remain explicit")
	}
}
