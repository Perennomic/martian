//go:build darwin && cgo
// +build darwin,cgo

//  Utility method to read Darwin memory statistics.

package core

/*
#include <errno.h>
#include <mach/host_info.h>
#include <mach/mach.h>
#include <sys/sysctl.h>

static int martian_sysctl_hw_memsize(uint64_t *value, int *errnum) {
	size_t size = sizeof(*value);
	errno = 0;
	int result = sysctlbyname("hw.memsize", value, &size, NULL, 0);
	*errnum = errno;
	return result;
}

static kern_return_t martian_host_vm_info(vm_statistics64_data_t *vmstat, vm_size_t *page_size) {
	mach_msg_type_number_t count = HOST_VM_INFO64_COUNT;
	mach_port_t host = mach_host_self();
	kern_return_t kr = host_statistics64(host, HOST_VM_INFO64, (host_info64_t)vmstat, &count);
	if (kr == KERN_SUCCESS) {
		kr = host_page_size(host, page_size);
	}
	mach_port_deallocate(mach_task_self(), host);
	return kr;
}
*/
import "C"

import (
	"fmt"
	"syscall"
)

type MemInfo struct {
	Total      int64
	Used       int64
	Free       int64
	ActualFree int64
	ActualUsed int64
}

func (m *MemInfo) Get() error {
	*m = MemInfo{}

	var total C.uint64_t
	var errnum C.int
	if C.martian_sysctl_hw_memsize(&total, &errnum) != 0 {
		if errnum != 0 {
			return syscall.Errno(errnum)
		}
		return fmt.Errorf("sysctlbyname(hw.memsize) failed")
	}

	var vmstat C.vm_statistics64_data_t
	var pageSize C.vm_size_t
	if kr := C.martian_host_vm_info(&vmstat, &pageSize); kr != C.KERN_SUCCESS {
		return fmt.Errorf("host_statistics64(HOST_VM_INFO64) failed: %d", int(kr))
	}

	pageBytes := int64(pageSize)
	free := int64(vmstat.free_count) * pageBytes
	speculative := int64(vmstat.speculative_count) * pageBytes
	inactive := int64(vmstat.inactive_count) * pageBytes

	m.Total = int64(total)
	m.Free = free - speculative
	if m.Free < 0 {
		m.Free = 0
	}

	m.ActualFree = free + inactive
	if m.ActualFree < 0 {
		m.ActualFree = 0
	} else if m.ActualFree > m.Total {
		m.ActualFree = m.Total
	}

	m.Used = m.Total - m.Free
	m.ActualUsed = m.Total - m.ActualFree
	return nil
}
