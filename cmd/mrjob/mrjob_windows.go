// Copyright (c) 2022 10X Genomics, Inc. All rights reserved.

package main

import (
	"os"

	"github.com/martian-lang/martian/martian/util"
)

// Force the given file to sync. Windows does not provide Unix-style directory
// fsync through os.File.Sync, so directory sync requests are intentionally
// skipped.
func syncFile(filename string) {
	info, err := os.Stat(filename)
	if err != nil || info.IsDir() {
		return
	}
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		util.LogError(err, "mrjob",
			"Error syncing file descriptor for %s", filename)
	}
}

// reportChildren generic stub which always returns false.
func reportChildren() bool {
	return false
}

// waitChildren eneric stub which always returns false.
func waitChildren() bool {
	return false
}
