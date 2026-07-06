package exporter

import (
	"testing"
	"time"

	"pbs-exporter/internal/pbs"
)

func TestStoreClearRemovesSnapshot(t *testing.T) {
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			Version:     "2026.1.0",
		},
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

func TestStoreKeepsJobSampleHistogramsAcrossInspectionErrorsAndClear(t *testing.T) {
	store := NewStore()
	store.ConfigureJobSampleHistograms(JobSampleHistogramConfig{
		Enabled:         true,
		CPUBuckets:      []float64{4, 8},
		MemoryBuckets:   []float64{1024, 2048},
		WalltimeBuckets: []float64{60, 120},
		MPIBuckets:      []float64{1, 2},
		NodeBuckets:     []float64{1, 2},
		GPUBuckets:      []float64{1, 2},
	})

	now := time.Unix(1700000000, 0).UTC()
	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			JobInspection: &pbs.JobInspectionData{
				Jobs: []pbs.InspectedJob{
					{
						JobState: "R",
						Requested: pbs.RequestedJobResources{
							CPUCores:    pbs.OptionalFloat64{Value: 8, Set: true},
							MemoryBytes: pbs.OptionalFloat64{Value: 2048, Set: true},
						},
					},
				},
			},
		},
		JobInspectionAttempted: true,
	}, now, time.Second)

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot:               &pbs.Snapshot{CollectedAt: now.Add(time.Minute)},
		JobInspectionAttempted: true,
		JobInspectionError:     errNilSnapshot,
	}, now.Add(time.Minute), time.Second)

	beforeClear := store.JobSampleHistograms()
	if beforeClear == nil {
		t.Fatal("expected histogram snapshot to be available")
	}
	if beforeClear.CPU.Count != 1 || beforeClear.CPU.Sum != 8 {
		t.Fatalf("unexpected cpu histogram before clear: %+v", beforeClear.CPU)
	}

	store.Clear(now.Add(2*time.Minute), time.Second, nil)

	afterClear := store.JobSampleHistograms()
	if afterClear == nil {
		t.Fatal("expected histogram snapshot to survive clear")
	}
	if afterClear.CPU.Count != 1 || afterClear.CPU.Sum != 8 {
		t.Fatalf("unexpected cpu histogram after clear: %+v", afterClear.CPU)
	}
}
