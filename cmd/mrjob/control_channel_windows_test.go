//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWindowsControlChannelConfiguresEnv(t *testing.T) {
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

	cmd := exec.Command("cmd", "/c", "exit", "0")
	control.configure(cmd)
	env := strings.Join(cmd.Env, "\n")
	for _, want := range []string{
		controlChannelEnv,
		controlLogPathEnv + log.Name(),
		controlErrorPathEnv,
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("expected env to contain %q, got\n%s", want, env)
		}
	}
}

func TestWindowsControlChannelReadsErrorAfterWait(t *testing.T) {
	if os.Getenv("MRO_CONTROL_CHANNEL_TEST_HELPER") == "1" {
		errorPath := os.Getenv("MRO_CONTROL_ERROR_PATH")
		if errorPath == "" {
			t.Fatal("missing MRO_CONTROL_ERROR_PATH")
		}
		if err := os.WriteFile(errorPath, []byte("stage error"), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}

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

	cmd := exec.Command(os.Args[0], "-test.run=TestWindowsControlChannelReadsErrorAfterWait")
	cmd.Env = append(os.Environ(), "MRO_CONTROL_CHANNEL_TEST_HELPER=1")
	control.configure(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got := string(control.readErrorAfterWait(8100))
	if !strings.Contains(got, "stage error") {
		t.Fatalf("expected stage error in control channel, got %q", got)
	}
}
