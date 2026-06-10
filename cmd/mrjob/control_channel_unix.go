//go:build !windows
// +build !windows

package main

import (
	"os"
	"os/exec"
)

type fdControlChannel struct {
	log         *os.File
	errorRead   *os.File
	errorWriter *os.File
}

func newControlChannel(log *os.File) (controlChannel, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &fdControlChannel{
		log:         log,
		errorRead:   reader,
		errorWriter: writer,
	}, nil
}

func (self *fdControlChannel) configure(cmd *exec.Cmd) {
	cmd.ExtraFiles = []*os.File{self.log, self.errorWriter}
}

func (self *fdControlChannel) readError(maxBytes int) []byte {
	return readBytes(maxBytes, self.errorRead)
}

func (self *fdControlChannel) readErrorAfterWait(int) []byte {
	return nil
}

func (self *fdControlChannel) closeChild() {
	if self.errorWriter != nil {
		_ = self.errorWriter.Close()
		self.errorWriter = nil
	}
}

func (self *fdControlChannel) closeParent() {
	if self.errorRead != nil {
		_ = self.errorRead.Close()
		self.errorRead = nil
	}
}
