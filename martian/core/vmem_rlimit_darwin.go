//go:build darwin
// +build darwin

package core

// ShouldSetVMemRLimit returns true when mrjob should use RLIMIT_AS to constrain
// the child process address space. Darwin VMem monitoring uses physical
// footprint semantics, not Linux-style virtual address-space enforcement.
func ShouldSetVMemRLimit(*JobInfo) bool {
	return false
}
