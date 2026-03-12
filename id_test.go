package main

import (
	"testing"
	"time"
)

func TestGenerateUniqueIDBaseAvailable(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	got := generateUniqueID(nil, now)
	if got != "20260312-040000" {
		t.Fatalf("unexpected id: %q", got)
	}
}

func TestGenerateUniqueIDAllocatesSuffix(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	items := []handoffRecord{
		{ID: "20260312-040000"},
		{ID: "20260312-040000-002"},
	}
	got := generateUniqueID(items, now)
	if got != "20260312-040000-003" {
		t.Fatalf("unexpected id: %q", got)
	}
}

func TestGenerateUniqueIDReturnsEmptyWhenExhausted(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	items := make([]handoffRecord, 0, 999)
	items = append(items, handoffRecord{ID: "20260312-040000"})
	for i := 2; i <= 999; i++ {
		items = append(items, handoffRecord{ID: "20260312-040000-" + formatIDSuffix(i)})
	}

	got := generateUniqueID(items, now)
	if got != "" {
		t.Fatalf("expected empty id when exhausted, got %q", got)
	}
}
