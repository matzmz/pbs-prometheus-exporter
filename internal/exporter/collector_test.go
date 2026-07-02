package exporter

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"pbs-exporter/internal/pbs"
)

func TestCollectorEmitsExporterMetricsWithoutSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()

	registry.MustRegister(NewCollector(store, Options{}))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	assertMetricValue(t, metricFamilies, "pbs_exporter_collect_errors_total", nil, 0)
	assertMetricValue(t, metricFamilies, "pbs_exporter_up", nil, 0)
	assertMetricValue(t, metricFamilies, "pbs_exporter_job_inspection_up", nil, 0)
}

func TestCollectorEmitsPBSMetricsFromSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			Version:     "2026.1.0",
			Jobs: &pbs.JobData{
				StatusCount:      map[string]int{"running": 4, "queued": 2},
				UserJobCount:     map[string]int{"alice": 2},
				QueuedJobsByUser: map[string]int{"alice": 1},
				QueueJobCount:    map[string]int{"workq": 4},
				QueueTotalCount:  map[string]int{"workq": 6},
			},
			Nodes: &pbs.NodeData{
				CountFree: 1,
				Nodes: map[string]pbs.NodeInfo{
					"node01": {
						State:           "free",
						Jobs:            2,
						CPUsAvailable:   12,
						CPUsTotal:       16,
						GPUsAvailable:   1,
						GPUsTotal:       2,
						MemoryAvailable: 64,
						MemoryTotal:     128,
					},
				},
			},
			Queues: &pbs.QueueData{
				Queues: map[string]pbs.QueueInfo{
					"workq": {
						Running:  4,
						Queued:   2,
						Enabled:  true,
						Started:  true,
						Walltime: 3600,
					},
				},
			},
			QueueSummary: pbs.QueueSummary{
				Running: 4,
				Queued:  2,
			},
			Server: &pbs.ServerData{
				State:              "Active",
				Scheduling:         true,
				TotalJobs:          6,
				JobsRunning:        4,
				JobsQueued:         2,
				ResourcesNcpus:     16,
				ResourcesMemBytes:  128,
				ResourcesNodect:    1,
				LicensesAvailable:  4,
				LicensesUsed:       2,
				MaxArraySize:       1000,
				JobHistoryEnabled:  true,
				JobHistoryDuration: 86400,
			},
		},
	}, now, 150*time.Millisecond)

	registry.MustRegister(NewCollector(store, Options{IncludeUserMetrics: true}))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	assertMetricValue(t, metricFamilies, "pbs_exporter_up", nil, 1)
	assertMetricValue(t, metricFamilies, "pbs_jobs", map[string]string{"status": "running"}, 4)
	assertMetricValue(t, metricFamilies, "pbs_jobs", map[string]string{"status": "queued"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_jobs_running_by_user", map[string]string{"user": "alice"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_node_jobs", map[string]string{"node": "node01"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_queue_jobs", map[string]string{"queue": "workq", "state": "running"}, 4)
	assertMetricValue(t, metricFamilies, "pbs_queue_jobs", map[string]string{"queue": "workq", "state": "queued"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_queue_jobs", map[string]string{"queue": "workq", "state": "total"}, 6)
	assertMetricValue(t, metricFamilies, "pbs_server_scheduling_enabled", nil, 1)
	assertMetricValue(t, metricFamilies, "pbs_server_version_info", map[string]string{"version": "2026.1.0"}, 1)
}

func TestCollectorEmitsQueueWaitMetricsFromSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			Queues: &pbs.QueueData{
				Queues: map[string]pbs.QueueInfo{
					"workq":  {},
					"emptyq": {},
				},
			},
			QueueWaits: &pbs.QueueWaitData{
				Queues: map[string]pbs.QueueWaitInfo{
					"workq": {
						Buckets: map[float64]int{
							300:         1,
							1800:        1,
							3600:        2,
							7200:        2,
							21600:       2,
							43200:       2,
							86400:       2,
							172800:      2,
							432000:      2,
							math.Inf(1): 2,
						},
						Count:  2,
						Sum:    3900,
						Oldest: 3600,
					},
				},
			},
		},
	}, now, 150*time.Millisecond)

	registry.MustRegister(NewCollector(store, Options{}))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_bucket", map[string]string{"queue": "workq", "le": "300"}, 1)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_bucket", map[string]string{"queue": "workq", "le": "3600"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_bucket", map[string]string{"queue": "workq", "le": "+Inf"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_count", map[string]string{"queue": "workq"}, 2)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_sum", map[string]string{"queue": "workq"}, 3900)
	assertMetricValue(t, metricFamilies, "pbs_queue_oldest_job_wait_seconds", map[string]string{"queue": "workq"}, 3600)

	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_bucket", map[string]string{"queue": "emptyq", "le": "300"}, 0)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_count", map[string]string{"queue": "emptyq"}, 0)
	assertMetricValue(t, metricFamilies, "pbs_queue_job_wait_seconds_sum", map[string]string{"queue": "emptyq"}, 0)
	assertMetricValue(t, metricFamilies, "pbs_queue_oldest_job_wait_seconds", map[string]string{"queue": "emptyq"}, 0)
}

