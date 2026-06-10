// Copyright (c) 2020 10X Genomics, Inc. All rights reserved.

//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

//  Utility method to parse /proc/meminfo

package core

import "errors"

type MemInfo struct {
	Total      int64
	Used       int64
	Free       int64
	ActualFree int64
	ActualUsed int64
}

func (m *MemInfo) Get() error {
	return errors.New("not supported")
}
