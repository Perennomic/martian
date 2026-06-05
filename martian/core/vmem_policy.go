//go:build !darwin
// +build !darwin

package core

// ShouldCheckVMem returns true when monitor should treat VMemGB as an enforced
// threshold for this job.
func ShouldCheckVMem(jobInfo *JobInfo) bool {
	return jobInfo != nil && jobInfo.VMemGB > 0
}
