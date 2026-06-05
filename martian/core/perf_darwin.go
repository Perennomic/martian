//
// Converts Darwin process resource information into our structures.
//
//go:build darwin
// +build darwin

package core

import (
	"errors"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type darwinTaskInfo struct {
	residentSize  int64
	footprintSize int64
	threadCount   int
}

type darwinProcess struct {
	pid     int
	parent  int
	uid     uint32
	command string
}

func normalizeDarwinMaxRss(maxrss int64) int {
	if maxrss <= 0 {
		return 0
	}
	return int(maxrss / 1024)
}

func getRusage(who int) *Rusage {
	var ru syscall.Rusage
	if err := syscall.Getrusage(who, &ru); err == nil {
		return &Rusage{
			MaxRss:       normalizeDarwinMaxRss(ru.Maxrss),
			SharedRss:    int(ru.Ixrss),
			UnsharedRss:  int(ru.Idrss),
			MinorFaults:  int(ru.Minflt),
			MajorFaults:  int(ru.Majflt),
			SwapOuts:     int(ru.Nswap),
			UserTime:     time.Duration(ru.Utime.Nano()).Seconds(),
			SystemTime:   time.Duration(ru.Stime.Nano()).Seconds(),
			InBlocks:     int(ru.Inblock),
			OutBlocks:    int(ru.Oublock),
			MessagesSent: int(ru.Msgsnd),
			MessagesRcvd: int(ru.Msgrcv),
			SignalsRcvd:  int(ru.Nsignals),
			CtxSwitches:  int(ru.Nivcsw),
		}
	}
	return nil
}

func GetRusage() *RusageInfo {
	ru := RusageInfo{
		Self:     getRusage(syscall.RUSAGE_SELF),
		Children: getRusage(syscall.RUSAGE_CHILDREN),
	}
	if ru.Self == nil && ru.Children == nil {
		return nil
	}
	return &ru
}

func countDarwinUserThreads(processes map[int]darwinProcess, uid uint32,
	infoFn func(int) (darwinTaskInfo, error)) int {
	count := 0
	for _, process := range processes {
		if process.uid != uid {
			continue
		}
		if info, err := infoFn(process.pid); err == nil {
			if info.threadCount > 0 {
				count += info.threadCount
			}
		}
	}
	return count
}

// Get the number of processes currently running for the current user.
//
// Note: On macOS, RLIMIT_NPROC limits the number of threads (schedulable entities),
// not just processes. This function is used to accurately calculate the remaining
// RLIMIT_NPROC quota for resource management (e.g., LocalJobManager.procsSem),
// ensuring that thread creation limits are enforced correctly.
func GetUserProcessCount() (int, error) {
	processes, _, err := getDarwinProcessTable()
	if err != nil {
		return 0, err
	}
	return countDarwinUserThreads(processes, uint32(syscall.Getuid()), readDarwinTaskInfo), nil
}

// Gets the total memory usage for the given process and all of its
// children. Only errors getting the first process's memory, or the
// process table, are reported. includeParent specifies whether the
// top-level pid is included in the total.
func GetProcessTreeMemory(pid int, includeParent bool, io map[int]*IoAmount) (mem ObservedMemory, err error) {
	processes, children, err := getDarwinProcessTable()
	if err != nil {
		return mem, err
	}
	if _, ok := processes[pid]; !ok {
		return mem, syscall.ESRCH
	}
	if includeParent {
		return getDarwinProcessTreeMemory(pid, children, io)
	}
	for _, childPid := range children[pid] {
		if childMem, err := getDarwinProcessTreeMemory(childPid, children, io); err == nil {
			mem.Add(childMem)
		}
	}
	return mem, nil
}

// GetProcessTreeMemoryList returns the memory usage and other stats for all
// processes in the tree rooted with pid.
func GetProcessTreeMemoryList(pid int) (ProcessTree, error) {
	processes, children, err := getDarwinProcessTable()
	if err != nil {
		return nil, err
	}
	if _, ok := processes[pid]; !ok {
		return nil, syscall.ESRCH
	}
	var stats ProcessTree
	if err := appendDarwinProcessTreeMemoryList(pid, 0, processes, children, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// Gets the observed memory of a running process by pid.
func GetRunningMemory(pid int) (ObservedMemory, error) {
	info, err := readDarwinTaskInfo(pid)
	if err != nil {
		return ObservedMemory{}, err
	}
	vmem := info.footprintSize
	if vmem < info.residentSize {
		vmem = info.residentSize
	}
	mem := ObservedMemory{
		Rss: info.residentSize,
		// Darwin virtual_size can include very large reserved address space,
		// especially on arm64. Use physical footprint as the VMem proxy.
		Vmem: vmem,
	}
	if info.threadCount > 0 {
		mem.Procs = info.threadCount
	}
	return mem, nil
}

// Gets IO statistics for a running process by pid.
func GetRunningIo(pid int) (*IoAmount, error) {
	return nil, errors.New("not supported")
}

func getDarwinProcessTable() (map[int]darwinProcess, map[int][]int, error) {
	processList, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, nil, err
	}
	processes := make(map[int]darwinProcess, len(processList))
	children := make(map[int][]int)
	for _, proc := range processList {
		pid := int(proc.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		entry := darwinProcess{
			pid:     pid,
			parent:  int(proc.Eproc.Ppid),
			uid:     proc.Eproc.Ucred.Uid,
			command: strings.TrimRight(string(proc.Proc.P_comm[:]), "\x00"),
		}
		processes[pid] = entry
		if entry.parent > 0 {
			children[entry.parent] = append(children[entry.parent], pid)
		}
	}
	for parent := range children {
		sort.Ints(children[parent])
	}
	return processes, children, nil
}

func getDarwinProcessTreeMemory(pid int, children map[int][]int, io map[int]*IoAmount) (ObservedMemory, error) {
	_ = io
	mem, err := GetRunningMemory(pid)
	if err != nil {
		return ObservedMemory{}, err
	}
	for _, childPid := range children[pid] {
		childMem, err := getDarwinProcessTreeMemory(childPid, children, io)
		if err == nil {
			mem.Add(childMem)
		}
	}
	return mem, nil
}

func appendDarwinProcessTreeMemoryList(pid int, depth int,
	processes map[int]darwinProcess, children map[int][]int, stats *ProcessTree) error {
	mem, err := GetRunningMemory(pid)
	if err != nil {
		return err
	}
	stat := ProcessStats{
		Pid:    pid,
		Memory: mem,
		Depth:  depth,
	}
	if proc, ok := processes[pid]; ok && proc.command != "" {
		stat.Cmdline = []string{proc.command}
	}
	*stats = append(*stats, stat)
	for _, childPid := range children[pid] {
		_ = appendDarwinProcessTreeMemoryList(childPid, depth+1, processes, children, stats)
	}
	return nil
}
