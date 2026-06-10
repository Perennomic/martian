//go:build !windows
// +build !windows

package main

import (
	"os/exec"
	"syscall"

	"github.com/martian-lang/martian/martian/core"
	"github.com/martian-lang/martian/martian/util"
)

type defaultProcessSupervisor struct {
	cmd                   *exec.Cmd
	terminationReasonText string
}

func newProcessSupervisor() processSupervisor {
	return &defaultProcessSupervisor{}
}

func (self *defaultProcessSupervisor) configure(cmd *exec.Cmd, parentDeathSignal syscall.Signal) {
	cmd.SysProcAttr = jobSysProcAttr(parentDeathSignal)
}

func (self *defaultProcessSupervisor) start(cmd *exec.Cmd) error {
	self.cmd = cmd
	return cmd.Start()
}

func (self *defaultProcessSupervisor) startAuxiliary(cmd *exec.Cmd, parentDeathSignal syscall.Signal) error {
	cmd.SysProcAttr = util.Pdeathsig(&syscall.SysProcAttr{}, parentDeathSignal)
	return cmd.Start()
}

func (self *defaultProcessSupervisor) wait() error {
	if self.cmd == nil {
		return nil
	}
	return self.cmd.Wait()
}

func (self *defaultProcessSupervisor) kill(sig syscall.Signal, reason string) {
	if reason != "" {
		self.terminationReasonText = reason
	}
	if self.cmd != nil {
		killProcess(self.cmd.Process, sig)
	}
}

func (self *defaultProcessSupervisor) terminationReason() string {
	return self.terminationReasonText
}

func (self *defaultProcessSupervisor) rusage() *core.RusageInfo {
	return core.GetRusage()
}

func (self *defaultProcessSupervisor) waitChildren() bool {
	return waitChildren()
}

func (self *defaultProcessSupervisor) reportChildren() bool {
	return reportChildren()
}

func (self *defaultProcessSupervisor) close() {}
