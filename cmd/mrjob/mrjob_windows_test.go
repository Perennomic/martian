//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/martian-lang/martian/martian/core"
)

func TestWindowsSyncFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "sync.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	syncFile(file)
	syncFile(tmp)
	syncFile(filepath.Join(tmp, "missing"))
}

func TestWindowsMrjobCompletesStage(t *testing.T) {
	metadata := runWindowsMrjobSmoke(t, "success")
	if _, err := os.Stat(metadata.MetadataFilePath(core.CompleteFile)); err != nil {
		t.Fatalf("expected complete file: %v", err)
	}
	if _, err := os.Stat(metadata.MetadataFilePath(core.Errors)); !os.IsNotExist(err) {
		t.Fatalf("unexpected errors file: %v", err)
	}
	var jobInfo core.JobInfo
	if err := metadata.ReadInto(core.JobInfoFile, &jobInfo); err != nil {
		t.Fatal(err)
	}
	if jobInfo.Pid == 0 {
		t.Fatal("expected mrjob to write its pid to jobinfo")
	}
	if jobInfo.WallClockInfo == nil {
		t.Fatal("expected final jobinfo to include wallclock data")
	}
}

func TestWindowsMrjobRecordsStageError(t *testing.T) {
	metadata := runWindowsMrjobSmoke(t, "error")
	if _, err := os.Stat(metadata.MetadataFilePath(core.CompleteFile)); !os.IsNotExist(err) {
		t.Fatalf("unexpected complete file: %v", err)
	}
	contents, err := os.ReadFile(metadata.MetadataFilePath(core.Errors))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "stage helper error") {
		t.Fatalf("expected stage error metadata, got %q", string(contents))
	}
}

func TestWindowsMrjobRecordsStageExitFailure(t *testing.T) {
	metadata := runWindowsMrjobSmoke(t, "exit")
	if _, err := os.Stat(metadata.MetadataFilePath(core.CompleteFile)); !os.IsNotExist(err) {
		t.Fatalf("unexpected complete file: %v", err)
	}
	contents, err := os.ReadFile(metadata.MetadataFilePath(core.Errors))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "Job failed in stage code") ||
		!strings.Contains(text, "exit status 7") {
		t.Fatalf("expected nonzero exit metadata, got %q", text)
	}
}

func TestWindowsMrjobMonitorKillRecordsReason(t *testing.T) {
	metadata := runWindowsMrjobSmoke(t, "monitor")
	if _, err := os.Stat(metadata.MetadataFilePath(core.CompleteFile)); !os.IsNotExist(err) {
		t.Fatalf("unexpected complete file: %v", err)
	}
	contents, err := os.ReadFile(metadata.MetadataFilePath(core.Errors))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), core.ExceededMemQuotaMessage) {
		t.Fatalf("expected monitor kill metadata, got %q", string(contents))
	}
	if _, err := os.Stat(metadata.MetadataFilePath(core.MemViolation)); err != nil {
		t.Fatalf("expected mem violation file: %v", err)
	}
	var jobInfo core.JobInfo
	if err := metadata.ReadInto(core.JobInfoFile, &jobInfo); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jobInfo.SupervisorReason, core.ExceededMemQuotaMessage) {
		t.Fatalf("expected supervisor reason to describe monitor kill, got %q", jobInfo.SupervisorReason)
	}
}

