//go:build !darwin
// +build !darwin

package main

import (
	"os"
	"syscall"

	"github.com/martian-lang/martian/martian/util"
)

func jobSysProcAttr(sig syscall.Signal) *syscall.SysProcAttr {
	return util.Pdeathsig(&syscall.SysProcAttr{}, sig)
}

func killProcess(proc *os.Process, sig syscall.Signal) {
	if proc != nil {
		_ = proc.Signal(sig)
	}
}
