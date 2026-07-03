# PBS Exporter

`pbs-exporter` is a Prometheus exporter for PBS clusters built around a cached background collection model.

This implementation is a ground-up rewrite inspired by  `[pbs-exporter](https://github.com/jxdn/pbs-exporter.git)`. The current codebase keeps the PBS-specific parsing ideas, but restructures the service to be closer to Prometheus exporter guidance and to support hardened deployments with `exporter-toolkit`.

## Design

- PBS commands run in a background worker at a configurable interval.
- Base PBS snapshot collection uses JSON-capable PBS commands to avoid fragile text parsing.
- The worker builds one coherent in-memory snapshot of jobs, nodes, queues, and server state.
- `/metrics` exposes only the latest valid snapshot.
- If a collection fails, PBS metrics are cleared and the exporter exposes failure through self-metrics.
- TLS and authentication are configured through `--web.config.file` using `exporter-toolkit`.

## Key Features

- Configurable listen address with `--web.listen-address`
- Configurable metrics path with `--web.telemetry-path`
- TLS and basic auth with `--web.config.file`
- Background PBS collection with configurable interval and per-command timeout
- Exporter self-metrics for health, collection timestamps, duration, and errors
- Optional user-labeled metrics, disabled by default to limit cardinality
- Optional per-job inspection metrics from `qstat -F json -f`, disabled by default
- Landing page plus `/-/healthy` and `/-/ready` endpoints

## Runtime Flags

### Exporter runtime

- `--version`: print build version information
- `--config.file`: exporter runtime YAML file
- `--collector.interval`: snapshot refresh interval
- `--collector.timeout`: per-command PBS timeout
- `--collector.include-user-metrics`: enable user-labeled job metrics
- `--collector.include-job-inspection-metrics`: enable per-job inspection metrics
- `--pbs.binary-dir`: directory containing `qstat` and `pbsnodes`
- `--web.telemetry-path`: metrics endpoint path
- `--log.level`: log level
- `--log.format`: log format

### Web and security

- `--web.listen-address`: listen address for the HTTP server
- `--web.config.file`: `exporter-toolkit` web configuration file for TLS and auth

## Configuration Files

Example exporter config: [packaging/examples/config.yml](/packaging/examples/config.yml)

```yaml
collector:
  interval: 60s
  timeout: 15s
  include_user_metrics: false
  include_job_inspection_metrics: false

pbs:
  binary_dir: /opt/pbs/bin

web:
  telemetry_path: /metrics
```

Example web config: [packaging/examples/web-config.yml](/packaging/examples/web-config.yml)

```yaml
tls_server_config:
  cert_file: /etc/pbs-exporter/tls/server.crt
  key_file: /etc/pbs-exporter/tls/server.key

basic_auth_users:
  prometheus: replace-with-a-bcrypt-hash
```

The web config format comes from `exporter-toolkit` and is the supported way to configure TLS and basic authentication for the exporter.

## Exporter Health Metrics

The exporter always exposes self-metrics, including:

- `pbs_exporter_up`
- `pbs_exporter_collect_errors_total`
- `pbs_exporter_last_collect_duration_seconds`
- `pbs_exporter_last_collect_timestamp_seconds`
- `pbs_exporter_last_collect_success_timestamp_seconds`
- `pbs_exporter_snapshot_timestamp_seconds`
- `pbs_exporter_job_inspection_up`
- `pbs_exporter_job_inspection_errors_total`
- `pbs_exporter_job_inspection_last_success_timestamp_seconds`

If PBS collection fails, `pbs_exporter_up` becomes `0` and PBS domain metrics disappear from exposition until collection succeeds again.

If job inspection fails while the base PBS snapshot succeeds, aggregate PBS metrics stay available and only the per-job inspection metrics disappear until inspection succeeds again.

## Job Inspection Metrics

When `collector.include_job_inspection_metrics` is enabled, the exporter performs one additional `qstat -F json -f` collection per refresh cycle and exposes per-job metrics with labels:

- `job_id`
- `queue`
- `project`
- `job_owner`
- `job_state`

Job metadata:

- `pbs_job_info`

