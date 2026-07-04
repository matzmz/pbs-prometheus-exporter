package pbs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseJobsJSONMapsStatusesAndUsers(t *testing.T) {
	data := parseJobsJSON(&jobsJSONPayload{
		Jobs: map[string]jobsJSONRecord{
			"100.server": {
				Queue:         "workq",
				JobState:      "E",
				EffectiveUser: "alice",
			},
			"101.server": {
				Queue:         "workq",
				JobState:      "R",
				EffectiveUser: "alice",
			},
			"102.server": {
				Queue:    "gpuq",
				JobState: "R",
				JobOwner: "bob@submit02",
			},
			"103.server": {
				Queue:         "workq",
				JobState:      "Q",
				EffectiveUser: "alice",
			},
		},
	})

	if got := data.StatusCount["exiting"]; got != 1 {
		t.Fatalf("exiting count got %d want 1", got)
	}
	if got := data.StatusCount["running"]; got != 2 {
		t.Fatalf("running count got %d want 2", got)
	}
	if got := data.StatusCount["queued"]; got != 1 {
		t.Fatalf("queued count got %d want 1", got)
	}
	if got := data.UserJobCount["alice"]; got != 1 {
		t.Fatalf("alice running jobs got %d want 1", got)
	}
	if got := data.UserJobCount["bob"]; got != 1 {
		t.Fatalf("bob running jobs got %d want 1", got)
	}
	if got := data.QueuedJobsByUser["alice"]; got != 1 {
		t.Fatalf("alice queued jobs got %d want 1", got)
	}
	if got := data.QueueJobCount["workq"]; got != 1 {
		t.Fatalf("workq running jobs got %d want 1", got)
	}
	if got := data.QueueJobCount["gpuq"]; got != 1 {
		t.Fatalf("gpuq running jobs got %d want 1", got)
	}
	if got := data.QueueTotalCount["workq"]; got != 3 {
		t.Fatalf("workq total jobs got %d want 3", got)
	}
}

