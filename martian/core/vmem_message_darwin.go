//go:build darwin
// +build darwin

package core

import "fmt"

func VMemQuotaMessage(usingGB, allowedGB float64) string {
	return fmt.Sprintf(
		"Stage exceeded its explicit macOS physical-footprint vmem threshold (using %.1f, allowed %gG). "+
			"macOS does not provide Linux-style virtual address-space enforcement.",
		usingGB, allowedGB)
}
