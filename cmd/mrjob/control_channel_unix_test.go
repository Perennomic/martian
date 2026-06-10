//go:build !windows
// +build !windows

package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestFdControlChannelConfiguresLegacyExtraFiles(t *testing.T) {
	log, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	control, err := newControlChannel(log)
	if err != nil {
		t.Fatal(err)
	}
	defer control.closeParent()
	defer control.closeChild()

	cmd := exec.Command("true")
	control.configure(cmd)
	if len(cmd.ExtraFiles) != 2 {
		t.Fatalf("expected log and error fds, got %#v", cmd.ExtraFiles)
	}
	if cmd.ExtraFiles[0] != log {
		t.Fatal("expected fd 3 to be the stage log")
	}
	if cmd.ExtraFiles[1] == nil {
		t.Fatal("expected fd 4 error writer")
	}

	control.closeChild()
	control.closeChild()
	control.closeParent()
	control.closeParent()
}
