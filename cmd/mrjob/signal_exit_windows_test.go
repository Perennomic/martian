//go:build windows
// +build windows

package main

import (
	"errors"
	"testing"
)

func TestWindowsSigToErrPreservesNativeError(t *testing.T) {
	if sigToErr(nil) != nil {
		t.Fatal("nil error should remain nil")
	}
	err := errors.New("exit status 1")
	if sigToErr(err) != err {
		t.Fatal("Windows exit errors should not be converted to Unix signal errors")
	}
}
