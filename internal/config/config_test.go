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
	if len(*parsed.Web.WebListenAddresses) != 1 || (*parsed.Web.WebListenAddresses)[0] != ":9785" {
		t.Fatalf("unexpected listen addresses: %#v", *parsed.Web.WebListenAddresses)
	}
}

func TestFlagsOverrideDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	configYAML := []byte("collector:\n  interval: 10m\n  timeout: 45s\n  include_job_inspection_metrics: false\npbs:\n  binary_dir: /opt/pbs/bin\nweb:\n  telemetry_path: /from-config\n")
	if err := os.WriteFile(configPath, configYAML, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	parsed, err := Parse([]string{
		"--config.file", configPath,
		"--collector.interval=30s",
		"--collector.timeout=5s",
		"--collector.include-user-metrics",
		"--collector.include-job-inspection-metrics",
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
