//go:build !darwin && !windows
// +build !darwin,!windows

package core

// ShouldSetVMemRLimit returns true when mrjob should use RLIMIT_AS to constrain
// the child process address space.
func ShouldSetVMemRLimit(jobInfo *JobInfo) bool {
	return ShouldCheckVMem(jobInfo)
}
