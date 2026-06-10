//go:build windows

package core

import "testing"

func TestWindowsUniquificationPolicy(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"", true},
		{disable, true},
		{"invalid", true},
		{"enable", false},
	} {
		if got := shouldDisableUniquificationValue(tc.value); got != tc.want {
			t.Fatalf("shouldDisableUniquificationValue(%q) = %t, want %t",
				tc.value, got, tc.want)
		}
	}
}
