//go:build darwin && !cgo
// +build darwin,!cgo

//  Utility method to read Darwin memory statistics.

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
	*m = MemInfo{}
	return errors.New("darwin meminfo requires cgo")
}
