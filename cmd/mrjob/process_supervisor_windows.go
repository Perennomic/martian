//go:build windows
// +build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/martian-lang/martian/martian/core"
	"github.com/martian-lang/martian/martian/util"
)

type windowsProcessSupervisor struct {
	cmd                   *exec.Cmd
	job                   windows.Handle
	terminationReasonText string
}

type windowsJobObjectBasicAccountingInformation struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

type windowsJobObjectBasicAndIoAccountingInformation struct {
	BasicInfo windowsJobObjectBasicAccountingInformation
	IoInfo    windows.IO_COUNTERS
}

func newProcessSupervisor() processSupervisor {
	return &windowsProcessSupervisor{}
}

func (self *windowsProcessSupervisor) configure(cmd *exec.Cmd, _ syscall.Signal) {}

func (self *windowsProcessSupervisor) start(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	self.job = job
	if err := setKillOnJobClose(job); err != nil {
		self.close()
		return err
	}
	self.cmd = cmd
	if err := cmd.Start(); err != nil {
		self.close()
		return err
	}
	// Go's os/exec does not expose the primary thread handle needed to
	// CREATE_SUSPENDED, assign to the job, and ResumeThread without
	// reimplementing process creation and handle inheritance. Assign
	// immediately after Start for the MVP, and validate the remaining spawn
	// race on a Windows runner before claiming full process-tree coverage.
	if err := assignProcessToJob(job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		self.close()
		return err
	}
	return nil
}

func (self *windowsProcessSupervisor) startAuxiliary(cmd *exec.Cmd, _ syscall.Signal) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	if self.job == 0 {
		return nil
	}
	if err := assignProcessToJob(self.job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	return nil
}

func setKillOnJobClose(job windows.Handle) error {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))
	return err
}

func assignProcessToJob(job windows.Handle, pid int) error {
	proc, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(proc)
	return windows.AssignProcessToJobObject(job, proc)
}

func (self *windowsProcessSupervisor) wait() error {
	if self.cmd == nil {
		return nil
	}
	return self.cmd.Wait()
}

func (self *windowsProcessSupervisor) kill(_ syscall.Signal, reason string) {
	if reason != "" {
		self.terminationReasonText = reason
	}
	if self.job != 0 {
		if err := windows.TerminateJobObject(self.job, 1); err != nil {
			util.LogError(err, "monitor", "Could not terminate Windows Job Object.")
		}
	} else if self.cmd != nil && self.cmd.Process != nil {
		_ = self.cmd.Process.Kill()
	}
}

func (self *windowsProcessSupervisor) terminationReason() string {
	return self.terminationReasonText
}

func (self *windowsProcessSupervisor) rusage() *core.RusageInfo {
	result := core.GetRusage()
	if result == nil {
		result = &core.RusageInfo{}
	}
	if result.Self == nil {
		result.Self = &core.Rusage{}
	}
	result.Children = &core.Rusage{}
	if self.job == 0 {
		return result
	}
	info, err := self.queryJobAccounting()
	if err != nil {
		util.LogError(err, "monitor", "Could not query Windows Job Object accounting.")
		return result
	}
	result.Children = windowsJobAccountingInfoToRusage(info)
	return result
}

func (self *windowsProcessSupervisor) queryJobAccounting() (*windowsJobObjectBasicAndIoAccountingInformation, error) {
	var info windowsJobObjectBasicAndIoAccountingInformation
	var retLen uint32
	err := windows.QueryInformationJobObject(
		self.job,
		windows.JobObjectBasicAndIoAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&retLen)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func windowsJobAccountingInfoToRusage(info *windowsJobObjectBasicAndIoAccountingInformation) *core.Rusage {
	return &core.Rusage{
		MinorFaults: int(info.BasicInfo.TotalPageFaultCount),
		UserTime:    windowsJobObjectTimeSeconds(info.BasicInfo.TotalUserTime),
		SystemTime:  windowsJobObjectTimeSeconds(info.BasicInfo.TotalKernelTime),
		InBlocks:    int(info.IoInfo.ReadOperationCount),
		OutBlocks:   int(info.IoInfo.WriteOperationCount),
	}
}

func windowsJobObjectTimeSeconds(ticks100ns int64) float64 {
	return float64(ticks100ns) / 1e7
}

func (self *windowsProcessSupervisor) waitChildren() bool {
	return false
}

func (self *windowsProcessSupervisor) reportChildren() bool {
	return false
}

func (self *windowsProcessSupervisor) close() {
	if self.job != 0 {
		_ = windows.CloseHandle(self.job)
		self.job = 0
	}
}
