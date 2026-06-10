//go:build windows
// +build windows

package core

// ShouldCheckVMem returns true when monitor should treat VMemGB as an observed
// soft threshold for this job. Windows does not map the default VMem value to a
// Linux-style virtual address-space limit.
func ShouldCheckVMem(jobInfo *JobInfo) bool {
	return jobInfo != nil && jobInfo.VMemGB > 0 && jobInfo.VMemGBExplicit
}
