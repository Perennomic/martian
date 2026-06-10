//go:build windows
// +build windows

package core

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsLocalProcessSupervisor struct {
	job windows.Handle
}

func newLocalProcessSupervisor() localProcessSupervisor {
	return &windowsLocalProcessSupervisor{}
}

func (self *windowsLocalProcessSupervisor) start(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	self.job = job
	if err := setLocalJobKillOnClose(job); err != nil {
		self.close()
		return err
	}
	if err := cmd.Start(); err != nil {
		self.close()
		return err
	}
	if err := assignLocalProcessToJob(job, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		self.close()
		return err
	}
	return nil
}

func (self *windowsLocalProcessSupervisor) wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (self *windowsLocalProcessSupervisor) close() {
	if self.job != 0 {
		_ = windows.CloseHandle(self.job)
		self.job = 0
	}
}

func setLocalJobKillOnClose(job windows.Handle) error {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)))
	return err
}

func assignLocalProcessToJob(job windows.Handle, pid int) error {
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
