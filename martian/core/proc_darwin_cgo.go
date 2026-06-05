//go:build darwin && cgo
// +build darwin,cgo

package core

/*
#include <errno.h>
#include <libproc.h>
#include <sys/proc_info.h>
#include <sys/resource.h>

static int martian_proc_pidinfo(int pid, int flavor, uint64_t arg, void *buffer, int buffersize, int *errnum) {
	errno = 0;
	int result = proc_pidinfo(pid, flavor, arg, buffer, buffersize);
	*errnum = errno;
	return result;
}

static int martian_proc_pid_rusage(int pid, int flavor, void *buffer, int *errnum) {
	errno = 0;
	int result = proc_pid_rusage(pid, flavor, (rusage_info_t *)buffer);
	*errnum = errno;
	return result;
}
*/
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"
)

func readDarwinTaskInfo(pid int) (darwinTaskInfo, error) {
	var info C.struct_proc_taskinfo
	var usage C.struct_rusage_info_v6
	var errnum C.int
	result := C.martian_proc_pidinfo(
		C.int(pid),
		C.PROC_PIDTASKINFO,
		0,
		unsafe.Pointer(&info),
		C.int(C.PROC_PIDTASKINFO_SIZE),
		&errnum,
	)
	if result <= 0 {
		if errnum != 0 {
			return darwinTaskInfo{}, syscall.Errno(errnum)
		}
		return darwinTaskInfo{}, syscall.ESRCH
	}
	if result != C.int(C.PROC_PIDTASKINFO_SIZE) {
		return darwinTaskInfo{}, fmt.Errorf(
			"proc_pidinfo(%d, PROC_PIDTASKINFO) returned %d bytes", pid, int(result))
	}

	footprintSize := int64(info.pti_resident_size)
	if C.martian_proc_pid_rusage(C.int(pid), C.RUSAGE_INFO_CURRENT,
		unsafe.Pointer(&usage), &errnum) == 0 {
		footprintSize = int64(usage.ri_phys_footprint)
	}
	return darwinTaskInfo{
		residentSize:  int64(info.pti_resident_size),
		footprintSize: footprintSize,
		threadCount:   int(info.pti_threadnum),
	}, nil
}
