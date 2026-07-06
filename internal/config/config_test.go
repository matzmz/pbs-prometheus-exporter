package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	parsed, err := Parse([]string{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Runtime.Web.TelemetryPath != "/metrics" {
		t.Fatalf("unexpected telemetry path: %q", parsed.Runtime.Web.TelemetryPath)
	}
	if parsed.Runtime.Collector.Interval != time.Minute {
		t.Fatalf("unexpected interval: %v", parsed.Runtime.Collector.Interval)
	}
	if parsed.Runtime.Collector.Timeout != 15*time.Second {
		t.Fatalf("unexpected timeout: %v", parsed.Runtime.Collector.Timeout)
	}
	if parsed.Runtime.Collector.IncludeUserMetrics {
		t.Fatal("expected user metrics to be disabled by default")
	}
	if parsed.Runtime.Collector.IncludeJobInspectionMetrics {
		t.Fatal("expected job inspection metrics to be disabled by default")
	}
	if parsed.Runtime.Collector.IncludeJobSampleHistograms {
		t.Fatal("expected job sample histograms to be disabled by default")
	}
	if got := formatNumericBuckets(parsed.Runtime.Collector.JobSampleCPUBuckets); got != "1,2,4,8,16,32,64,128,256" {
		t.Fatalf("unexpected cpu bucket defaults: %q", got)
	}
	if got := formatMemoryBuckets(parsed.Runtime.Collector.JobSampleMemoryBuckets); got != "1073741824,2147483648,4294967296,8589934592,17179869184,34359738368,68719476736,137438953472,274877906944,549755813888" {
		t.Fatalf("unexpected memory bucket defaults: %q", got)
	}
	if got := formatDurationBuckets(parsed.Runtime.Collector.JobSampleWalltimeBuckets); got != "30m0s,1h0m0s,2h0m0s,4h0m0s,8h0m0s,12h0m0s,24h0m0s,48h0m0s,96h0m0s,168h0m0s" {
		t.Fatalf("unexpected walltime bucket defaults: %q", got)
	}
	if len(*parsed.Web.WebListenAddresses) != 1 || (*parsed.Web.WebListenAddresses)[0] != ":9785" {
		t.Fatalf("unexpected listen addresses: %#v", *parsed.Web.WebListenAddresses)
	}
}

func TestFlagsOverrideDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	configYAML := []byte("collector:\n  interval: 10m\n  timeout: 45s\n  include_job_inspection_metrics: false\n  include_job_sample_histograms: false\n  job_sample_cpu_buckets: [1, 8, 32]\n  job_sample_memory_buckets: [1gb, 4gb, 16gb]\n  job_sample_walltime_buckets: [30m, 2h, 24h]\n  job_sample_mpi_buckets: [1, 4, 16]\n  job_sample_node_buckets: [1, 2, 8]\n  job_sample_gpu_buckets: [1, 2, 4]\npbs:\n  binary_dir: /opt/pbs/bin\nweb:\n  telemetry_path: /from-config\n")
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	parsed, err := Parse([]string{
		"--config.file", configPath,
		"--collector.interval=30s",
		"--collector.timeout=5s",
		"--collector.include-user-metrics",
		"--collector.include-job-inspection-metrics",
		"--collector.include-job-sample-histograms",
		"--collector.job-sample-cpu-buckets=2,4,8",
		"--collector.job-sample-memory-buckets=2gb,8gb",
		"--collector.job-sample-walltime-buckets=1h,12h",
		"--collector.job-sample-mpi-buckets=2,8",
		"--collector.job-sample-node-buckets=1,4",
		"--collector.job-sample-gpu-buckets=1,8",
		"--pbs.binary-dir=/usr/pbs/bin",
		"--web.telemetry-path=/custom-metrics",
		"--web.listen-address=:9999",
		"--web.config.file=/etc/pbs-exporter/web-config.yml",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Runtime.Collector.Interval != 30*time.Second {
		t.Fatalf("unexpected interval: %v", parsed.Runtime.Collector.Interval)
	}
	if parsed.Runtime.Collector.Timeout != 5*time.Second {
		t.Fatalf("unexpected timeout: %v", parsed.Runtime.Collector.Timeout)
	}
	if !parsed.Runtime.Collector.IncludeUserMetrics {
		t.Fatal("expected include_user_metrics to be enabled")
	}
	if !parsed.Runtime.Collector.IncludeJobInspectionMetrics {
		t.Fatal("expected include_job_inspection_metrics to be enabled")
	}
	if !parsed.Runtime.Collector.IncludeJobSampleHistograms {
		t.Fatal("expected include_job_sample_histograms to be enabled")
	}
	if got := formatNumericBuckets(parsed.Runtime.Collector.JobSampleCPUBuckets); got != "2,4,8" {
		t.Fatalf("unexpected cpu bucket override: %q", got)
	}
	if got := formatMemoryBuckets(parsed.Runtime.Collector.JobSampleMemoryBuckets); got != "2147483648,8589934592" {
		t.Fatalf("unexpected memory bucket override: %q", got)
	}
	if got := formatDurationBuckets(parsed.Runtime.Collector.JobSampleWalltimeBuckets); got != "1h0m0s,12h0m0s" {
		t.Fatalf("unexpected walltime bucket override: %q", got)
	}
	if got := formatNumericBuckets(parsed.Runtime.Collector.JobSampleMPIBuckets); got != "2,8" {
		t.Fatalf("unexpected mpi bucket override: %q", got)
	}
	if got := formatNumericBuckets(parsed.Runtime.Collector.JobSampleNodeBuckets); got != "1,4" {
		t.Fatalf("unexpected node bucket override: %q", got)
	}
	if got := formatNumericBuckets(parsed.Runtime.Collector.JobSampleGPUBuckets); got != "1,8" {
		t.Fatalf("unexpected gpu bucket override: %q", got)
	}
	if parsed.Runtime.PBS.BinaryDir != "/usr/pbs/bin" {
		t.Fatalf("unexpected binary dir: %q", parsed.Runtime.PBS.BinaryDir)
	}
	if parsed.Runtime.Web.TelemetryPath != "/custom-metrics" {
		t.Fatalf("unexpected telemetry path: %q", parsed.Runtime.Web.TelemetryPath)
	}
	if len(*parsed.Web.WebListenAddresses) != 1 || (*parsed.Web.WebListenAddresses)[0] != ":9999" {
		t.Fatalf("unexpected listen addresses: %#v", *parsed.Web.WebListenAddresses)
	}
	if *parsed.Web.WebConfigFile != "/etc/pbs-exporter/web-config.yml" {
		t.Fatalf("unexpected web config file: %q", *parsed.Web.WebConfigFile)
	}
}

func TestParseRejectsInvalidRuntimeOverrides(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "zero interval",
			args: []string{"--collector.interval=0s"},
			want: "collector.interval must be greater than zero",
		},
		{
			name: "negative timeout",
			args: []string{"--collector.timeout=-5s"},
			want: "collector.timeout must be greater than zero",
		},
		{
			name: "missing leading slash in telemetry path",
			args: []string{"--web.telemetry-path=metrics"},
			want: "web.telemetry-path must start with '/' and must not be '/'",
		},
		{
			name: "root telemetry path",
			args: []string{"--web.telemetry-path=/"},
			want: "web.telemetry-path must start with '/' and must not be '/'",
		},
		{
			name: "invalid cpu buckets",
			args: []string{"--collector.job-sample-cpu-buckets=4,4"},
			want: "collector.job_sample_cpu_buckets buckets must be strictly increasing",
		},
		{
			name: "invalid memory bucket value",
			args: []string{"--collector.job-sample-memory-buckets=bogus"},
			want: "parse --collector.job-sample-memory-buckets",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.args)
			if err == nil {
				t.Fatal("expected Parse to return an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse error = %q, want substring %q", err.Error(), test.want)
			}
		})
	}
}

func TestLoadFileRejectsUnknownYAMLKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	configYAML := []byte("collector:\n  interval: 30s\nunexpected: true\n")
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := LoadFile(configPath)
	if err == nil {
		t.Fatal("expected LoadFile to reject unknown keys")
	}
	if !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("LoadFile error = %q, want unknown key information", err.Error())
	}
}
