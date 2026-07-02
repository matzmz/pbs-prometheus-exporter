package exporter

import (
	"sync"
	"time"

	"pbs-exporter/internal/pbs"
)

type Store struct {
	mu       sync.RWMutex
	snapshot *pbs.Snapshot
	status   Status
}

type Status struct {
	Up                          bool
	LastCollectTimestamp        time.Time
	LastCollectSuccessTimestamp time.Time
	SnapshotTimestamp           time.Time
	LastCollectDuration         time.Duration
	CollectErrorsTotal          uint64
	LastError                   string
	JobInspectionUp             bool
	JobInspectionErrorsTotal    uint64
	JobInspectionLastSuccessAt  time.Time
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) UpdateSuccess(result *pbs.CollectionResult, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := result.Snapshot
	s.snapshot = snapshot
	s.status.Up = true
	s.status.LastCollectTimestamp = collectedAt
	s.status.LastCollectSuccessTimestamp = collectedAt
	s.status.SnapshotTimestamp = snapshot.CollectedAt
	s.status.LastCollectDuration = duration
	s.status.LastError = ""
	s.applyJobInspectionResult(result, collectedAt)
}

func (s *Store) Clear(collectedAt time.Time, duration time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot = nil
	s.status.Up = false
	s.status.LastCollectTimestamp = collectedAt
	s.status.SnapshotTimestamp = time.Time{}
	s.status.LastCollectDuration = duration
	s.status.JobInspectionUp = false
	if err != nil {
		s.status.CollectErrorsTotal++
		s.status.LastError = err.Error()
	}
}

func (s *Store) applyJobInspectionResult(result *pbs.CollectionResult, collectedAt time.Time) {
	if !result.JobInspectionAttempted {
		s.status.JobInspectionUp = false
		return
	}

	if result.JobInspectionError != nil {
		s.status.JobInspectionUp = false
		s.status.JobInspectionErrorsTotal++
		return
	}

	s.status.JobInspectionUp = result.Snapshot != nil && result.Snapshot.JobInspection != nil
	if s.status.JobInspectionUp {
		s.status.JobInspectionLastSuccessAt = collectedAt
	}
}

func (s *Store) Snapshot() *pbs.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}
