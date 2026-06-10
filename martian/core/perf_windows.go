//go:build windows
// +build windows

package core

import (
	"sort"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
	procGetProcessIoCounters = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessIoCounters")
)

type windowsProcessMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type windowsIoCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type windowsProcess struct {
	pid     int
	parent  int
	threads int
	command string
}

func GetRusage() *RusageInfo {
	process, err := windows.GetCurrentProcess()
	if err != nil {
		return nil
	}
	ru, err := getWindowsProcessRusage(process)
	if err != nil {
		return nil
	}
	return &RusageInfo{
		Self:     ru,
		Children: &Rusage{},
	}
}

func getWindowsProcessRusage(process windows.Handle) (*Rusage, error) {
	mem, err := getWindowsProcessMemoryInfo(process)
	if err != nil {
		return nil, err
	}
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	if err := windows.GetProcessTimes(process, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return nil, err
	}
	return &Rusage{
		MaxRss:     int(sizeToInt64(mem.PeakWorkingSetSize) / 1024),
		UserTime:   windowsFiletimeDurationSeconds(userTime),
		SystemTime: windowsFiletimeDurationSeconds(kernelTime),
	}, nil
}

// Get the number of process threads currently visible on the system.
func GetUserProcessCount() (int, error) {
	processes, _, err := getWindowsProcessTable()
	if err != nil {
		return 0, err
	}
	return countWindowsProcessThreads(processes), nil
}

func countWindowsProcessThreads(processes map[int]windowsProcess) int {
	count := 0
	for _, process := range processes {
		count += process.threads
	}
	return count
}

// Gets the total memory usage for the given process and all of its children.
// On Windows, ObservedMemory.Rss is the working set and ObservedMemory.Vmem is
// private bytes/commit, not Linux virtual address space size.
func GetProcessTreeMemory(pid int, includeParent bool, io map[int]*IoAmount) (mem ObservedMemory, err error) {
	processes, children, err := getWindowsProcessTable()
	if err != nil {
		return mem, err
	}
	if _, ok := processes[pid]; !ok {
		return mem, syscall.ESRCH
	}
	if includeParent {
		return getWindowsProcessTreeMemory(pid, processes, children, io)
	}
	for _, childPid := range children[pid] {
		if childMem, err := getWindowsProcessTreeMemory(childPid, processes, children, io); err == nil {
			mem.Add(childMem)
		}
	}
	return mem, nil
}

