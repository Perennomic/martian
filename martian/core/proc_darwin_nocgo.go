//go:build darwin && !cgo
// +build darwin,!cgo

package core

import "errors"

func readDarwinTaskInfo(pid int) (darwinTaskInfo, error) {
	return darwinTaskInfo{}, errors.New("darwin memory sampling requires cgo")
}
