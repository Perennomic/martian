// Copyright (c) 2020 10X Genomics, Inc. All rights reserved.

package core

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestMemViolationMerge(t *testing.T) {
	// MemReservationGB should track the most recent (max) reservation.
	// MaxRssBytes should track the highest observed RSS.
	prev := MemViolationContents{MemReservationGB: 2.0, MaxRssBytes: 2 * 1024 * 1024 * 1024}
	next := MemViolationContents{MemReservationGB: 4.0, MaxRssBytes: 4 * 1024 * 1024 * 1024}
	merged := prev.Merge(next)
	if merged.MemReservationGB != 4.0 {
		t.Errorf("expected MemReservationGB=4.0, got %v", merged.MemReservationGB)
	}
	if merged.MaxRssBytes != next.MaxRssBytes {
		t.Errorf("expected MaxRssBytes=%d, got %d", next.MaxRssBytes, merged.MaxRssBytes)
	}

	// Merging with zero prev should behave correctly (first violation).
	zero := MemViolationContents{}
	first := MemViolationContents{MemReservationGB: 2.0, MaxRssBytes: 1024}
	if m := zero.Merge(first); m.MemReservationGB != 2.0 {
		t.Errorf("expected MemReservationGB=2.0 from zero merge, got %v", m.MemReservationGB)
	}
}

// TestAutoAdjustMemoryFibonacci verifies that the getJobReqs scaling formula,
// combined with the corrected Merge, produces the Fibonacci-like sequence
// documented in the code comment: 1 → 2 → 3.25 → 4.81 → 6.77.
//
// Formula: newMem = maxRssGb + 0.75*origMem + 0.25*violation.MemReservationGB
// where origMem is the constant MRO-defined reservation (1.0 here).
func TestAutoAdjustMemoryFibonacci(t *testing.T) {
	const orig = 1.0
	const epsilon = 0.01

	expected := []float64{2.0, 3.25, 4.8125, 6.765625}

	// Simulate the fork's persisted violation report across retries.
	// The report is updated (via updateMemViolationReport) when a job fails,
	// before getJobReqs is called for the next retry.
	report := MemViolationContents{}

	allocation := orig
	for i, want := range expected {
		// Worst case: maxRss just barely exceeds the allocation.
		maxRss := allocation

		// Job fails: mrjob writes violation, fork merges into its report.
		report = report.Merge(MemViolationContents{
			MemReservationGB: allocation,
			MaxRssBytes:      int64(maxRss * 1024 * 1024 * 1024),
		})

		// getJobReqs computes next allocation using the updated report.
		newMem := maxRss + 0.75*orig + 0.25*report.MemReservationGB

		if math.Abs(newMem-want) > epsilon {
			t.Errorf("step %d: expected %.4f, got %.4f", i+1, want, newMem)
		}
		allocation = newMem
	}
}

func TestObservedMemoryWindowsFields(t *testing.T) {
	var observed ObservedMemory
	current := ObservedMemory{
		Rss:              10,
		Vmem:             20,
		WorkingSet:       10,
		PeakWorkingSet:   15,
		PrivateBytes:     20,
		PeakPrivateBytes: 25,
	}
	observed.IncreaseTo(current)
	observed.Add(ObservedMemory{
		Rss:              1,
		Vmem:             2,
		WorkingSet:       1,
		PeakWorkingSet:   2,
		PrivateBytes:     2,
		PeakPrivateBytes: 3,
	})
	if observed.WorkingSet != 11 || observed.PrivateBytes != 22 {
		t.Fatalf("expected Windows current counters to aggregate, got working_set=%d private_bytes=%d",
			observed.WorkingSet, observed.PrivateBytes)
	}
	if observed.PeakWorkingSet != 17 || observed.PeakPrivateBytes != 28 {
		t.Fatalf("expected Windows peak counters to aggregate, got peak_working_set=%d peak_private_bytes=%d",
			observed.PeakWorkingSet, observed.PeakPrivateBytes)
	}
	if observed.IsZero() {
		t.Fatal("expected observed memory with Windows counters to be non-zero")
	}
}

func TestObservedMemoryWindowsFieldsOmitEmpty(t *testing.T) {
	b, err := json.Marshal(ObservedMemory{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, field := range []string{
		"working_set",
		"peak_working_set",
		"private_bytes",
		"peak_private_bytes",
	} {
		if strings.Contains(got, field) {
			t.Fatalf("expected zero Windows field %q to be omitted from %s", field, got)
		}
	}
}
