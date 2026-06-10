//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

const (
	controlChannelEnv   = "MRO_CONTROL_CHANNEL=windows_file"
	controlLogPathEnv   = "MRO_CONTROL_LOG_PATH="
	controlErrorPathEnv = "MRO_CONTROL_ERROR_PATH="
)

type windowsControlChannel struct {
	logPath   string
	errorPath string
}

func newControlChannel(log *os.File) (controlChannel, error) {
	errorFile, err := os.CreateTemp(filepath.Dir(log.Name()), "_control_errors_")
	if err != nil {
		return nil, err
	}
	errorPath := errorFile.Name()
	if err := errorFile.Close(); err != nil {
		_ = os.Remove(errorPath)
		return nil, err
	}
	return &windowsControlChannel{
		logPath:   log.Name(),
		errorPath: errorPath,
	}, nil
}

func (self *windowsControlChannel) configure(cmd *exec.Cmd) {
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = append(env,
		controlChannelEnv,
		controlLogPathEnv+self.logPath,
		controlErrorPathEnv+self.errorPath)
}

func (self *windowsControlChannel) readError(int) []byte {
	return nil
}

func (self *windowsControlChannel) readErrorAfterWait(maxBytes int) []byte {
	if self.errorPath == "" {
		return nil
	}
	file, err := os.Open(self.errorPath)
	if err != nil {
		return nil
	}
	defer file.Close()
	return readBytes(maxBytes, file)
}

func (self *windowsControlChannel) closeChild() {}

func (self *windowsControlChannel) closeParent() {
	if self.errorPath != "" {
		_ = os.Remove(self.errorPath)
		self.errorPath = ""
	}
}
