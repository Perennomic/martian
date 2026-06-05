//go:build darwin && cgo
// +build darwin,cgo

// Utility method to read Darwin load average.

package core

/*
#include <stdlib.h>
*/
import "C"

import "errors"

type LoadAverage struct {
	One, Five, Fifteen float64
}

func (la *LoadAverage) Get() error {
	var loads [3]C.double
	if C.getloadavg(&loads[0], 3) != 3 {
		return errors.New("getloadavg failed")
	}
	la.One = float64(loads[0])
	la.Five = float64(loads[1])
	la.Fifteen = float64(loads[2])
	return nil
}
