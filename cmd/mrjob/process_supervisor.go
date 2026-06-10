package main

import (
	"os/exec"
	"syscall"

	"github.com/martian-lang/martian/martian/core"
)

type processSupervisor interface {
	configure(cmd *exec.Cmd, parentDeathSignal syscall.Signal)
	start(cmd *exec.Cmd) error
	startAuxiliary(cmd *exec.Cmd, parentDeathSignal syscall.Signal) error
	wait() error
	kill(sig syscall.Signal, reason string)
	terminationReason() string
	rusage() *core.RusageInfo
	waitChildren() bool
	reportChildren() bool
	close()
}
