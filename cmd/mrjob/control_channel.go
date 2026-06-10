package main

import (
	"os/exec"
)

type controlChannel interface {
	configure(cmd *exec.Cmd)
	readError(maxBytes int) []byte
	readErrorAfterWait(maxBytes int) []byte
	closeChild()
	closeParent()
}
