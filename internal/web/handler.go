package web

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	toolkitweb "github.com/prometheus/exporter-toolkit/web"

	"pbs-exporter/internal/exporter"
)

func NewHandler(registry *prometheus.Registry, telemetryPath string, store *exporter.Store) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle(telemetryPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !store.Status().Up {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("no valid PBS snapshot\n"))
			return
		}
		_, _ = w.Write([]byte("ready\n"))
	})

	landing, err := toolkitweb.NewLandingPage(toolkitweb.LandingConfig{
		Name:        "PBS Exporter",
		Description: "Prometheus exporter for PBS clusters using a cached background collection model.",
		Version:     "",
		Links: []toolkitweb.LandingLinks{
			{Address: telemetryPath, Text: "Metrics"},
			{Address: "/-/healthy", Text: "Healthy"},
			{Address: "/-/ready", Text: "Ready"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build landing page: %w", err)
	}
	mux.Handle("/", landing)

	return mux, nil
}
