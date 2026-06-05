//go:build darwin && cgo
// +build darwin,cgo

package core

import "testing"

func TestMemInfoDarwin(t *testing.T) {
	var m MemInfo
	if err := m.Get(); err != nil {
		t.Fatal(err)
	}
	if m.Total <= 0 {
		t.Fatalf("expected nonzero total memory, got %d", m.Total)
	}
	if m.Free < 0 {
		t.Fatalf("expected non-negative free memory, got %d", m.Free)
	}
	if m.ActualFree <= 0 {
		t.Fatalf("expected nonzero available memory, got %d", m.ActualFree)
	}
	if m.ActualFree > m.Total {
		t.Fatalf("expected available memory <= total memory, got free=%d total=%d",
			m.ActualFree, m.Total)
	}
	if m.Free > m.ActualFree {
		t.Fatalf("expected free memory <= available memory, got free=%d available=%d",
			m.Free, m.ActualFree)
	}
	if m.Used != m.Total-m.Free {
		t.Fatalf("expected Used=%d, got %d", m.Total-m.Free, m.Used)
	}
	if m.ActualUsed != m.Total-m.ActualFree {
		t.Fatalf("expected ActualUsed=%d, got %d", m.Total-m.ActualFree, m.ActualUsed)
	}
}
