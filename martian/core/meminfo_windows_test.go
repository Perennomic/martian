//go:build windows
// +build windows

package core

import "testing"

func TestMemInfoWindows(t *testing.T) {
	var m MemInfo
	if err := m.Get(); err != nil {
		t.Fatal(err)
	}
	if m.Total <= 0 {
		t.Fatalf("expected nonzero total physical memory, got %d", m.Total)
	}
	if m.Free < 0 {
		t.Fatalf("expected non-negative available physical memory, got %d", m.Free)
	}
	if m.ActualFree != m.Free {
		t.Fatalf("expected ActualFree to use Windows available physical memory, got actual=%d free=%d",
			m.ActualFree, m.Free)
	}
	if m.Free > m.Total {
		t.Fatalf("expected available physical memory <= total physical memory, got free=%d total=%d",
			m.Free, m.Total)
	}
	if m.Used != m.Total-m.Free {
		t.Fatalf("expected Used=%d, got %d", m.Total-m.Free, m.Used)
	}
	if m.ActualUsed != m.Total-m.ActualFree {
		t.Fatalf("expected ActualUsed=%d, got %d", m.Total-m.ActualFree, m.ActualUsed)
	}
}
