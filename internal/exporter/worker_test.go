package exporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"pbs-exporter/internal/pbs"
)

type fakeCollector struct {
	snapshot            *pbs.Snapshot
	err                 error
	jobInspectionErr    error
	jobInspectionActive bool
}

func (f fakeCollector) Collect(_ context.Context) (*pbs.CollectionResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &pbs.CollectionResult{
		Snapshot:               f.snapshot,
		JobInspectionAttempted: f.jobInspectionActive,
		JobInspectionError:     f.jobInspectionErr,
	}, nil
}

func TestWorkerFailureClearsSnapshotAndSetsExporterDown(t *testing.T) {
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()
	store.UpdateSuccess(&pbs.CollectionResult{Snapshot: &pbs.Snapshot{CollectedAt: now}}, now, time.Second)

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

func TestWorkerKeepsSnapshotWhenJobInspectionFails(t *testing.T) {
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	worker := NewWorker(fakeCollector{
		snapshot:            &pbs.Snapshot{CollectedAt: now},
		jobInspectionActive: true,
		jobInspectionErr:    errors.New("qstat json failed"),
	}, store, time.Minute, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if store.Snapshot() == nil {
		t.Fatal("expected snapshot to be stored")
	}

	status := store.Status()
	if !status.Up {
		t.Fatal("expected exporter to stay up")
	}
	if status.JobInspectionUp {
		t.Fatal("expected job inspection to be marked down")
	}
	if status.JobInspectionErrorsTotal != 1 {
		t.Fatalf("expected job inspection errors to increment, got %d", status.JobInspectionErrorsTotal)
	}
}
