//go:build windows
// +build windows

package core

import (
	"os"
	"testing"
)

func TestWindowsProcessTableIncludesCurrentProcess(t *testing.T) {
	processes, _, err := getWindowsProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	proc, ok := processes[os.Getpid()]
	if !ok {
		t.Fatalf("expected process table to include current pid %d", os.Getpid())
	}
	if proc.threads <= 0 {
		t.Fatalf("expected current process to report at least one thread, got %d", proc.threads)
	}
	if countWindowsProcessThreads(processes) < proc.threads {
		t.Fatalf("expected process thread count to include current process threads")
	}
}

func TestGetRunningMemoryWindows(t *testing.T) {
	mem, err := GetRunningMemory(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if mem.Rss <= 0 {
		t.Fatalf("expected nonzero Windows working set, got %d", mem.Rss)
	}
	if mem.WorkingSet != mem.Rss {
		t.Fatalf("expected explicit working set to match compatibility rss, got working_set=%d rss=%d",
			mem.WorkingSet, mem.Rss)
	}
	if mem.Vmem <= 0 {
		t.Fatalf("expected nonzero Windows private bytes/commit, got %d", mem.Vmem)
	}
	if mem.PrivateBytes != mem.Vmem {
		t.Fatalf("expected explicit private bytes to match compatibility vmem, got private_bytes=%d vmem=%d",
			mem.PrivateBytes, mem.Vmem)
	}
	if mem.PeakWorkingSet < mem.WorkingSet {
		t.Fatalf("expected peak working set >= working set, got peak=%d current=%d",
			mem.PeakWorkingSet, mem.WorkingSet)
	}
	if mem.PeakPrivateBytes < mem.PrivateBytes {
		t.Fatalf("expected peak private bytes/commit >= private bytes/commit, got peak=%d current=%d",
			mem.PeakPrivateBytes, mem.PrivateBytes)
	}
	if mem.Procs <= 0 {
		t.Fatalf("expected nonzero thread count, got %d", mem.Procs)
	}
}

func TestGetProcessTreeMemoryWindowsIncludesParent(t *testing.T) {
	io := make(map[int]*IoAmount)
	mem, err := GetProcessTreeMemory(os.Getpid(), true, io)
	if err != nil {
		t.Fatal(err)
	}
	if mem.Rss <= 0 {
		t.Fatalf("expected nonzero Windows process-tree working set, got %d", mem.Rss)
	}
	if mem.Procs <= 0 {
		t.Fatalf("expected nonzero Windows process-tree thread count, got %d", mem.Procs)
	}
	if _, ok := io[os.Getpid()]; !ok {
		t.Fatalf("expected current pid IO counters to be present")
	}
}

func TestGetProcessTreeMemoryListWindows(t *testing.T) {
	tree, err := GetProcessTreeMemoryList(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) == 0 {
		t.Fatal("expected non-empty process tree")
	}
	if tree[0].Pid != os.Getpid() {
		t.Fatalf("expected current pid first, got %d", tree[0].Pid)
	}
	if tree[0].Memory.Rss <= 0 {
		t.Fatalf("expected current process working set, got %d", tree[0].Memory.Rss)
	}
}

func TestGetRusageWindows(t *testing.T) {
	ru := GetRusage()
	if ru == nil || ru.Self == nil || ru.Children == nil {
		t.Fatalf("expected self and empty children rusage on Windows")
	}
	if ru.Self.MaxRss <= 0 {
		t.Fatalf("expected nonzero peak working set in MaxRss, got %d", ru.Self.MaxRss)
	}
	if totalCpu(ru) < 0 {
		t.Fatalf("expected non-negative CPU time")
	}
}

func totalCpu(ru *RusageInfo) float64 {
	if ru == nil {
		return 0
	}
	var total float64
	if ru.Self != nil {
		total += ru.Self.UserTime + ru.Self.SystemTime
	}
	if ru.Children != nil {
		total += ru.Children.UserTime + ru.Children.SystemTime
	}
	return total
}
