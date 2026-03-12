package handoffid

import (
	"testing"
	"time"
)

func TestGenerateBaseIDWhenAvailable(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	got := Generate(nil, now)
	if got != "20260312-040000" {
		t.Fatalf("unexpected id: %q", got)
	}
}

func TestGenerateAddsSuffixOnCollision(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	existing := []string{"20260312-040000", "20260312-040000-002"}

	got := Generate(existing, now)
	if got != "20260312-040000-003" {
		t.Fatalf("unexpected id: %q", got)
	}
}

func TestGenerateReturnsEmptyAfterExhaustion(t *testing.T) {
	now := time.Date(2026, 3, 12, 4, 0, 0, 0, time.UTC)
	existing := make([]string, 0, 999)
	existing = append(existing, "20260312-040000")
	for i := 2; i <= 999; i++ {
		existing = append(existing, "20260312-040000-"+formatSuffix(i))
	}

	got := Generate(existing, now)
	if got != "" {
		t.Fatalf("expected empty id when exhausted, got %q", got)
	}
}
