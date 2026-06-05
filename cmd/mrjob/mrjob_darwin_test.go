//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/martian-lang/martian/martian/core"
	"github.com/martian-lang/martian/martian/util"
)

func TestReportChildrenDarwin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sleep", "5")
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	defer cmd.Wait()

	var buf bytes.Buffer
	util.LogTeeWriter(&buf)
	defer func() {
		util.LOGGER = nil
	}()
	if !reportChildren() {
		t.Fatal("expected to find child process")
	}
	output := string(bytes.TrimSpace(buf.Bytes()))
	if !strings.Contains(output, "Orphaned child process") ||
		!strings.Contains(output, "is still running") {
		t.Fatalf("expected message about orphaned child process, got\n%s", output)
	}
}

func waitForFile(t *testing.T, filename string) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filename); err == nil && len(b) > 0 {
			return b
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", filename)
	return nil
}

func waitForProcessExit(pid int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestDarwinKillProcessSignalsProcessGroup(t *testing.T) {
	tmp := t.TempDir()
	pidFile := filepath.Join(tmp, "child.pid")
	script := filepath.Join(tmp, "spawn-child.sh")
	if err := os.WriteFile(script, []byte(
		"#!/bin/sh\n"+
			"sleep 30 &\n"+
			"echo $! > \"$1\"\n"+
			"wait\n"), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(script, pidFile)
	cmd.SysProcAttr = jobSysProcAttr(syscall.SIGKILL)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			killProcess(cmd.Process, syscall.SIGKILL)
			_ = cmd.Wait()
		}
	}()

	childPid, err := strconv.Atoi(strings.TrimSpace(string(waitForFile(t, pidFile))))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(childPid, 0); err != nil {
		t.Fatalf("expected child process %d to be running: %v", childPid, err)
	}

	killProcess(cmd.Process, syscall.SIGKILL)
	if err := cmd.Wait(); err != nil {
		t.Logf("process group leader exited after signal: %v", err)
	}
	waited = true
	if !waitForProcessExit(childPid) {
		t.Fatalf("expected child process %d to exit after process group signal", childPid)
	}
}

func newDarwinMonitorVMemTestRunner(t *testing.T, explicit bool) runner {
	t.Helper()
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return runner{
		job: &exec.Cmd{
			Process: proc,
		},
		ioStats: core.NewIoStatsBuilder(),
		jobInfo: &core.JobInfo{
			Pid:            os.Getpid(),
			MemGB:          1024,
			VMemGB:         4,
			VMemGBExplicit: explicit,
		},
		highMem: core.ObservedMemory{
			Vmem: 8 * 1024 * 1024 * 1024,
		},
	}
}

func runDarwinMonitorVMemTest(t *testing.T, run *runner) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	util.LogTeeWriter(&buf)
	defer func() {
		util.LOGGER = nil
	}()
	lastHeartbeat := time.Now()
	err := run.monitor(&lastHeartbeat)
	return string(bytes.TrimSpace(buf.Bytes())), err
}

func newDarwinMonitorMetadata(t *testing.T) *core.Metadata {
	t.Helper()
	tmp := t.TempDir()
	metadataPath := tmp + "/metadata"
	filesPath := tmp + "/files"
	journalPath := tmp + "/journal"
	for _, path := range []string{metadataPath, filesPath, journalPath} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return core.NewMetadataRunWithJournalPath(
		"STAGE.fork0.chnk0",
		metadataPath,
		filesPath,
		journalPath,
		core.STAGE_TYPE_CHUNK)
}

func TestDarwinMonitorIgnoresImplicitVMemQuota(t *testing.T) {
	run := newDarwinMonitorVMemTestRunner(t, false)
	output, err := runDarwinMonitorVMemTest(t, &run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "physical-footprint vmem threshold") ||
		strings.Contains(output, "address space quota") {
		t.Fatalf("expected implicit Darwin vmem quota to be ignored, got\n%s", output)
	}
}

func TestDarwinMonitorLogsExplicitVMemQuota(t *testing.T) {
	run := newDarwinMonitorVMemTestRunner(t, true)
	output, err := runDarwinMonitorVMemTest(t, &run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "explicit macOS physical-footprint vmem threshold") {
		t.Fatalf("expected explicit Darwin vmem quota message, got\n%s", output)
	}
	if !strings.Contains(output, "macOS does not provide Linux-style virtual address-space enforcement") {
		t.Fatalf("expected Darwin vmem message to explain enforcement limits, got\n%s", output)
	}
}

func TestDarwinMonitorWritesRSSMemViolation(t *testing.T) {
	run := newDarwinMonitorVMemTestRunner(t, true)
	run.metadata = newDarwinMonitorMetadata(t)
	run.jobInfo.MemGB = 1
	run.jobInfo.VMemGB = 2
	run.highMem = core.ObservedMemory{
		Rss:  3 * 1024 * 1024 * 1024,
		Vmem: 4 * 1024 * 1024 * 1024,
	}

	output, err := runDarwinMonitorVMemTest(t, &run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, core.ExceededMemQuotaMessage) {
		t.Fatalf("expected RSS quota message, got\n%s", output)
	}
	if strings.Contains(output, "physical-footprint vmem threshold") {
		t.Fatalf("expected RSS quota branch to take precedence over vmem branch, got\n%s", output)
	}

	var violation core.MemViolationContents
	if err := run.metadata.ReadInto(core.MemViolation, &violation); err != nil {
		t.Fatalf("reading mem_violation: %v", err)
	}
	if violation.MemReservationGB != 1 {
		t.Fatalf("expected mem reservation 1GB, got %g", violation.MemReservationGB)
	}
	if violation.MaxRssBytes != run.highMem.Rss {
		t.Fatalf("expected max RSS %d, got %d", run.highMem.Rss, violation.MaxRssBytes)
	}
}
