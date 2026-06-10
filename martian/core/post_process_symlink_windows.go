//go:build windows

package core

import (
	"errors"
	"os"

	"github.com/martian-lang/martian/martian/util"
	"golang.org/x/sys/windows"
)

func restoreMovedOutFile(relPath, filePath, outPath string) error {
	if err := os.Symlink(relPath, filePath); err == nil {
		return nil
	} else if !isWindowsSymlinkUnavailable(err) {
		return err
	}

	util.LogInfo("filesys",
		"Windows symlink unavailable for %s -> %s; keeping canonical output at %s.",
		filePath, relPath, outPath)
	return nil
}

func linkOutFile(target, outPath string) error {
	if err := os.Symlink(target, outPath); err == nil {
		return nil
	} else if !isWindowsSymlinkUnavailable(err) {
		return err
	}

	util.LogInfo("filesys",
		"Windows symlink unavailable for outs link %s -> %s; output remains at its recorded path.",
		outPath, target)
	return nil
}

func isWindowsSymlinkUnavailable(err error) bool {
	return errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD)
}
