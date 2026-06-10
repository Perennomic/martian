//go:build windows
// +build windows

package core

import (
	"syscall"
	"unsafe"
)

var procGlobalMemoryStatusEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

type MemInfo struct {
	Total      int64
	Used       int64
	Free       int64
	ActualFree int64
	ActualUsed int64
}

func (m *MemInfo) Get() error {
	*m = MemInfo{}
	status := memoryStatusEx{
		length: uint32(unsafe.Sizeof(memoryStatusEx{})),
	}
	if err := globalMemoryStatusEx(&status); err != nil {
		return err
	}
	m.Total = int64(status.totalPhys)
	m.Free = int64(status.availPhys)
	m.ActualFree = int64(status.availPhys)
	m.Used = m.Total - m.Free
	m.ActualUsed = m.Total - m.ActualFree
	return nil
}

func globalMemoryStatusEx(status *memoryStatusEx) error {
	ret, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(status)))
	if ret == 0 {
		return err
	}
	return nil
}
