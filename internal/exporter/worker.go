package exporter

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"pbs-exporter/internal/pbs"
)

type SnapshotCollector interface {
	Collect(ctx context.Context) (*pbs.CollectionResult, error)
}

type Worker struct {
	collector SnapshotCollector
	store     *Store
	interval  time.Duration
	logger    *slog.Logger
}

var (
	errNilCollectionResult = errors.New("collector returned nil result")
	errNilSnapshot         = errors.New("collector returned nil snapshot")
)

func NewWorker(collector SnapshotCollector, store *Store, interval time.Duration, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return &Worker{
		collector: collector,
		store:     store,
		interval:  interval,
		logger:    logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.runOnceAndLog(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnceAndLog(ctx)
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	started := time.Now().UTC()
	result, err := w.collector.Collect(ctx)
	duration := time.Since(started)
	if err != nil {
		w.store.Clear(started, duration, err)
		return err
	}
	if result == nil {
		w.store.Clear(started, duration, errNilCollectionResult)
		return errNilCollectionResult
	}

	snapshot := result.Snapshot
	if snapshot == nil {
		w.store.Clear(started, duration, errNilSnapshot)
		return errNilSnapshot
	}
	if snapshot.CollectedAt.IsZero() {
		snapshot.CollectedAt = started
	}

	w.store.UpdateSuccess(result, started, duration)
	if result.JobInspectionError != nil {
		w.logger.Warn("job inspection collection failed", "err", result.JobInspectionError)
	}
	return nil
}

func (w *Worker) runOnceAndLog(ctx context.Context) {
	if err := w.RunOnce(ctx); err != nil {
		w.logger.Error("background PBS collection failed", "err", err)
		return
	}
	w.logger.Debug("background PBS collection succeeded")
}
