//go:build windows
// +build windows

package util

import "testing"

func TestWindowsParseMroPathPreservesDriveLetters(t *testing.T) {
	paths := ParseMroPath(`C:\martian mros;D:\shared\pipelines`)
	if len(paths) != 2 {
		t.Fatalf("expected two Windows MROPATH entries, got %#v", paths)
	}
	if paths[0] != `C:\martian mros` {
		t.Fatalf("expected first drive-letter path to be preserved, got %q", paths[0])
	}
	if paths[1] != `D:\shared\pipelines` {
		t.Fatalf("expected second drive-letter path to be preserved, got %q", paths[1])
	}
}
