//go:build darwin && cgo
// +build darwin,cgo

package core

import "testing"

func TestLoadAverageDarwin(t *testing.T) {
	var la LoadAverage
	if err := la.Get(); err != nil {
		t.Fatal(err)
	}
	if la.One < 0 {
		t.Errorf("one-minute loadavg should be non-negative, was %f", la.One)
	}
	if la.Five < 0 {
		t.Errorf("five-minute loadavg should be non-negative, was %f", la.Five)
	}
	if la.Fifteen < 0 {
		t.Errorf("fifteen-minute loadavg should be non-negative, was %f", la.Fifteen)
	}
	if la.One == 0 && la.Five == 0 && la.Fifteen == 0 {
		t.Error("expected at least one load average window to be non-zero")
	}
}
