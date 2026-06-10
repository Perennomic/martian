//go:build windows
// +build windows

package main

// Windows does not expose Unix signal termination semantics through mrjob's
// Job Object supervisor. Preserve the native process error so callers report
// the exit code or stage-returned metadata rather than inventing a signal.
func sigToErr(err error) error {
	return err
}