func TestWindowsMrjobHandleSignalRecordsCancel(t *testing.T) {
	tmp := t.TempDir()
	metadataPath := filepath.Join(tmp, "metadata")
	filesPath := filepath.Join(tmp, "files")
	journalPath := filepath.Join(tmp, "journal")
	for _, dir := range []string{metadataPath, filesPath, journalPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	metadata := core.NewMetadataRunWithJournalPath(
		"stage", metadataPath, filesPath, journalPath, "main")
	if err := metadata.WriteAtomic(core.JobInfoFile, core.JobInfo{Name: "stage"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	close(done)
	supervisor := &recordingProcessSupervisor{}
	run := &runner{
		start:      time.Now(),
		job:        &exec.Cmd{},
		ioStats:    core.NewIoStatsBuilder(),
		metadata:   metadata,
		jobInfo:    &core.JobInfo{},
		supervisor: supervisor,
		isDone:     done,
	}

	run.HandleSignal(os.Interrupt)

	if supervisor.killReason != "caught signal interrupt" {
		t.Fatalf("expected cancel kill reason, got %q", supervisor.killReason)
	}
	contents, err := os.ReadFile(metadata.MetadataFilePath(core.Errors))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "Caught signal interrupt") {
		t.Fatalf("expected cancel error metadata, got %q", string(contents))
	}
	var jobInfo core.JobInfo
	if err := metadata.ReadInto(core.JobInfoFile, &jobInfo); err != nil {
		t.Fatal(err)
	}
	if jobInfo.SupervisorReason != "caught signal interrupt" {
		t.Fatalf("expected supervisor reason in jobinfo, got %q", jobInfo.SupervisorReason)
	}
}

func TestWindowsMrjobMainHelper(t *testing.T) {
	if os.Getenv("MRO_MRJOB_MAIN_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"mrjob"}, os.Args[i+1:]...)
			main()
			os.Exit(2)
		}
	}
	t.Fatal("missing helper argument separator")
}

func TestWindowsMrjobStageHelper(t *testing.T) {
	switch os.Getenv("MRO_MRJOB_STAGE_HELPER") {
	case "":
		return
	case "success":
		return
	case "error":
		errorPath := os.Getenv("MRO_CONTROL_ERROR_PATH")
		if errorPath == "" {
			t.Fatal("missing MRO_CONTROL_ERROR_PATH")
		}
		if err := os.WriteFile(errorPath, []byte("stage helper error"), 0644); err != nil {
			t.Fatal(err)
		}
	case "exit":
		os.Exit(7)
	case "monitor":
		time.Sleep(30 * time.Second)
	default:
		t.Fatalf("unknown helper mode %q", os.Getenv("MRO_MRJOB_STAGE_HELPER"))
	}
}

func runWindowsMrjobSmoke(t *testing.T, mode string) *core.Metadata {
	t.Helper()

	tmp := t.TempDir()
	metadataPath := filepath.Join(tmp, "metadata")
	filesPath := filepath.Join(tmp, "files")
	journalPath := filepath.Join(tmp, "journal")
	for _, dir := range []string{metadataPath, filesPath, journalPath} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	metadata := core.NewMetadataRunWithJournalPath(
		"stage", metadataPath, filesPath, journalPath, "main")
	initial := core.JobInfo{
		Name:    "stage",
		Threads: 1,
		MemGB:   1,
	}
	if mode == "monitor" {
		initial.Monitor = "monitor"
		initial.MemGB = 0
	}
	if err := metadata.WriteAtomic(core.JobInfoFile, initial); err != nil {
		t.Fatal(err)
	}

	args := []string{
		"-test.run=TestWindowsMrjobMainHelper",
		"--",
		os.Args[0],
		"-test.run=TestWindowsMrjobStageHelper",
		"--",
		"main",
		metadataPath,
		filesPath,
		filepath.Join(journalPath, "stage"),
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(),
		"MRO_MRJOB_MAIN_HELPER=1",
		"MRO_MRJOB_STAGE_HELPER="+mode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mrjob helper failed: %v\n%s", err, output)
	}
	if !json.Valid(mustReadFile(t, metadata.MetadataFilePath(core.JobInfoFile))) {
		t.Fatal("expected jobinfo to remain valid json")
	}
	return metadata
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

type recordingProcessSupervisor struct {
	killReason string
}

func (self *recordingProcessSupervisor) configure(*exec.Cmd, syscall.Signal) {}

func (self *recordingProcessSupervisor) start(*exec.Cmd) error {
	return nil
}

func (self *recordingProcessSupervisor) startAuxiliary(*exec.Cmd, syscall.Signal) error {
	return nil
}

func (self *recordingProcessSupervisor) wait() error {
	return nil
}

func (self *recordingProcessSupervisor) kill(_ syscall.Signal, reason string) {
	self.killReason = reason
}

func (self *recordingProcessSupervisor) terminationReason() string {
	return self.killReason
}

func (self *recordingProcessSupervisor) rusage() *core.RusageInfo {
	return &core.RusageInfo{}
}

func (self *recordingProcessSupervisor) waitChildren() bool {
	return false
}

func (self *recordingProcessSupervisor) reportChildren() bool {
	return false
}

func (self *recordingProcessSupervisor) close() {}
