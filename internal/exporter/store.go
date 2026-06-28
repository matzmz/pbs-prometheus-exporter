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
	Up                         bool
	LastCollectTimestamp       time.Time
	LastCollectSuccessTimestamp time.Time
	SnapshotTimestamp          time.Time
	LastCollectDuration        time.Duration
	CollectErrorsTotal         uint64
	LastError                  string
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) UpdateSuccess(snapshot *pbs.Snapshot, collectedAt time.Time, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot = snapshot
	s.status.Up = true
	s.status.LastCollectTimestamp = collectedAt
	s.status.LastCollectSuccessTimestamp = collectedAt
	s.status.SnapshotTimestamp = snapshot.CollectedAt
	s.status.LastCollectDuration = duration
	s.status.LastError = ""
}

func (s *Store) Clear(collectedAt time.Time, duration time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot = nil
	s.status.Up = false
	s.status.LastCollectTimestamp = collectedAt
	s.status.SnapshotTimestamp = time.Time{}
	s.status.LastCollectDuration = duration
	if err != nil {
		s.status.CollectErrorsTotal++
		s.status.LastError = err.Error()
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