func TestCollectorEmitsJobInspectionMetricsFromSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			Jobs: &pbs.JobData{
				StatusCount: map[string]int{"running": 1, "queued": 1},
			},
			JobInspection: &pbs.JobInspectionData{
				Jobs: []pbs.InspectedJob{
					{
						JobID:    "100.server",
						Queue:    "workq",
						Project:  "astro",
						JobOwner: "alice@submit01",
						JobState: "R",
						Requested: pbs.RequestedJobResources{
							MemoryBytes:     pbs.OptionalFloat64{Value: 4096, Set: true},
							WalltimeSeconds: pbs.OptionalFloat64{Value: 7200, Set: true},
							CPUCores:        pbs.OptionalFloat64{Value: 8, Set: true},
							GPUDevices:      pbs.OptionalFloat64{Value: 1, Set: true},
							MPIProcesses:    pbs.OptionalFloat64{Value: 2, Set: true},
							Nodes:           pbs.OptionalFloat64{Value: 1, Set: true},
						},
						Used: pbs.UsedJobResources{
							CPUPercent:         pbs.OptionalFloat64{Value: 210, Set: true},
							CPUTimeSeconds:     pbs.OptionalFloat64{Value: 300, Set: true},
							MemoryBytes:        pbs.OptionalFloat64{Value: 2048, Set: true},
							VirtualMemoryBytes: pbs.OptionalFloat64{Value: 8192, Set: true},
							CPUCores:           pbs.OptionalFloat64{Value: 8, Set: true},
							GPUDevices:         pbs.OptionalFloat64{Value: 0, Set: true},
							WalltimeSeconds:    pbs.OptionalFloat64{Value: 120, Set: true},
						},
						RuntimeSeconds: pbs.OptionalFloat64{Value: 600, Set: true},
					},
					{
						JobID:    "101.server",
						Queue:    "gpuq",
						Project:  "",
						JobOwner: "bob@submit02",
						JobState: "Q",
						Requested: pbs.RequestedJobResources{
							CPUCores: pbs.OptionalFloat64{Value: 4, Set: true},
						},
						QueueWaitSeconds: pbs.OptionalFloat64{Value: 900, Set: true},
					},
				},
			},
		},
		JobInspectionAttempted: true,
	}, now, 150*time.Millisecond)

	registry.MustRegister(NewCollector(store, Options{}))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	runningLabels := map[string]string{
		"job_id":     "100.server",
		"queue":      "workq",
		"project":    "astro",
		"job_owner":  "alice@submit01",
		"job_state":  "R",
	}
	queuedLabels := map[string]string{
		"job_id":     "101.server",
		"queue":      "gpuq",
		"project":    "",
		"job_owner":  "bob@submit02",
		"job_state":  "Q",
	}

	assertMetricValue(t, metricFamilies, "pbs_exporter_job_inspection_up", nil, 1)
	assertMetricValue(t, metricFamilies, "pbs_job_info", runningLabels, 1)
	assertMetricValue(t, metricFamilies, "pbs_job_requested_memory_bytes", runningLabels, 4096)
	assertMetricValue(t, metricFamilies, "pbs_job_used_cpu_percent", runningLabels, 210)
	assertMetricValue(t, metricFamilies, "pbs_job_runtime_seconds", runningLabels, 600)
	assertMetricValue(t, metricFamilies, "pbs_job_info", queuedLabels, 1)
	assertMetricValue(t, metricFamilies, "pbs_job_requested_cpu_cores", queuedLabels, 4)
	assertMetricValue(t, metricFamilies, "pbs_job_queue_wait_seconds", queuedLabels, 900)
	assertMetricMissing(t, metricFamilies, "pbs_job_used_cpu_percent", queuedLabels)
}

func TestCollectorKeepsBaseMetricsWhenJobInspectionFails(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.CollectionResult{
		Snapshot: &pbs.Snapshot{
			CollectedAt: now,
			Jobs: &pbs.JobData{
				StatusCount: map[string]int{"running": 4},
			},
		},
		JobInspectionAttempted: true,
		JobInspectionError:     errors.New("qstat json failed"),
	}, now, 150*time.Millisecond)

	registry.MustRegister(NewCollector(store, Options{}))

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	assertMetricValue(t, metricFamilies, "pbs_jobs", map[string]string{"status": "running"}, 4)
	assertMetricValue(t, metricFamilies, "pbs_exporter_job_inspection_up", nil, 0)
	assertMetricValue(t, metricFamilies, "pbs_exporter_job_inspection_errors_total", nil, 1)
	assertMetricMissing(t, metricFamilies, "pbs_job_info", map[string]string{
		"job_id":    "100.server",
		"queue":     "workq",
		"project":   "astro",
		"job_owner": "alice@submit01",
		"job_state": "R",
	})
}

func assertMetricValue(t *testing.T, metricFamilies []*dto.MetricFamily, name string, labels map[string]string, expected float64) {
	t.Helper()

	for _, family := range metricFamilies {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if labelsMatch(metric.GetLabel(), labels) {
				value := metricValue(metric)
				if value != expected {
					t.Fatalf("unexpected value for %s with labels %v: got %v want %v", name, labels, value, expected)
				}
				return
			}
		}
	}

	t.Fatalf("metric %s with labels %v not found", name, labels)
}

func assertMetricMissing(t *testing.T, metricFamilies []*dto.MetricFamily, name string, labels map[string]string) {
	t.Helper()

	for _, family := range metricFamilies {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if labelsMatch(metric.GetLabel(), labels) {
				t.Fatalf("metric %s with labels %v should be absent", name, labels)
			}
		}
	}
}

func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	if len(want) == 0 {
		return len(pairs) == 0
	}
	if len(pairs) != len(want) {
		return false
	}
	for _, pair := range pairs {
		if want[pair.GetName()] != pair.GetValue() {
			return false
		}
	}
	return true
}

func metricValue(metric *dto.Metric) float64 {
	switch {
	case metric.Gauge != nil:
		return metric.GetGauge().GetValue()
	case metric.Counter != nil:
		return metric.GetCounter().GetValue()
	default:
		return 0
	}
}
