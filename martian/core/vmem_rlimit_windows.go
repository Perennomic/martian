//go:build windows
// +build windows

package core

// ShouldSetVMemRLimit returns false on Windows. The MVP treats Windows VMem as
// an observed resource policy, not as an RLIMIT_AS-equivalent hard limit.
func ShouldSetVMemRLimit(*JobInfo) bool {
	return false
}
