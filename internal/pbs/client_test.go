package pbs

import (
	"testing"
	"time"
)

func TestParseQstatOutputMapsExitingStatus(t *testing.T) {
	client := NewClient("", 0, ClientOptions{}, nil)

	data := client.ParseQstatOutput(`Job id            Name             User              Time Use S Queue
----------------  ---------------- ----------------  -------- - -----
123.server        job              alice             00:00:00 E workq
`)

	if got := data.StatusCount["exiting"]; got != 1 {
		t.Fatalf("exiting count got %d want 1", got)
	}
	if got := data.StatusCount["error"]; got != 0 {
		t.Fatalf("error count got %d want 0", got)
	}
}

func TestParseQstatFullQueueWaitAggregatesQueuedJobs(t *testing.T) {
	client := NewClient("", 0, ClientOptions{}, nil)
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
	client := NewClient("", 0, ClientOptions{}, nil)
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

func TestParseJobInspectionOutputParsesTypedMetrics(t *testing.T) {
	client := NewClient("", 0, ClientOptions{IncludeJobInspection: true}, nil)
	collectedAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)

	data, err := client.ParseJobInspectionOutput(`{
  "Jobs": {
    "100.server": {
      "queue": "workq",
      "project": "astro",
      "Job_Owner": "alice@submit01",
      "job_state": "R",
      "stime": "Thu Jul  2 09:00:00 2026",
      "Resource_List": {
        "mem": "4gb",
        "mpiprocs": 1,
        "ncpus": 8,
        "ngpus": 1,
        "nodect": 1,
        "walltime": "47:00:00"
      },
      "resources_used": {
        "cpupercent": 210,
        "cput": "41:05:35",
        "mem": "14713128kb",
        "ncpus": 8,
        "ngpus": 0,
        "vmem": "213273680kb",
        "walltime": "17:43:19"
      }
    },
    "101.server": {
      "queue": "gpuq",
      "Job_Owner": "bob@submit02",
      "job_state": "Q",
      "qtime": "Thu Jul  2 08:30:00 2026",
      "Resource_List": {
        "mem": "broken",
        "mpiprocs": "oops",
        "ncpus": 4,
        "ngpus": 0,
        "nodect": 1,
        "walltime": {"invalid": true}
      },
      "resources_used": {
        "cpupercent": 12
      }
    }
  }
}`, collectedAt)
	if err != nil {
		t.Fatalf("ParseJobInspectionOutput returned error: %v", err)
	}

	if len(data.Jobs) != 2 {
		t.Fatalf("job count got %d want 2", len(data.Jobs))
	}

	runningJob := data.Jobs[0]
	if runningJob.JobID != "100.server" {
		t.Fatalf("unexpected running job id: %q", runningJob.JobID)
	}
	if runningJob.Project != "astro" {
		t.Fatalf("unexpected project: %q", runningJob.Project)
	}
	if runningJob.JobOwner != "alice@submit01" {
		t.Fatalf("unexpected job owner: %q", runningJob.JobOwner)
	}
	if !runningJob.Requested.MemoryBytes.Set || runningJob.Requested.MemoryBytes.Value != 4*1024*1024*1024 {
		t.Fatalf("unexpected requested memory: %+v", runningJob.Requested.MemoryBytes)
	}
	if !runningJob.Used.MemoryBytes.Set || runningJob.Used.MemoryBytes.Value != 14713128*1024 {
		t.Fatalf("unexpected used memory: %+v", runningJob.Used.MemoryBytes)
	}
	if !runningJob.Used.CPUTimeSeconds.Set || runningJob.Used.CPUTimeSeconds.Value != 147935 {
		t.Fatalf("unexpected CPU time: %+v", runningJob.Used.CPUTimeSeconds)
	}
	if !runningJob.RuntimeSeconds.Set || runningJob.RuntimeSeconds.Value != 3600 {
		t.Fatalf("unexpected runtime: %+v", runningJob.RuntimeSeconds)
	}

	queuedJob := data.Jobs[1]
	if queuedJob.Project != "" {
		t.Fatalf("expected empty project, got %q", queuedJob.Project)
	}
	if queuedJob.Requested.MemoryBytes.Set {
		t.Fatalf("expected malformed requested memory to be skipped, got %+v", queuedJob.Requested.MemoryBytes)
	}
	if queuedJob.Requested.MPIProcesses.Set {
		t.Fatalf("expected malformed mpiprocs to be skipped, got %+v", queuedJob.Requested.MPIProcesses)
	}
	if queuedJob.Requested.WalltimeSeconds.Set {
		t.Fatalf("expected malformed walltime to be skipped, got %+v", queuedJob.Requested.WalltimeSeconds)
	}
	if queuedJob.Used.CPUPercent.Set {
		t.Fatalf("expected non-running job to skip used resources, got %+v", queuedJob.Used.CPUPercent)
	}
	if !queuedJob.QueueWaitSeconds.Set || queuedJob.QueueWaitSeconds.Value != 5400 {
		t.Fatalf("unexpected queue wait: %+v", queuedJob.QueueWaitSeconds)
	}
}
