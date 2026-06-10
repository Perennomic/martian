//go:build windows
// +build windows

package core

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsAvailableSpaceAndMountOptions(t *testing.T) {
	p, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	bytes, inodes, statfsType, err := GetAvailableSpace(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes == 0 {
		t.Fatal("expected nonzero available bytes")
	}
	if inodes != 0 {
		t.Fatalf("expected Windows inode availability to be unavailable as 0, got %d", inodes)
	}
	if statfsType == "" {
		t.Fatal("expected Windows filesystem type")
	}

	mountType, opts, err := GetMountOptions(p)
	if err != nil {
		t.Fatal(err)
	}
	if mountType != statfsType {
		t.Fatalf("expected matching filesystem types, got statfs=%q mount=%q", statfsType, mountType)
	}
	if !strings.ContainsRune(opts, ',') {
		t.Fatalf("expected comma-separated Windows volume options, got %q", opts)
	}
}

func TestWindowsVolumeOptions(t *testing.T) {
	opts := windowsVolumeOptions("", 0)
	if opts != "rw" {
		t.Fatalf("expected default options to be rw, got %q", opts)
	}
	opts = windowsVolumeOptions("", windows.FILE_READ_ONLY_VOLUME|windows.FILE_SUPPORTS_HARD_LINKS)
	if !strings.Contains(opts, "ro") || !strings.Contains(opts, "hardlinks") {
		t.Fatalf("expected read-only hardlink options, got %q", opts)
	}
}
