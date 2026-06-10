//go:build windows
// +build windows

package core

import "fmt"

func VMemQuotaMessage(usingGB, allowedGB float64) string {
	return fmt.Sprintf(
		"Stage exceeded its explicit Windows observed-memory threshold (using %.1f, allowed %gG). "+
			"Windows monitoring uses platform memory counters such as working set, private bytes, and commit; "+
			"this is not Linux-style virtual address-space enforcement.",
		usingGB, allowedGB)
}
