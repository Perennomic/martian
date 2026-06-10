//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsProcessSupervisorImplementsInterface(t *testing.T) {
	var supervisor processSupervisor = &windowsProcessSupervisor{}
	cmd := exec.Command("cmd", "/c", "exit", "0")
	supervisor.configure(cmd, syscall.SIGKILL)
	if cmd.SysProcAttr != nil {
		t.Fatalf("expected Windows supervisor to leave SysProcAttr unset, got %#v", cmd.SysProcAttr)
	}
}

func TestNewWindowsProcessSupervisor(t *testing.T) {
	if _, ok := newProcessSupervisor().(*windowsProcessSupervisor); !ok {
		t.Fatalf("expected Windows process supervisor, got %T", newProcessSupervisor())
	}
}

func TestWindowsProcessSupervisorTerminationReason(t *testing.T) {
	supervisor := &windowsProcessSupervisor{}
	supervisor.kill(syscall.SIGKILL, "monitor kill")
	if got := supervisor.terminationReason(); got != "monitor kill" {
		t.Fatalf("expected termination reason %q, got %q", "monitor kill", got)
	}
	supervisor.kill(syscall.SIGKILL, "")
	if got := supervisor.terminationReason(); got != "monitor kill" {
		t.Fatalf("empty reason should preserve previous reason, got %q", got)
	}
}

func TestWindowsProcessSupervisorStartWait(t *testing.T) {
	supervisor := &windowsProcessSupervisor{}
	defer supervisor.close()

	cmd := exec.Command("cmd", "/c", "exit", "0")
	if err := supervisor.start(cmd); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.wait(); err != nil {
		t.Fatal(err)
	}
	if supervisor.job == 0 {
		t.Fatal("expected supervisor to create a Windows Job Object")
	}
}

func TestWindowsProcessSupervisorKillTerminatesJob(t *testing.T) {
	supervisor := &windowsProcessSupervisor{}
	defer supervisor.close()

	cmd := exec.Command("cmd", "/c", "ping -n 30 127.0.0.1 >NUL")
	if err := supervisor.start(cmd); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- supervisor.wait()
	}()
	time.Sleep(200 * time.Millisecond)
	supervisor.kill(syscall.SIGKILL, "test kill")

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Job Object termination")
	case err := <-done:
		if err == nil {
			t.Fatal("expected killed command to report an error")
		}
	}
	if got := supervisor.terminationReason(); got != "test kill" {
		t.Fatalf("expected termination reason %q, got %q", "test kill", got)
	}
}

func TestWindowsProcessSupervisorKillTerminatesChildProcess(t *testing.T) {
	supervisor := &windowsProcessSupervisor{}
	defer supervisor.close()

	childPidFile := filepath.Join(t.TempDir(), "child.pid")
	script := `$ErrorActionPreference = 'Stop'; ` +
		`$p = Start-Process -FilePath ping.exe -ArgumentList '-n','30','127.0.0.1' -PassThru -WindowStyle Hidden; ` +
		`Set-Content -LiteralPath $env:MRO_CHILD_PID_FILE -Value $p.Id -NoNewline; ` +
		`Wait-Process -Id $p.Id`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "MRO_CHILD_PID_FILE="+childPidFile)
	if err := supervisor.start(cmd); err != nil {
		t.Fatal(err)
	}

	childPid, err := waitForWindowsChildPidFile(childPidFile, 5*time.Second)
	if err != nil {
		supervisor.kill(syscall.SIGKILL, "cleanup after child pid wait failure")
		_ = supervisor.wait()
		t.Fatal(err)
	}
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, childPid)
	if err != nil {
		supervisor.kill(syscall.SIGKILL, "cleanup after child open failure")
		_ = supervisor.wait()
		t.Fatal(err)
	}
	defer windows.CloseHandle(child)

	done := make(chan error, 1)
	go func() {
		done <- supervisor.wait()
	}()
	supervisor.kill(syscall.SIGKILL, "test child kill")

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Job Object parent termination")
	case err := <-done:
		if err == nil {
			t.Fatal("expected killed parent command to report an error")
		}
	}
	if status, err := windows.WaitForSingleObject(child, 5000); err != nil {
		t.Fatal(err)
	} else if status == uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("child process %d survived Job Object termination", childPid)
	}
}

func TestWindowsJobAccountingInfoToRusage(t *testing.T) {
	info := &windowsJobObjectBasicAndIoAccountingInformation{}
	info.BasicInfo.TotalUserTime = 20_000_000
	info.BasicInfo.TotalKernelTime = 30_000_000
	info.BasicInfo.TotalPageFaultCount = 7
	info.IoInfo.ReadOperationCount = 11
	info.IoInfo.WriteOperationCount = 13

	got := windowsJobAccountingInfoToRusage(info)
	if got.UserTime != 2 {
		t.Fatalf("expected user time 2s, got %g", got.UserTime)
	}
	if got.SystemTime != 3 {
		t.Fatalf("expected system time 3s, got %g", got.SystemTime)
	}
	if got.MinorFaults != 7 {
		t.Fatalf("expected page fault count 7, got %d", got.MinorFaults)
	}
	if got.InBlocks != 11 || got.OutBlocks != 13 {
		t.Fatalf("expected IO operation counts 11/13, got %d/%d", got.InBlocks, got.OutBlocks)
	}
}

func waitForWindowsChildPidFile(name string, timeout time.Duration) (uint32, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(name)
		if err == nil {
			pid, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 32)
			if err != nil {
				return 0, err
			}
			return uint32(pid), nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return 0, fmt.Errorf("timed out waiting for child pid file %s: %w", name, lastErr)
}
