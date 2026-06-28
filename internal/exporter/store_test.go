package exporter

import (
	"testing"
	"time"

	"pbs-exporter/internal/pbs"
)

func TestStoreClearRemovesSnapshot(t *testing.T) {
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.Snapshot{
		CollectedAt: now,
		Version:     "2026.1.0",
	}, now, 2*time.Second)

	if store.Snapshot() == nil {
		t.Fatal("expected snapshot to be stored")
	}

	store.Clear(now.Add(time.Second), 250*time.Millisecond, nil)

	if store.Snapshot() != nil {
		t.Fatal("expected snapshot to be cleared")
	}

	status := store.Status()
	if status.Up {
		t.Fatal("expected exporter to be marked down after clear")
	}
	if status.LastCollectTimestamp != now.Add(time.Second) {
		t.Fatalf("unexpected last collect timestamp: got %v", status.LastCollectTimestamp)
	}
}
