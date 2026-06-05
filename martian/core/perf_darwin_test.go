//go:build darwin
// +build darwin

package core

import (
	"runtime"
	"syscall"
	"testing"
)

func TestCountDarwinUserThreadsUsesThreadCounts(t *testing.T) {
	processes := map[int]darwinProcess{
		11: {pid: 11, uid: 501},
		12: {pid: 12, uid: 501},
		13: {pid: 13, uid: 502},
		14: {pid: 14, uid: 501},
	}
	got := countDarwinUserThreads(processes, 501,
		func(pid int) (darwinTaskInfo, error) {
			switch pid {
			case 11:
				return darwinTaskInfo{threadCount: 3}, nil
			case 12:
				return darwinTaskInfo{threadCount: 7}, nil
			case 14:
				return darwinTaskInfo{}, syscall.ESRCH
			default:
				return darwinTaskInfo{threadCount: 100}, nil
			}
		})
	if got != 10 {
		t.Fatalf("expected darwin user thread count 10, got %d", got)
	}
}

func TestGetRusageNormalizesDarwinMaxRss(t *testing.T) {
	buf := make([]byte, 4<<20)
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = 1
	}
	runtime.KeepAlive(buf)

	var rawBefore syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rawBefore); err != nil {
		t.Fatal(err)
	}
	if rawBefore.Maxrss <= 0 {
		t.Fatalf("expected raw darwin ru_maxrss to be non-zero, got %d", rawBefore.Maxrss)
	}

	got := GetRusage()
	if got == nil || got.Self == nil {
		t.Fatal("expected self rusage on darwin")
	}

	var rawAfter syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rawAfter); err != nil {
		t.Fatal(err)
	}

	minKiB := normalizeDarwinMaxRss(rawBefore.Maxrss)
	maxKiB := normalizeDarwinMaxRss(rawAfter.Maxrss)
	if got.Self.MaxRss < minKiB || got.Self.MaxRss > maxKiB {
		t.Fatalf("expected normalized ru_maxrss to stay within [%d, %d] KiB, got %d KiB (before=%d bytes after=%d bytes)",
			minKiB, maxKiB, got.Self.MaxRss, rawBefore.Maxrss, rawAfter.Maxrss)
	}

	var observed ObservedMemory
	observed.IncreaseRusage(got)
	if observed.Rss <= 0 {
		t.Fatal("expected non-zero observed rss after IncreaseRusage")
	}
	minBytes := int64(minKiB) * 1024
	maxBytes := int64(maxKiB) * 1024
	if observed.Rss < minBytes || observed.Rss > maxBytes {
		t.Fatalf("expected normalized rss to stay within [%d, %d] bytes, got %d bytes",
			minBytes, maxBytes, observed.Rss)
	}
}
