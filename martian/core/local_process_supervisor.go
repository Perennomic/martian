package core

import "os/exec"

type localProcessSupervisor interface {
	start(cmd *exec.Cmd) error
	wait(cmd *exec.Cmd) error
	close()
}
