//go:build darwin
// +build darwin

package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/martian-lang/martian/martian/core"
	"github.com/martian-lang/martian/martian/util"
)

// Force the given file to sync.
func syncFile(filename string) {
	if fd, err := syscall.Open(filename, syscall.O_RDONLY, 0); err == nil {
		if err := syscall.Fsync(fd); err != nil {
			util.LogError(err, "mrjob",
				"Error syncing file descriptor for %s", filename)
		}
		syscall.Close(fd)
	}
}

func jobSysProcAttr(_ syscall.Signal) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(proc *os.Process, sig syscall.Signal) {
	if proc == nil {
		return
	}
	if err := syscall.Kill(-proc.Pid, sig); err != nil {
		util.LogError(err, "monitor",
			"Could not signal process group %d. Falling back to process signal.",
			proc.Pid)
		_ = proc.Signal(sig)
	}
}

// setSubreaper is unavailable on Darwin. Stage subprocesses are launched in a
// dedicated process group instead, so monitor-triggered cleanup can signal the
// group even though Darwin cannot adopt orphaned grandchildren like Linux.
func setSubreaper() {}

// Log any child processes which are still up.
//
// Returns true if any child processes were found and reported.
func reportChildren() bool {
	tree, err := core.GetProcessTreeMemoryList(os.Getpid())
	if err != nil {
		util.LogError(err, "monitor", "Error getting process tree.")
		return false
	}
	found := false
	for _, proc := range tree {
		if proc.Depth <= 0 {
			continue
		}
		cmdline := strings.Join(proc.Cmdline, " ")
		if cmdline == "" {
			cmdline = "unknown"
		}
		util.LogInfo("monitor",
			"Orphaned child process %s (%s) is still running.",
			strconv.Itoa(proc.Pid),
			cmdline)
		found = true
	}
	return found
}

// Wait for any orphaned direct children, to collect their rusage.
//
// Returns true if there are any child processes which have not terminated.
func waitChildren() bool {
	var ws syscall.WaitStatus
	wpid, err := syscall.Wait4(-1, &ws, unix.WNOHANG, nil)
	start := time.Now()
	for err == nil && wpid > 0 {
		if ws.Exited() {
			util.LogInfo("monitor",
				"orphaned child process %d terminated with status %d",
				wpid, ws.ExitStatus())
		} else if ws.Signaled() {
			util.LogInfo("monitor",
				"orphaned child process %d got signal %v",
				wpid, ws.Signal())
		} else if time.Since(start) > time.Second {
			break
		} else {
			time.Sleep(time.Millisecond)
		}
		wpid, err = syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
	}
	return err == nil
}
