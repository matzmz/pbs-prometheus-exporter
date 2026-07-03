package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/common/promslog"
	toolkitweb "github.com/prometheus/exporter-toolkit/web"

	"pbs-exporter/internal/buildinfo"
	"pbs-exporter/internal/config"
	"pbs-exporter/internal/exporter"
	"pbs-exporter/internal/pbs"
	webhandler "pbs-exporter/internal/web"
)

func main() {
	parsed, err := config.Parse(os.Args[1:])
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(2)
	}

	logger := promslog.New(parsed.Log)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store := exporter.NewStore()
	client := pbs.NewClient(parsed.Runtime.PBS.BinaryDir, parsed.Runtime.Collector.Timeout, pbs.ClientOptions{
		IncludeJobInspection: parsed.Runtime.Collector.IncludeJobInspectionMetrics,
	}, logger)
	worker := exporter.NewWorker(client, store, parsed.Runtime.Collector.Interval, logger)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		collectors.NewBuildInfoCollector(),
		exporter.NewCollector(store, exporter.Options{
			IncludeUserMetrics: parsed.Runtime.Collector.IncludeUserMetrics,
		}),
	)

	handler, err := webhandler.NewHandler(registry, parsed.Runtime.Web.TelemetryPath, store)
	if err != nil {
		logger.Error("failed to build HTTP handler", "err", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler: handler,
	}

	go worker.Run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), parsed.Runtime.Collector.Timeout)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP shutdown failed", "err", err)
		}
	}()

	logger.Info("starting pbs-exporter",
		"version", buildinfo.Short(),
		"listen_addresses", *parsed.Web.WebListenAddresses,
		"telemetry_path", parsed.Runtime.Web.TelemetryPath,
		"collector_interval", parsed.Runtime.Collector.Interval.String(),
		"collector_timeout", parsed.Runtime.Collector.Timeout.String(),
		"include_user_metrics", parsed.Runtime.Collector.IncludeUserMetrics,
		"include_job_inspection_metrics", parsed.Runtime.Collector.IncludeJobInspectionMetrics,
	)

	if err := toolkitweb.ListenAndServe(server, parsed.Web, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server failed", "err", err)
		os.Exit(1)
	}
}
