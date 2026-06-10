//go:build !windows
// +build !windows

package core

import (
	"os/exec"
	"syscall"

	"github.com/martian-lang/martian/martian/util"
)

type defaultLocalProcessSupervisor struct{}

func newLocalProcessSupervisor() localProcessSupervisor {
	return defaultLocalProcessSupervisor{}
}

func (defaultLocalProcessSupervisor) start(cmd *exec.Cmd) error {
	cmd.SysProcAttr = util.Pdeathsig(&syscall.SysProcAttr{}, syscall.SIGTERM)
	return cmd.Start()
}

func (defaultLocalProcessSupervisor) wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (defaultLocalProcessSupervisor) close() {}
