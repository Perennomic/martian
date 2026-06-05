//go:build darwin
// +build darwin

package core

// ShouldCheckVMem returns true when monitor should treat VMemGB as an enforced
// threshold for this job. Darwin only enforces explicitly requested vmem_gb
// because the default VMem value is a Linux compatibility reservation, not a
// Darwin hard address-space limit.
func ShouldCheckVMem(jobInfo *JobInfo) bool {
	return jobInfo != nil && jobInfo.VMemGB > 0 && jobInfo.VMemGBExplicit
}
