package exporter

import (
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
}

func TestCollectorEmitsPBSMetricsFromSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	store := NewStore()
	now := time.Unix(1700000000, 0).UTC()

	store.UpdateSuccess(&pbs.Snapshot{
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
