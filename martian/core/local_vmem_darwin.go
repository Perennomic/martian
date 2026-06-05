//go:build darwin
// +build darwin

package core

import (
	"strconv"

	"github.com/martian-lang/martian/martian/util"
)

func (self *LocalJobManager) setMaxVMem(userMaxVMemGB int) {
	if userMaxVMemGB <= 0 {
		self.maxVmemMB = 0
		return
	}
	self.maxVmemMB = int64(userMaxVMemGB) * 1024
	util.LogInfo("jobmngr",
		"Using %d GB of macOS physical-footprint vmem budget, per --localvmem option. "+
			"macOS does not provide Linux-style virtual address-space enforcement.",
		userMaxVMemGB)
}

func formatLocalVMemMB(size int64) string {
	if size < 1024 {
		return string(append(
			strconv.AppendInt(make([]byte, 0, 64), size/100, 10),
			" MB of physical-footprint vmem"...))
	}
	buf := make([]byte, 0, 18+len(" GB of physical-footprint vmem"))
	buf = strconv.AppendFloat(buf, float64(size)/1024, 'g', 3, 64)
	return string(append(buf, " GB of physical-footprint vmem"...))
}

func localVMemLimitName() string {
	return "macOS physical-footprint vmem"
}
