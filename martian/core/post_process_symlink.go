//go:build !windows

package core

import "os"

func restoreMovedOutFile(relPath, filePath, outPath string) error {
	_ = outPath
	return os.Symlink(relPath, filePath)
}

func linkOutFile(target, outPath string) error {
	return os.Symlink(target, outPath)
}
