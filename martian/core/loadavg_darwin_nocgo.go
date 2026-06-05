//go:build darwin && !cgo
// +build darwin,!cgo

// Utility method to read Darwin load average.

package core

import "errors"

type LoadAverage struct {
	One, Five, Fifteen float64
}

func (*LoadAverage) Get() error {
	return errors.New("darwin loadavg requires cgo")
}