Requested resources for all inspected jobs:

- `pbs_job_requested_memory_bytes`
- `pbs_job_requested_walltime_seconds`
- `pbs_job_requested_cpu_cores`
- `pbs_job_requested_gpu_devices`
- `pbs_job_requested_mpi_processes`
- `pbs_job_requested_nodes`

Used resources for running jobs only (`job_state = R`):

- `pbs_job_used_cpu_percent`
- `pbs_job_used_cpu_time_seconds`
- `pbs_job_used_memory_bytes`
- `pbs_job_used_virtual_memory_bytes`
- `pbs_job_used_cpu_cores`
- `pbs_job_used_gpu_devices`
- `pbs_job_used_walltime_seconds`

Timing metrics in v1:

- `pbs_job_runtime_seconds` for running jobs, derived from `stime`
- `pbs_job_queue_wait_seconds` for queued jobs, derived from `qtime`

## Queue Wait Metrics

The exporter exposes snapshot-based wait-time buckets for jobs currently in `job_state = Q`, aggregated by PBS queue:

- `pbs_queue_job_wait_seconds_bucket{queue,le}`
- `pbs_queue_job_wait_seconds_count{queue}`
- `pbs_queue_job_wait_seconds_sum{queue}`
- `pbs_queue_oldest_job_wait_seconds{queue}`

Wait time is calculated as `snapshot_collected_at - qtime`. The bucket thresholds are:

```text
5m, 30m, 1h, 2h, 6h, 12h, 1d, 2d, 5d, +Inf
```

These metrics describe the current queue snapshot. They are gauge-valued cumulative buckets, not event counters, so do not use `rate()` or `increase()` on them.

Current p95 wait per queue:

```promql
histogram_quantile(
  0.95,
  sum by (queue, le) (pbs_queue_job_wait_seconds_bucket)
)
```

Jobs waiting more than one day:

```promql
pbs_queue_job_wait_seconds_count
- ignoring(le) pbs_queue_job_wait_seconds_bucket{le="86400"}
```

Worst queues by oldest queued job:

```promql
topk(10, pbs_queue_oldest_job_wait_seconds)
```

## Build

To build a static Linux `amd64` binary in Docker:

```bash
./scripts/build-static.sh
```

The build script derives the version from Git:

- exact tag builds use the matching tag such as `0.3.0`
- non-tagged builds fall back to `git describe --tags --dirty --always`
- `--version` reports the injected version, revision, branch, build user, and build date

You can override any injected field for CI or packaging with environment variables such as `BUILD_VERSION`, `BUILD_REVISION`, `BUILD_BRANCH`, `BUILD_DATE`, and `BUILD_USER`.

This writes the artifact to:

```bash
dist/pbs-exporter-linux-amd64
```

The build uses `CGO_ENABLED=0`, so the resulting binary does not depend on `glibc`.

## Local Run

Without TLS/auth:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 sh -c 'go run . --collector.interval=30s --collector.timeout=10s'
```

With config files:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 sh -c 'go run . --config.file=packaging/examples/config.yml --web.config.file=packaging/examples/web-config.yml'
```

To inspect the local development version:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 sh -c 'go run . --version'
```

## systemd

The packaged unit is [packaging/systemd/pbs-exporter.service](/packaging/systemd/pbs-exporter.service). It runs as `prom-exporter` and expects:

- `/usr/local/bin/pbs-exporter`
- `/etc/pbs-exporter/config.yml`
- `/etc/pbs-exporter/web-config.yml`

It also sets `PATH` to include `/opt/pbs/bin` for PBS CLI installations that live outside the default system path.

## Operational Notes

- Prefer keeping `collector.include_user_metrics` disabled unless you specifically need per-user series.
- Enable `collector.include_job_inspection_metrics` only if per-job cardinality is acceptable for your Prometheus environment.
- Tune `collector.timeout` low enough to avoid long hangs in PBS CLI calls.
- Alert on `pbs_exporter_up == 0`.
- Use `/-/ready` if you want a probe that reflects whether a valid PBS snapshot is currently available.

## Development

Full module test run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 sh -c 'go test ./...'
```