// GetProcessTreeMemoryList returns the memory usage and other stats for all
// processes in the tree rooted with pid.
func GetProcessTreeMemoryList(pid int) (ProcessTree, error) {
	processes, children, err := getWindowsProcessTable()
	if err != nil {
		return nil, err
	}
	if _, ok := processes[pid]; !ok {
		return nil, syscall.ESRCH
	}
	var stats ProcessTree
	if err := appendWindowsProcessTreeMemoryList(pid, 0, processes, children, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// Gets the observed memory of a running process by pid.
func GetRunningMemory(pid int) (ObservedMemory, error) {
	process, err := openWindowsProcess(pid)
	if err != nil {
		return ObservedMemory{}, err
	}
	defer windows.CloseHandle(process)
	mem, err := getWindowsRunningMemory(process)
	if err != nil {
		return ObservedMemory{}, err
	}
	if processes, _, err := getWindowsProcessTable(); err == nil {
		if proc, ok := processes[pid]; ok {
			mem.Procs = proc.threads
		}
	}
	return mem, nil
}

// Gets IO statistics for a running process by pid.
func GetRunningIo(pid int) (*IoAmount, error) {
	process, err := openWindowsProcess(pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(process)
	io, err := getWindowsRunningIo(process)
	return &io, err
}

func getWindowsProcessTable() (map[int]windowsProcess, map[int][]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, nil, err
	}
	defer windows.CloseHandle(snapshot)

	processes := make(map[int]windowsProcess)
	children := make(map[int][]int)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, nil, err
	}
	for {
		pid := int(entry.ProcessID)
		if pid > 0 {
			process := windowsProcess{
				pid:     pid,
				parent:  int(entry.ParentProcessID),
				threads: int(entry.Threads),
				command: windows.UTF16ToString(entry.ExeFile[:]),
			}
			processes[pid] = process
			if process.parent > 0 {
				children[process.parent] = append(children[process.parent], pid)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, nil, err
		}
	}
	for parent := range children {
		sort.Ints(children[parent])
	}
	return processes, children, nil
}

func getWindowsProcessTreeMemory(pid int, processes map[int]windowsProcess,
	children map[int][]int, io map[int]*IoAmount) (ObservedMemory, error) {
	proc := processes[pid]
	mem, err := getWindowsRunningMemoryByPid(pid, proc)
	if err != nil {
		return ObservedMemory{}, err
	}
	if io != nil {
		if processIo, err := GetRunningIo(pid); err == nil {
			io[pid] = processIo
		}
	}
	for _, childPid := range children[pid] {
		if childMem, err := getWindowsProcessTreeMemory(childPid, processes, children, io); err == nil {
			mem.Add(childMem)
		}
	}
	return mem, nil
}

func appendWindowsProcessTreeMemoryList(pid int, depth int,
	processes map[int]windowsProcess, children map[int][]int, stats *ProcessTree) error {
	proc := processes[pid]
	mem, err := getWindowsRunningMemoryByPid(pid, proc)
	if err != nil {
		return err
	}
	stat := ProcessStats{
		Pid:    pid,
		Memory: mem,
		Depth:  depth,
	}
	if proc.command != "" {
		stat.Cmdline = []string{proc.command}
	}
	if io, err := GetRunningIo(pid); err == nil {
		stat.IO = *io
	}
	*stats = append(*stats, stat)
	for _, childPid := range children[pid] {
		_ = appendWindowsProcessTreeMemoryList(childPid, depth+1, processes, children, stats)
	}
	return nil
}

func getWindowsRunningMemoryByPid(pid int, proc windowsProcess) (ObservedMemory, error) {
	process, err := openWindowsProcess(pid)
	if err != nil {
		return ObservedMemory{}, err
	}
	defer windows.CloseHandle(process)
	mem, err := getWindowsRunningMemory(process)
	if err != nil {
		return ObservedMemory{}, err
	}
	mem.Procs = proc.threads
	return mem, nil
}

func getWindowsRunningMemory(process windows.Handle) (ObservedMemory, error) {
	counters, err := getWindowsProcessMemoryInfo(process)
	if err != nil {
		return ObservedMemory{}, err
	}
	workingSet := sizeToInt64(counters.WorkingSetSize)
	privateBytes := sizeToInt64(counters.PrivateUsage)
	return ObservedMemory{
		Rss:              workingSet,
		Vmem:             privateBytes,
		WorkingSet:       workingSet,
		PeakWorkingSet:   sizeToInt64(counters.PeakWorkingSetSize),
		PrivateBytes:     privateBytes,
		PeakPrivateBytes: sizeToInt64(counters.PeakPagefileUsage),
	}, nil
}

func getWindowsProcessMemoryInfo(process windows.Handle) (*windowsProcessMemoryCountersEx, error) {
	counters := windowsProcessMemoryCountersEx{
		Cb: uint32(unsafe.Sizeof(windowsProcessMemoryCountersEx{})),
	}
	ret, _, err := procGetProcessMemoryInfo.Call(
		uintptr(process),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Cb))
	if ret == 0 {
		return nil, err
	}
	return &counters, nil
}

func getWindowsRunningIo(process windows.Handle) (IoAmount, error) {
	var counters windowsIoCounters
	ret, _, err := procGetProcessIoCounters.Call(
		uintptr(process),
		uintptr(unsafe.Pointer(&counters)))
	if ret == 0 {
		return IoAmount{}, err
	}
	return IoAmount{
		Read: IoValues{
			Syscalls:   int64(counters.ReadOperationCount),
			BlockBytes: int64(counters.ReadTransferCount),
		},
		Write: IoValues{
			Syscalls:   int64(counters.WriteOperationCount),
			BlockBytes: int64(counters.WriteTransferCount),
		},
	}, nil
}

func openWindowsProcess(pid int) (windows.Handle, error) {
	return windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		uint32(pid))
}

func sizeToInt64(v uintptr) int64 {
	return int64(uint64(v))
}

func windowsFiletimeDurationSeconds(ft windows.Filetime) float64 {
	ticks := uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
	return float64(ticks) / 1e7
}
