package web

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"pbs-exporter/internal/exporter"
)

func TestNewHandlerRejectsInvalidTelemetryPath(t *testing.T) {
	store := exporter.NewStore()
	registry := prometheus.NewRegistry()

	tests := []string{"", "metrics", "/"}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, err := NewHandler(registry, path, store)
			if err == nil {
				t.Fatalf("expected NewHandler to reject %q", path)
			}
		})
	}
}
