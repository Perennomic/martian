package core

import "testing"

func TestNewResourceSampler(t *testing.T) {
	sampler := NewResourceSampler()
	if sampler == nil {
		t.Fatal("expected a default resource sampler")
	}
	if _, ok := sampler.(defaultResourceSampler); !ok {
		t.Fatalf("expected defaultResourceSampler, got %T", sampler)
	}
}
