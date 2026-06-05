//go:build !darwin
// +build !darwin

package core

import (
	"strconv"

	"github.com/martian-lang/martian/martian/util"
)

func (self *LocalJobManager) setMaxVMem(userMaxVMemGB int) {
	self.maxVmemMB = int64(CheckMaxVmem(
		uint64(1+self.maxMemGB)*uint64(self.highMem.Vmem+1024*1024*1024)) /
		(1024 * 1024))

	if self.maxVmemMB == 0 ||
		int64(userMaxVMemGB)*1024 < self.maxVmemMB {
		self.maxVmemMB = int64(userMaxVMemGB) * 1024
	}
	// Subtract off mrp's current vmem usage.  If mrp is running in e.g. and
	// SGE cluster job with h_vmem set, the process will be killed if vmem
	// for the whole tree exceeds the value given, so we need to make sure to
	// reserve some space for mrp itself.  But, make sure at least 1gb remains
	// or else it'll just hang on local jobs.
	//
	// Subtracting it here is not ideal, since mrp's vmem usage may increase
	// over time.  However as we update the process tree memory usage later on
	// we ignore mrp's usage because for rss that can cause the pipeline to
	// hang.
	if selfMem := self.highMem.Vmem / (1024 * 1024); selfMem+1024 < self.maxVmemMB {
		self.maxVmemMB -= selfMem
	}
	requiredVmemGB := int64(self.jobSettings.MemGBPerJob+self.jobSettings.ExtraVmemGB) +
		(self.highMem.Vmem+1024*1024*1024-1)/(1024*1024*1024)
	if self.maxVmemMB > 0 && self.maxVmemMB/1024 < requiredVmemGB {
		util.PrintInfo("jobmngr",
			"WARNING: mrp will not run correctly with less"+
				"\n                              "+
				"than %dGB of virtual address space available.",
			requiredVmemGB)
	}
}

func formatLocalVMemMB(size int64) string {
	if size < 1024 {
		return string(append(
			strconv.AppendInt(make([]byte, 0, 64), size/100, 10),
			" MB of address space"...))
	}
	buf := make([]byte, 0, 18+len(" GB of address space"))
	buf = strconv.AppendFloat(buf, float64(size)/1024, 'g', 3, 64)
	return string(append(buf, " GB of address space"...))
}

func localVMemLimitName() string {
	return "virtual memory"
}
