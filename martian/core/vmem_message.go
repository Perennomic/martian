//go:build !darwin
// +build !darwin

package core

import "fmt"

func VMemQuotaMessage(usingGB, allowedGB float64) string {
	return fmt.Sprintf(
		"Stage exceeded its address space quota (using %.1f, allowed %gG)",
		usingGB, allowedGB)
}
