package pbs

import (
	"testing"
	"time"
)

func TestParseQstatFullQueueWaitAggregatesQueuedJobs(t *testing.T) {
	client := NewClient("", 0, nil)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	data := client.ParseQstatFullQueueWait(`Job Id: 100.server
    job_state = Q
    queue = workq
    qtime = Mon Jun 29 11:55:00 2026

Job Id: 101.server
    job_state = Q
    queue = workq
    qtime = Mon Jun 29 11:00:00 2026

Job Id: 102.server
    job_state = Q
    queue = longq
    qtime = Sat Jun 27 12:00:00 2026

Job Id: 103.server
    job_state = R
    queue = workq
    qtime = Mon Jun 29 10:00:00 2026
`, now)

	workq := data.Queues["workq"]
	if workq.Count != 2 {
		t.Fatalf("workq count got %d want 2", workq.Count)
	}
	if workq.Sum != 3900 {
		t.Fatalf("workq sum got %v want 3900", workq.Sum)
	}
	if workq.Oldest != 3600 {
		t.Fatalf("workq oldest got %v want 3600", workq.Oldest)
	}
	assertBucket(t, workq, 300, 1)
	assertBucket(t, workq, 1800, 1)
	assertBucket(t, workq, 3600, 2)
	assertBucket(t, workq, queueWaitInfBucket, 2)

	longq := data.Queues["longq"]
	if longq.Count != 1 {
		t.Fatalf("longq count got %d want 1", longq.Count)
	}
	assertBucket(t, longq, 86400, 0)
	assertBucket(t, longq, 172800, 1)
}

func TestParseQstatFullQueueWaitSkipsMissingFieldsAndClampsNegativeWaits(t *testing.T) {
	client := NewClient("", 0, nil)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	data := client.ParseQstatFullQueueWait(`Job Id: 200.server
    job_state = Q
    queue = workq

Job Id: 201.server
    job_state = Q
    qtime = Mon Jun 29 11:55:00 2026

Job Id: 202.server
    job_state = Q
    queue = futureq
    qtime = Mon Jun 29 12:05:00 2026
`, now)

	if _, ok := data.Queues["workq"]; ok {
		t.Fatal("workq should be skipped when qtime is missing")
	}
	if _, ok := data.Queues[""]; ok {
		t.Fatal("empty queue should not be recorded")
	}

	futureq := data.Queues["futureq"]
	if futureq.Count != 1 {
		t.Fatalf("futureq count got %d want 1", futureq.Count)
	}
	if futureq.Sum != 0 || futureq.Oldest != 0 {
		t.Fatalf("futureq wait got sum=%v oldest=%v want zeroes", futureq.Sum, futureq.Oldest)
	}
	assertBucket(t, futureq, 300, 1)
}

func assertBucket(t *testing.T, info QueueWaitInfo, upperBound float64, expected int) {
	t.Helper()
	if got := info.Buckets[upperBound]; got != expected {
		t.Fatalf("bucket %v got %d want %d", upperBound, got, expected)
	}
}
