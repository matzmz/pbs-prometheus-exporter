package exporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"pbs-exporter/internal/pbs"
)

type fakeCollector struct {
	snapshot *pbs.Snapshot
	err      error
}

func (f fakeCollector) Collect(_ context.Context) (*pbs.Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

func TestWorkerFailureClearsSnapshotAndSetsExporterDown(t *testing.T) {
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()
	store.UpdateSuccess(&pbs.Snapshot{CollectedAt: now}, now, time.Second)

	worker := NewWorker(fakeCollector{err: errors.New("pbs unavailable")}, store, time.Minute, nil)

	err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected RunOnce to return an error")
	}

	if store.Snapshot() != nil {
		t.Fatal("expected worker failure to clear the snapshot")
	}

	status := store.Status()
	if status.Up {
		t.Fatal("expected exporter to be down after worker failure")
	}
	if status.CollectErrorsTotal != 1 {
		t.Fatalf("expected collect errors to increment, got %d", status.CollectErrorsTotal)
	}
}
