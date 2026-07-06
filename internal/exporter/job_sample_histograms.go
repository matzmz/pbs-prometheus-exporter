package exporter

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"pbs-exporter/internal/pbs"
)

type JobSampleHistogramConfig struct {
	Enabled         bool
	CPUBuckets      []float64
	MemoryBuckets   []float64
	WalltimeBuckets []float64
	MPIBuckets      []float64
	NodeBuckets     []float64
	GPUBuckets      []float64
}

type histogramSnapshot struct {
	Count   uint64
	Sum     float64
	Buckets map[float64]uint64
}

type JobSampleHistogramSnapshots struct {
	CPU      histogramSnapshot
	Memory   histogramSnapshot
	Walltime histogramSnapshot
	MPI      histogramSnapshot
	Nodes    histogramSnapshot
	GPU      histogramSnapshot
}

type histogramAccumulator struct {
	buckets []float64
	counts  []uint64
	count   uint64
	sum     float64
}

func newHistogramAccumulator(buckets []float64) *histogramAccumulator {
	cloned := make([]float64, len(buckets))
	copy(cloned, buckets)

	return &histogramAccumulator{
		buckets: cloned,
		counts:  make([]uint64, len(cloned)),
	}
}

func (h *histogramAccumulator) Observe(value float64) {
	h.count++
	h.sum += value
	for index, bucket := range h.buckets {
		if value <= bucket {
			h.counts[index]++
		}
	}
}

func (h *histogramAccumulator) Snapshot() histogramSnapshot {
	buckets := make(map[float64]uint64, len(h.buckets))
	for index, bucket := range h.buckets {
		buckets[bucket] = h.counts[index]
	}

	return histogramSnapshot{
		Count:   h.count,
		Sum:     h.sum,
		Buckets: buckets,
	}
}

type jobSampleHistograms struct {
	cpu      *histogramAccumulator
	memory   *histogramAccumulator
	walltime *histogramAccumulator
	mpi      *histogramAccumulator
	nodes    *histogramAccumulator
	gpu      *histogramAccumulator
}

func newJobSampleHistograms(config JobSampleHistogramConfig) *jobSampleHistograms {
	if !config.Enabled {
		return nil
	}

	return &jobSampleHistograms{
		cpu:      newHistogramAccumulator(config.CPUBuckets),
		memory:   newHistogramAccumulator(config.MemoryBuckets),
		walltime: newHistogramAccumulator(config.WalltimeBuckets),
		mpi:      newHistogramAccumulator(config.MPIBuckets),
		nodes:    newHistogramAccumulator(config.NodeBuckets),
		gpu:      newHistogramAccumulator(config.GPUBuckets),
	}
}

func (h *jobSampleHistograms) Observe(inspection *pbs.JobInspectionData) {
	if h == nil || inspection == nil {
		return
	}

	for _, job := range inspection.Jobs {
		if !strings.EqualFold(job.JobState, "R") {
			continue
		}

		observeOptionalHistogram(h.cpu, job.Requested.CPUCores)
		observeOptionalHistogram(h.memory, job.Requested.MemoryBytes)
		observeOptionalHistogram(h.walltime, job.Requested.WalltimeSeconds)
		observeOptionalHistogram(h.mpi, job.Requested.MPIProcesses)
		observeOptionalHistogram(h.nodes, job.Requested.Nodes)
		observeOptionalHistogram(h.gpu, job.Requested.GPUDevices)
	}
}

func (h *jobSampleHistograms) Snapshot() *JobSampleHistogramSnapshots {
	if h == nil {
		return nil
	}

	return &JobSampleHistogramSnapshots{
		CPU:      h.cpu.Snapshot(),
		Memory:   h.memory.Snapshot(),
		Walltime: h.walltime.Snapshot(),
		MPI:      h.mpi.Snapshot(),
		Nodes:    h.nodes.Snapshot(),
		GPU:      h.gpu.Snapshot(),
	}
}

func observeOptionalHistogram(histogram *histogramAccumulator, value pbs.OptionalFloat64) {
	if histogram == nil || !value.Set {
		return
	}

	histogram.Observe(value.Value)
}

type jobSampleHistogramMetrics struct {
	cpuDesc      *prometheus.Desc
	memoryDesc   *prometheus.Desc
	walltimeDesc *prometheus.Desc
	mpiDesc      *prometheus.Desc
	nodesDesc    *prometheus.Desc
	gpuDesc      *prometheus.Desc
}

func newJobSampleHistogramMetrics() jobSampleHistogramMetrics {
	return jobSampleHistogramMetrics{
		cpuDesc: prometheus.NewDesc(
			"pbs_job_running_requested_cpu_cores_distribution",
			"Cumulative histogram of sampled observations of requested CPU cores for running PBS jobs.",
			nil, nil,
		),
		memoryDesc: prometheus.NewDesc(
			"pbs_job_running_requested_memory_bytes_distribution",
			"Cumulative histogram of sampled observations of requested memory bytes for running PBS jobs.",
			nil, nil,
		),
		walltimeDesc: prometheus.NewDesc(
			"pbs_job_running_requested_walltime_seconds_distribution",
			"Cumulative histogram of sampled observations of requested walltime seconds for running PBS jobs.",
			nil, nil,
		),
		mpiDesc: prometheus.NewDesc(
			"pbs_job_running_requested_mpi_processes_distribution",
			"Cumulative histogram of sampled observations of requested MPI processes for running PBS jobs.",
			nil, nil,
		),
		nodesDesc: prometheus.NewDesc(
			"pbs_job_running_requested_nodes_distribution",
			"Cumulative histogram of sampled observations of requested node counts for running PBS jobs.",
			nil, nil,
		),
		gpuDesc: prometheus.NewDesc(
			"pbs_job_running_requested_gpu_devices_distribution",
			"Cumulative histogram of sampled observations of requested GPU devices for running PBS jobs.",
			nil, nil,
		),
	}
}

func (m jobSampleHistogramMetrics) descriptors() []*prometheus.Desc {
	return []*prometheus.Desc{
		m.cpuDesc,
		m.memoryDesc,
		m.walltimeDesc,
		m.mpiDesc,
		m.nodesDesc,
		m.gpuDesc,
	}
}

func (m jobSampleHistogramMetrics) collect(ch chan<- prometheus.Metric, snapshots *JobSampleHistogramSnapshots) {
	if snapshots == nil {
		return
	}

	ch <- prometheus.MustNewConstHistogram(m.cpuDesc, snapshots.CPU.Count, snapshots.CPU.Sum, snapshots.CPU.Buckets)
	ch <- prometheus.MustNewConstHistogram(m.memoryDesc, snapshots.Memory.Count, snapshots.Memory.Sum, snapshots.Memory.Buckets)
	ch <- prometheus.MustNewConstHistogram(m.walltimeDesc, snapshots.Walltime.Count, snapshots.Walltime.Sum, snapshots.Walltime.Buckets)
	ch <- prometheus.MustNewConstHistogram(m.mpiDesc, snapshots.MPI.Count, snapshots.MPI.Sum, snapshots.MPI.Buckets)
	ch <- prometheus.MustNewConstHistogram(m.nodesDesc, snapshots.Nodes.Count, snapshots.Nodes.Sum, snapshots.Nodes.Buckets)
	ch <- prometheus.MustNewConstHistogram(m.gpuDesc, snapshots.GPU.Count, snapshots.GPU.Sum, snapshots.GPU.Buckets)
}
