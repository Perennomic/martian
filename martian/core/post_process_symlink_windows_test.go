//go:build windows

package core

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsSymlinkUnavailable(t *testing.T) {
	if !isWindowsSymlinkUnavailable(&os.LinkError{
		Op:  "symlink",
		Err: windows.ERROR_PRIVILEGE_NOT_HELD,
	}) {
		t.Fatal("expected symlink privilege errors to allow post-process fallback")
	}
	if isWindowsSymlinkUnavailable(&os.LinkError{
		Op:  "symlink",
		Err: windows.ERROR_FILE_NOT_FOUND,
	}) {
		t.Fatal("unexpected fallback for missing path errors")
	}
}

func TestWindowsPathIsInsideDirVolume(t *testing.T) {
	if !pathIsInsideDir(`C:\ps\files\out.txt`, `C:\ps`) {
		t.Fatal("expected child path on the same volume to be inside")
	}
	if pathIsInsideDir(`C:\ps-other\out.txt`, `C:\ps`) {
		t.Fatal("unexpected sibling-prefix path containment")
	}
	if pathIsInsideDir(`D:\ps\out.txt`, `C:\ps`) {
		t.Fatal("unexpected cross-volume path containment")
	}
}