func TestParseQueueWaitJSONAggregatesQueuedJobs(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	data := parseQueueWaitJSON(&jobsJSONPayload{
		Jobs: map[string]jobsJSONRecord{
			"100.server": {
				Queue:    "workq",
				JobState: "Q",
				Qtime:    "Mon Jun 29 11:55:00 2026",
			},
			"101.server": {
				Queue:    "workq",
				JobState: "Q",
				Qtime:    "Mon Jun 29 11:00:00 2026",
			},
			"102.server": {
				Queue:    "longq",
				JobState: "Q",
				Qtime:    "Sat Jun 27 12:00:00 2026",
			},
			"103.server": {
				Queue:    "workq",
				JobState: "R",
				Qtime:    "Mon Jun 29 10:00:00 2026",
			},
		},
	}, now)

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

func TestParseQueueWaitJSONSkipsMissingFieldsAndClampsNegativeWaits(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	data := parseQueueWaitJSON(&jobsJSONPayload{
		Jobs: map[string]jobsJSONRecord{
			"200.server": {
				Queue:    "workq",
				JobState: "Q",
			},
			"201.server": {
				JobState: "Q",
				Qtime:    "Mon Jun 29 11:55:00 2026",
			},
			"202.server": {
				Queue:    "futureq",
				JobState: "Q",
				Qtime:    "Mon Jun 29 12:05:00 2026",
			},
		},
	}, now)

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

func TestParseQueuesJSONBuildsQueueInfoAndSummary(t *testing.T) {
	queueData := parseQueuesJSON(&queuesJSONPayload{
		Queue: map[string]queueJSONRecord{
			"workq": {
				StateCount:  "Transit:0 Queued:2 Held:0 Waiting:0 Running:4 Exiting:0",
				Enabled:     jsonScalar{text: "True", set: true},
				Started:     jsonScalar{text: "True", set: true},
				MaxWalltime: jsonScalar{text: "47:00:00", set: true},
			},
			"gpuq": {
				StateCount:      "Transit:0 Queued:1 Held:0 Waiting:0 Running:0 Exiting:0",
				Enabled:         jsonScalar{text: "False", set: true},
				Started:         jsonScalar{text: "True", set: true},
				DefaultWalltime: jsonScalar{text: "12:00:00", set: true},
			},
		},
	})

	workq := queueData.Queues["workq"]
	if workq.Running != 4 || workq.Queued != 2 {
		t.Fatalf("unexpected workq counts: %+v", workq)
	}
	if !workq.Enabled || !workq.Started {
		t.Fatalf("unexpected workq booleans: %+v", workq)
	}
	if workq.Walltime != 169200 {
		t.Fatalf("unexpected workq walltime: %d", workq.Walltime)
	}

	gpuq := queueData.Queues["gpuq"]
	if gpuq.Enabled {
		t.Fatalf("expected gpuq disabled: %+v", gpuq)
	}
	if gpuq.Walltime != 43200 {
		t.Fatalf("unexpected gpuq walltime: %d", gpuq.Walltime)
	}

	summary := summarizeQueues(queueData)
	if summary.Running != 4 || summary.Queued != 3 {
		t.Fatalf("unexpected queue summary: %+v", summary)
	}
}

func TestParseServerJSONBuildsServerData(t *testing.T) {
	server := parseServerJSON(&serverJSONPayload{
		Server: map[string]serverJSONRecord{
			"pbs": {
				ServerState:        "Active",
				Scheduling:         jsonScalar{text: "True", set: true},
				TotalJobs:          jsonScalar{text: "6", set: true},
				StateCount:         "Transit:0 Queued:2 Held:1 Waiting:0 Running:4 Exiting:3",
				AssignedCPUs:       jsonScalar{text: "16", set: true},
				AssignedMemory:     jsonScalar{text: "128gb", set: true},
				AssignedNodes:      jsonScalar{text: "2", set: true},
				LicenseCount:       "Avail_Global:4 Used:2",
				MaxArraySize:       jsonScalar{text: "1000", set: true},
				JobHistoryEnabled:  jsonScalar{text: "True", set: true},
				JobHistoryDuration: jsonScalar{text: "24:00:00", set: true},
			},
		},
	})

	if server.State != "Active" || !server.Scheduling {
		t.Fatalf("unexpected server state: %+v", server)
	}
	if server.TotalJobs != 6 || server.JobsRunning != 4 || server.JobsQueued != 2 {
		t.Fatalf("unexpected server counts: %+v", server)
	}
	if server.JobsHeld != 1 || server.JobsExiting != 3 {
		t.Fatalf("unexpected server state counts: %+v", server)
	}
	if server.ResourcesNcpus != 16 || server.ResourcesNodect != 2 {
		t.Fatalf("unexpected assigned resources: %+v", server)
	}
	if server.ResourcesMemBytes != 128*1024*1024*1024 {
		t.Fatalf("unexpected assigned memory: %v", server.ResourcesMemBytes)
	}
	if server.LicensesAvailable != 4 || server.LicensesUsed != 2 {
		t.Fatalf("unexpected licenses: %+v", server)
	}
	if !server.JobHistoryEnabled || server.JobHistoryDuration != 86400 {
		t.Fatalf("unexpected job history: %+v", server)
	}
}

func TestParseNodesJSONBuildsNodeData(t *testing.T) {
	nodes := parseNodesJSON(&nodesJSONPayload{
		Nodes: map[string]nodeJSONRecord{
			"node01": {
				State:  "free",
				Jobs:   jsonScalar{text: "2", set: true},
				Memory: jsonScalar{text: "64gb/128gb", set: true},
				CPUs:   jsonScalar{text: "12/16", set: true},
				GPUs:   jsonScalar{text: "1/2", set: true},
			},
			"node02": {
				State:    "<various>",
				JobCount: jsonScalar{text: "1", set: true},
				Memory:   jsonScalar{text: "32gb/64gb", set: true},
				CPUs:     jsonScalar{text: "0/8", set: true},
				GPUs:     jsonScalar{text: "0/0", set: true},
			},
			"node03": {
				State: "state-unknown",
			},
		},
	})

	if nodes.CountFree != 1 || nodes.CountBusy != 1 || nodes.CountOffline != 0 || nodes.CountDown != 0 {
		t.Fatalf("unexpected node counts: %+v", nodes)
	}

	node01 := nodes.Nodes["node01"]
	if node01.Jobs != 2 || node01.CPUsAvailable != 12 || node01.CPUsTotal != 16 {
		t.Fatalf("unexpected node01 cpu data: %+v", node01)
	}
	if node01.GPUsAvailable != 1 || node01.GPUsTotal != 2 {
		t.Fatalf("unexpected node01 gpu data: %+v", node01)
	}
	if node01.MemoryAvailable != 64*1024*1024*1024 || node01.MemoryTotal != 128*1024*1024*1024 {
		t.Fatalf("unexpected node01 memory data: %+v", node01)
	}

	node02 := nodes.Nodes["node02"]
	if node02.State != "job-busy" || node02.Jobs != 1 {
		t.Fatalf("unexpected node02 state: %+v", node02)
	}
}

func TestParseNodesJSONHandlesPBSNodesSummaryJSON(t *testing.T) {
	var payload nodesJSONPayload
	if err := json.Unmarshal([]byte(`{
  "nodes": {
    "gnode02": {
      "State": "free",
      "Total Jobs": 2,
      "Running Jobs": 2,
      "Suspended Jobs": "",
      "mem f/t": "153gb/187gb",
      "ncpus f/t": "4/24",
      "nmics f/t": "0/0",
      "ngpus f/t": "2/2",
      "jobs": ["812001.cluster01", "812002.cluster01"]
    },
    "anode02": {
      "State": "<various>",
      "Total Jobs": 6,
      "Running Jobs": 6,
      "Suspended Jobs": "",
      "mem f/t": "2gb/502gb",
      "ncpus f/t": "16/64",
      "nmics f/t": "0/0",
      "ngpus f/t": "0/4",
      "jobs": ["812003.cluster01"]
    },
    "cnode03": {
      "State": "<various>",
      "Total Jobs": "\t",
      "Running Jobs": "\t",
      "mem f/t": "496gb/503gb",
      "ncpus f/t": "0/24",
      "ngpus f/t": "0/0",
      "jobs": ["812004.cluster01", "812005.cluster01"]
    }
  }
}`), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	nodes := parseNodesJSON(&payload)

	gnode := nodes.Nodes["gnode02"]
	if gnode.State != "free" || gnode.Jobs != 2 {
		t.Fatalf("unexpected gnode02 state/jobs: %+v", gnode)
	}
	if gnode.CPUsAvailable != 4 || gnode.CPUsTotal != 24 {
		t.Fatalf("unexpected gnode02 cpu data: %+v", gnode)
	}
	if gnode.GPUsAvailable != 2 || gnode.GPUsTotal != 2 {
		t.Fatalf("unexpected gnode02 gpu data: %+v", gnode)
	}
	if gnode.MemoryAvailable != 153*1024*1024*1024 || gnode.MemoryTotal != 187*1024*1024*1024 {
		t.Fatalf("unexpected gnode02 memory data: %+v", gnode)
	}

	anode := nodes.Nodes["anode02"]
	if anode.State != "job-busy" || anode.Jobs != 6 {
		t.Fatalf("unexpected anode02 state/jobs: %+v", anode)
	}
	if anode.CPUsAvailable != 16 || anode.CPUsTotal != 64 {
		t.Fatalf("unexpected anode02 cpu data: %+v", anode)
	}
	if anode.GPUsAvailable != 0 || anode.GPUsTotal != 4 {
		t.Fatalf("unexpected anode02 gpu data: %+v", anode)
	}
	if anode.MemoryAvailable != 2*1024*1024*1024 || anode.MemoryTotal != 502*1024*1024*1024 {
		t.Fatalf("unexpected anode02 memory data: %+v", anode)
	}

	cnode := nodes.Nodes["cnode03"]
	if cnode.Jobs != 2 {
		t.Fatalf("unexpected cnode03 fallback job count: %+v", cnode)
	}
}

func assertBucket(t *testing.T, info QueueWaitInfo, upperBound float64, expected int) {
	t.Helper()
	if got := info.Buckets[upperBound]; got != expected {
		t.Fatalf("bucket %v got %d want %d", upperBound, got, expected)
	}
}

func TestParseJobInspectionOutputParsesTypedMetrics(t *testing.T) {
	collectedAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)

	data, err := parseJobInspectionJSON(`{
  "Jobs": {
    "100.server01": {
      "queue": "workq",
      "project": "astro",
      "Job_Owner": "alice@lab@submit01",
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
        "walltime": "invalid"
      },
      "resources_used": {
        "cpupercent": 12
      }
    }
  }
}`, collectedAt)
	if err != nil {
		t.Fatalf("parseJobInspectionJSON returned error: %v", err)
	}

	if len(data.Jobs) != 2 {
		t.Fatalf("job count got %d want 2", len(data.Jobs))
	}

	runningJob := data.Jobs[0]
	if runningJob.JobID != "100" {
		t.Fatalf("unexpected running job id: %q", runningJob.JobID)
	}
	if runningJob.Project != "astro" {
		t.Fatalf("unexpected project: %q", runningJob.Project)
	}
	if runningJob.JobOwner != "alice@lab" {
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
	if queuedJob.JobID != "101" {
		t.Fatalf("unexpected queued job id: %q", queuedJob.JobID)
	}
	if queuedJob.JobOwner != "bob" {
		t.Fatalf("unexpected queued job owner: %q", queuedJob.JobOwner)
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

func TestParseJobInspectionOutputRejectsNonScalarFields(t *testing.T) {
	collectedAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)

	_, err := parseJobInspectionJSON(`{
  "Jobs": {
    "101.server": {
      "queue": "gpuq",
      "Job_Owner": "bob@submit02",
      "job_state": "Q",
      "qtime": "Thu Jul  2 08:30:00 2026",
      "Resource_List": {
        "walltime": {"invalid": true}
      }
    }
  }
}`, collectedAt)
	if err == nil {
		t.Fatal("expected parseJobInspectionJSON to reject object-valued scalar fields")
	}
	if !strings.Contains(err.Error(), "expected JSON scalar") {
		t.Fatalf("parseJobInspectionJSON error = %q, want scalar parsing context", err.Error())
	}
}

func TestNormalizeJobInspectionLabels(t *testing.T) {
	tests := []struct {
		name      string
		jobID     string
		jobOwner  string
		wantID    string
		wantOwner string
	}{
		{
			name:      "pbs server suffix",
			jobID:     "12345.server01",
			jobOwner:  "foo@hostname",
			wantID:    "12345",
			wantOwner: "foo",
		},
		{
			name:      "owner containing at sign",
			jobID:     "  9876.server  ",
			jobOwner:  "foo@bar@hostname",
			wantID:    "9876",
			wantOwner: "foo@bar",
		},
		{
			name:      "fallbacks preserve unparseable values",
			jobID:     "interactive",
			jobOwner:  "service",
			wantID:    "interactive",
			wantOwner: "service",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeJobID(test.jobID); got != test.wantID {
				t.Fatalf("NormalizeJobID(%q) = %q, want %q", test.jobID, got, test.wantID)
			}
			if got := NormalizeJobOwner(test.jobOwner); got != test.wantOwner {
				t.Fatalf("NormalizeJobOwner(%q) = %q, want %q", test.jobOwner, got, test.wantOwner)
			}
		})
	}
}
