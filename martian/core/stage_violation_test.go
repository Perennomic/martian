// Copyright (c) 2024 10X Genomics, Inc. All rights reserved.

package core

import (
	"os"
	"path/filepath"
	"testing"
)

// makeForkForViolationTest constructs a minimal Fork with real on-disk metadata
// directories, sufficient for testing violation report storage and updateState
// routing. The node field is intentionally nil — the functions under test do
// not access it (updateState is guarded with Running-state priming below to
// suppress endJob calls, which do require a full runtime).
func makeForkForViolationTest(t *testing.T) (*Fork, string) {
	t.Helper()
	tmp := t.TempDir()
	for _, sub := range []string{"fork0", "fork0/split", "fork0/join", "fork0/chnk0"} {
		if err := os.MkdirAll(filepath.Join(tmp, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	fork := &Fork{
		fqname:         "STAGE.fork0",
		metadata:       NewMetadata("STAGE.fork0", filepath.Join(tmp, "fork0")),
		split_metadata: NewMetadata("STAGE.fork0.split", filepath.Join(tmp, "fork0", "split")),
		join_metadata:  NewMetadata("STAGE.fork0.join", filepath.Join(tmp, "fork0", "join")),
	}
	return fork, tmp
}

// TestMemViolationSplitJoinStoredSeparately verifies that split and join
// violations are stored under independent keys and don't overwrite each other.
func TestMemViolationSplitJoinStoredSeparately(t *testing.T) {
	fork, _ := makeForkForViolationTest(t)

	splitV := MemViolationContents{MemReservationGB: 2.0, MaxRssBytes: 2 << 30}
	joinV := MemViolationContents{MemReservationGB: 4.0, MaxRssBytes: 5 << 30}

	fork.updateMemViolationReport(STAGE_TYPE_SPLIT, splitV)
	fork.updateMemViolationReport(STAGE_TYPE_JOIN, joinV)

	if got := fork.getMemViolation(STAGE_TYPE_SPLIT, 0); got == nil {
		t.Fatal("split violation missing from report")
	} else if got.MemReservationGB != 2.0 {
		t.Errorf("split MemReservationGB: want 2.0, got %v", got.MemReservationGB)
	}

	if got := fork.getMemViolation(STAGE_TYPE_JOIN, 0); got == nil {
		t.Fatal("join violation missing from report")
	} else if got.MemReservationGB != 4.0 {
		t.Errorf("join MemReservationGB: want 4.0, got %v", got.MemReservationGB)
	}
}

// TestUpdateStateSplitViolationRouting verifies that a split:mem_violation
// journal event causes the violation to be read from split_metadata and stored
// in the fork's report under the split key.
//
// split_metadata is primed as Running to suppress the endJob call.
func TestUpdateStateSplitViolationRouting(t *testing.T) {
	fork, _ := makeForkForViolationTest(t)

	splitV := MemViolationContents{MemReservationGB: 2.0, MaxRssBytes: 2 << 30}
	if err := fork.split_metadata.WriteAtomic(MemViolation, splitV); err != nil {
		t.Fatalf("writing split violation: %v", err)
	}

	// Prime split_metadata as Running so updateState won't call endJob.
	fork.split_metadata.contents[LogFile] = struct{}{}

	fork.updateState(string(SplitPrefix)+string(MemViolation), "")

	got := fork.getMemViolation(STAGE_TYPE_SPLIT, 0)
	if got == nil {
		t.Fatal("split violation missing from fork report after updateState")
	}
	if got.MemReservationGB != 2.0 {
		t.Errorf("want MemReservationGB=2.0, got %v", got.MemReservationGB)
	}
}

// TestUpdateStateJoinViolationRouting directly tests the updateState routing
// that was broken by a copy-paste bug: the join branch was reading from
// split_metadata instead of join_metadata, so join OOM violations were silently
// dropped and a spurious "no such file" error was logged.
//
// The test writes a violation only to join_metadata (no split violation exists),
// then fires the join:mem_violation journal event via updateState. With the bug,
// split_metadata.ReadInto fails and the violation is never recorded. With the fix,
// join_metadata.ReadInto succeeds and the fork report contains the violation.
//
// join_metadata is primed as Running to suppress the endJob call.
func TestUpdateStateJoinViolationRouting(t *testing.T) {
	fork, _ := makeForkForViolationTest(t)

	// Write a violation only to join_metadata; split_metadata has nothing.
	joinV := MemViolationContents{MemReservationGB: 4.0, MaxRssBytes: 5 << 30}
	if err := fork.join_metadata.WriteAtomic(MemViolation, joinV); err != nil {
		t.Fatalf("writing join violation: %v", err)
	}

	// Prime join_metadata as Running so updateState won't call endJob.
	fork.join_metadata.contents[LogFile] = struct{}{}

	fork.updateState(string(JoinPrefix)+string(MemViolation), "")

	got := fork.getMemViolation(STAGE_TYPE_JOIN, 0)
	if got == nil {
		t.Fatal("join violation missing from fork report after updateState; " +
			"updateState likely read from split_metadata instead of join_metadata")
	}
	if got.MemReservationGB != 4.0 {
		t.Errorf("want MemReservationGB=4.0, got %v", got.MemReservationGB)
	}
}

// TestUpdateStateChunkViolationRouting verifies that a chunk mem_violation
// journal event causes the violation to be stored in the fork's report under
// the chunk's integer index key.
//
// Chunk.updateState only calls endJob if beginState was Running or Queued.
// Fresh metadata has no contents so getState returns Waiting — no priming needed.
func TestUpdateStateChunkViolationRouting(t *testing.T) {
	fork, tmp := makeForkForViolationTest(t)

	const chunkIndex = 0
	chunkDir := filepath.Join(tmp, "fork0", "chnk0")
	chunk := &Chunk{
		fork:     fork,
		fqname:   "STAGE.fork0.chnk0",
		index:    chunkIndex,
		metadata: NewMetadata("STAGE.fork0.chnk0", chunkDir),
	}

	chunkV := MemViolationContents{MemReservationGB: 3.0, MaxRssBytes: 4 << 30}
	if err := chunk.metadata.WriteAtomic(MemViolation, chunkV); err != nil {
		t.Fatalf("writing chunk violation: %v", err)
	}

	// No priming needed: fresh metadata getState() returns Waiting,
	// so the beginState guard suppresses the endJob call.
	chunk.updateState(MemViolation, "")

	got := fork.getMemViolation(STAGE_TYPE_CHUNK, chunkIndex)
	if got == nil {
		t.Fatal("chunk violation missing from fork report after updateState")
	}
	if got.MemReservationGB != 3.0 {
		t.Errorf("want MemReservationGB=3.0, got %v", got.MemReservationGB)
	}
}
