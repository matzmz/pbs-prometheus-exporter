# PBS Queue Wait Bucket Metrics Design

## Goal

Increase PBS cluster observability by exposing how long currently queued jobs have been waiting, aggregated per queue without job-, user-, project-, or job-name labels.

The primary use case is answering:

- Which queues currently have the oldest waiting jobs?
- What is the current p50/p95 wait time per queue?
- How many jobs per queue have waited more than operational thresholds such as 1 day, 2 days, or 5 days?

## Scope

This design adds snapshot-based queue wait metrics only. It does not add per-job metrics, per-user wait metrics, scheduler diagnostics, requested-resource backlog metrics, or parsing of job eligibility time.

Wait time is defined as:

```text
snapshot_collected_at - qtime
```

where `qtime` is the PBS queued/submission timestamp for a job currently in `job_state = Q`.

## Metric Semantics

Expose a cumulative bucket distribution per queue:

```text
pbs_queue_job_wait_seconds_bucket{queue="<queue>",le="<upper_bound>"}
pbs_queue_job_wait_seconds_count{queue="<queue>"}
pbs_queue_job_wait_seconds_sum{queue="<queue>"}
pbs_queue_oldest_job_wait_seconds{queue="<queue>"}
```

The bucket values are cumulative counts for the latest valid PBS snapshot:

```text
pbs_queue_job_wait_seconds_bucket{queue="workq",le="300"} = number of queued jobs in workq with wait <= 300 seconds
```

These metrics represent the distribution of the current queue state. They are not event counters and must not be queried with `rate()` or `increase()`.

`histogram_quantile()` can be used directly on instant vectors or range-reduced instant vectors because the bucket series are cumulative by `le` within each scrape.

## Default Buckets

Use fixed bucket upper bounds:

```text
300, 1800, 3600, 7200, 21600, 43200, 86400, 172800, 432000, +Inf
```

Human-readable thresholds:

```text
5m, 30m, 1h, 2h, 6h, 12h, 1d, 2d, 5d, +Inf
```

The 15-minute bucket is intentionally excluded. The 5-day bucket is included for long-running operational backlogs.

## Cardinality

The only unbounded label is `queue`, whose cardinality is bounded by PBS queue count in normal deployments. `le` has a fixed set of 10 values.

Per queue series added:

- 10 bucket series
- 1 count series
- 1 sum series
- 1 oldest wait series

Total added series:

```text
queue_count * 13
```

No labels may include job ID, username, project, job name, vnode, host, or reason strings.

## Collection

The current exporter collects jobs with `qstat -t`, which is enough for state and queue counts but not enough for robust wait-time calculation. Add a full job collection source using `qstat -f` or an equivalent PBS command that exposes at least:

- job ID delimiter
- `job_state`
- `queue`
- `qtime`

The implementation should parse all job records internally, aggregate queued-job waits per queue, and discard per-job data before storing the snapshot.

Use the snapshot collection timestamp as the calculation reference. Clamp negative waits to zero to tolerate clock skew or parse edge cases.

If a queued job is missing `queue` or `qtime`, exclude that job from the wait distribution. Queue job count metrics from the existing parser remain authoritative for total queued counts.

## Data Model

Add queue wait aggregation to the PBS snapshot model, for example:

```go
type QueueWaitData struct {
    Queues map[string]QueueWaitInfo
}

type QueueWaitInfo struct {
    Buckets map[float64]int
    Count   int
    Sum     float64
    Oldest  float64
}
```

The exact Go shape can follow local style, but the stored model must already be aggregated by queue.

## Prometheus Exposure

Expose the bucket, count, sum, and oldest metrics from the existing custom collector. Because these are snapshot values, emit each `_bucket`, `_count`, `_sum`, and `_oldest` series with `prometheus.GaugeValue`. Do not model them as event counters.

This deliberately creates histogram-compatible cumulative bucket series for instant-vector queries while preserving snapshot semantics. The values may decrease between scrapes when queued jobs start or leave the queue.

Metric help text must clearly say "from the latest successful cached collection" and "current queued jobs" to avoid event-latency interpretation.

If no queued jobs exist for a queue, emit zero count, zero sum, zero oldest, and zero bucket counts only if the queue exists in `snapshot.Queues`. This keeps dashboards stable for empty queues without inventing unknown queue names.

## Dashboard Queries

Current p95 wait per queue:

```promql
histogram_quantile(
  0.95,
  sum by (queue, le) (pbs_queue_job_wait_seconds_bucket)
)
```

Current average wait per queue:

```promql
pbs_queue_job_wait_seconds_sum
/
clamp_min(pbs_queue_job_wait_seconds_count, 1)
```

Jobs waiting more than 1 day:

```promql
pbs_queue_job_wait_seconds_count
- ignoring(le) pbs_queue_job_wait_seconds_bucket{le="86400"}
```

Worst queues by oldest waiting job:

```promql
topk(10, pbs_queue_oldest_job_wait_seconds)
```

Recommended Grafana panels:

- Table per queue: queued jobs, average wait, p50, p95, oldest wait.
- Time series: p95 wait per queue over time.
- Bar gauge: top queues by oldest wait.
- Stat or table: jobs above 1d, 2d, and 5d thresholds.

## Error Handling

If the full job command fails and queue-wait metrics are part of the required snapshot collection, the collector should treat the PBS collection as failed, matching the existing all-or-nothing snapshot behavior.

Malformed individual job records should not fail the collection. Skip records that lack required fields for wait aggregation, and keep the parser deterministic.

## Tests

Add parser tests for:

- queued jobs in multiple queues produce cumulative bucket counts, count, sum, and oldest wait
- running and held jobs are ignored
- missing `qtime` or missing `queue` records are skipped
- negative waits are clamped to zero
- empty queues expose zero wait metrics when queue data exists

Add collector tests for:

- emitted bucket, count, sum, and oldest series
- no user/job/project labels are present
- bucket labels match the fixed default list

## Non-Goals

- No per-job metrics.
- No per-user queue wait metrics.
- No requested CPU, memory, GPU, or walltime backlog metrics.
- No scheduler reason or eligibility-time metrics.
- No custom bucket configuration in the first implementation.
