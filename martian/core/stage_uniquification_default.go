//go:build !windows

package core

func shouldDisableUniquificationValue(value string) bool {
	return value == disable
}
