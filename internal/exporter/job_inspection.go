package exporter

import (
	"github.com/prometheus/client_golang/prometheus"

	"pbs-exporter/internal/pbs"
)

var jobInspectionLabelNames = []string{"job_id", "queue", "project", "job_owner", "job_state"}

type jobInspectionMetrics struct {
	upDesc                *prometheus.Desc
	errorsTotalDesc       *prometheus.Desc
	lastSuccessTimestamp  *prometheus.Desc
	infoDesc              *prometheus.Desc
	requestedMemoryDesc   *prometheus.Desc
	requestedWalltimeDesc *prometheus.Desc
	requestedCPUDesc      *prometheus.Desc
	requestedGPUDesc      *prometheus.Desc
	requestedMPIProcsDesc *prometheus.Desc
	requestedNodesDesc    *prometheus.Desc
	usedCPUPercentDesc    *prometheus.Desc
	ncpusRealUsageDesc    *prometheus.Desc
	usedCPUTimeDesc       *prometheus.Desc
	usedMemoryDesc        *prometheus.Desc
	usedVirtualMemoryDesc *prometheus.Desc
	usedCPUDesc           *prometheus.Desc
	usedGPUDesc           *prometheus.Desc
	usedWalltimeDesc      *prometheus.Desc
	runtimeDesc           *prometheus.Desc
	queueWaitDesc         *prometheus.Desc
}

func newJobInspectionMetrics() jobInspectionMetrics {
	return jobInspectionMetrics{
		upDesc: prometheus.NewDesc(
			"pbs_exporter_job_inspection_up",
			"Whether the exporter currently has valid job inspection data for the latest PBS snapshot.",
			nil, nil,
		),
		errorsTotalDesc: prometheus.NewDesc(
			"pbs_exporter_job_inspection_errors_total",
			"Total number of job inspection collection or parsing errors.",
			nil, nil,
		),
		lastSuccessTimestamp: prometheus.NewDesc(
			"pbs_exporter_job_inspection_last_success_timestamp_seconds",
			"Unix timestamp of the most recent successful job inspection collection.",
			nil, nil,
		),
		infoDesc: prometheus.NewDesc(
			"pbs_job_info",
			"Static PBS job metadata from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedMemoryDesc: prometheus.NewDesc(
			"pbs_job_requested_memory_bytes",
			"Requested PBS job memory in bytes from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedWalltimeDesc: prometheus.NewDesc(
			"pbs_job_requested_walltime_seconds",
			"Requested PBS job walltime in seconds from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedCPUDesc: prometheus.NewDesc(
			"pbs_job_requested_cpu_cores",
			"Requested PBS job CPU cores from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedGPUDesc: prometheus.NewDesc(
			"pbs_job_requested_gpu_devices",
			"Requested PBS job GPU devices from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedMPIProcsDesc: prometheus.NewDesc(
			"pbs_job_requested_mpi_processes",
			"Requested PBS job MPI processes from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		requestedNodesDesc: prometheus.NewDesc(
			"pbs_job_requested_nodes",
			"Requested PBS job nodes from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedCPUPercentDesc: prometheus.NewDesc(
			"pbs_job_used_cpu_percent",
			"Used PBS job CPU percent from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		ncpusRealUsageDesc: prometheus.NewDesc(
			"pbs_job_ncpusrealusage",
			"Real CPU core usage derived from PBS resources_used.cpupercent divided by 100.",
			jobInspectionLabelNames, nil,
		),
		usedCPUTimeDesc: prometheus.NewDesc(
			"pbs_job_used_cpu_time_seconds",
			"Used PBS job CPU time in seconds from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedMemoryDesc: prometheus.NewDesc(
			"pbs_job_used_memory_bytes",
			"Used PBS job memory in bytes from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedVirtualMemoryDesc: prometheus.NewDesc(
			"pbs_job_used_virtual_memory_bytes",
			"Used PBS job virtual memory in bytes from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedCPUDesc: prometheus.NewDesc(
			"pbs_job_used_cpu_cores",
			"Used PBS job CPU cores from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedGPUDesc: prometheus.NewDesc(
			"pbs_job_used_gpu_devices",
			"Used PBS job GPU devices from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		usedWalltimeDesc: prometheus.NewDesc(
			"pbs_job_used_walltime_seconds",
			"Used PBS job walltime in seconds from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		runtimeDesc: prometheus.NewDesc(
			"pbs_job_runtime_seconds",
			"Current runtime in seconds for running PBS jobs from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
		queueWaitDesc: prometheus.NewDesc(
			"pbs_job_queue_wait_seconds",
			"Current queue wait in seconds for queued PBS jobs from the latest successful cached inspection snapshot.",
			jobInspectionLabelNames, nil,
		),
	}
}

func (m jobInspectionMetrics) descriptors() []*prometheus.Desc {
	return []*prometheus.Desc{
		m.upDesc,
		m.errorsTotalDesc,
		m.lastSuccessTimestamp,
		m.infoDesc,
		m.requestedMemoryDesc,
		m.requestedWalltimeDesc,
		m.requestedCPUDesc,
		m.requestedGPUDesc,
		m.requestedMPIProcsDesc,
		m.requestedNodesDesc,
		m.usedCPUPercentDesc,
		m.ncpusRealUsageDesc,
		m.usedCPUTimeDesc,
		m.usedMemoryDesc,
		m.usedVirtualMemoryDesc,
		m.usedCPUDesc,
		m.usedGPUDesc,
		m.usedWalltimeDesc,
		m.runtimeDesc,
		m.queueWaitDesc,
	}
}

func (m jobInspectionMetrics) collectStatus(ch chan<- prometheus.Metric, status Status) {
	ch <- prometheus.MustNewConstMetric(m.upDesc, prometheus.GaugeValue, boolFloat(status.JobInspectionUp))
	ch <- prometheus.MustNewConstMetric(m.errorsTotalDesc, prometheus.CounterValue, float64(status.JobInspectionErrorsTotal))
	ch <- prometheus.MustNewConstMetric(m.lastSuccessTimestamp, prometheus.GaugeValue, unixOrZero(status.JobInspectionLastSuccessAt))
}

func (c *Collector) collectJobInspection(ch chan<- prometheus.Metric, inspection *pbs.JobInspectionData) {
	if inspection == nil {
		return
	}

	for _, job := range inspection.Jobs {
		labels := jobInspectionLabelValues(job)
		ch <- prometheus.MustNewConstMetric(c.jobInspection.infoDesc, prometheus.GaugeValue, 1, labels...)

		emitOptionalMetric(ch, c.jobInspection.requestedMemoryDesc, job.Requested.MemoryBytes, labels)
		emitOptionalMetric(ch, c.jobInspection.requestedWalltimeDesc, job.Requested.WalltimeSeconds, labels)
		emitOptionalMetric(ch, c.jobInspection.requestedCPUDesc, job.Requested.CPUCores, labels)
		emitOptionalMetric(ch, c.jobInspection.requestedGPUDesc, job.Requested.GPUDevices, labels)
		emitOptionalMetric(ch, c.jobInspection.requestedMPIProcsDesc, job.Requested.MPIProcesses, labels)
		emitOptionalMetric(ch, c.jobInspection.requestedNodesDesc, job.Requested.Nodes, labels)

		emitOptionalMetric(ch, c.jobInspection.usedCPUPercentDesc, job.Used.CPUPercent, labels)
		emitOptionalMetric(ch, c.jobInspection.ncpusRealUsageDesc, ncpusRealUsage(job.Used.CPUPercent), labels)
		emitOptionalMetric(ch, c.jobInspection.usedCPUTimeDesc, job.Used.CPUTimeSeconds, labels)
		emitOptionalMetric(ch, c.jobInspection.usedMemoryDesc, job.Used.MemoryBytes, labels)
		emitOptionalMetric(ch, c.jobInspection.usedVirtualMemoryDesc, job.Used.VirtualMemoryBytes, labels)
		emitOptionalMetric(ch, c.jobInspection.usedCPUDesc, job.Used.CPUCores, labels)
		emitOptionalMetric(ch, c.jobInspection.usedGPUDesc, job.Used.GPUDevices, labels)
		emitOptionalMetric(ch, c.jobInspection.usedWalltimeDesc, job.Used.WalltimeSeconds, labels)
		emitOptionalMetric(ch, c.jobInspection.runtimeDesc, job.RuntimeSeconds, labels)
		emitOptionalMetric(ch, c.jobInspection.queueWaitDesc, job.QueueWaitSeconds, labels)
	}
}

func emitOptionalMetric(ch chan<- prometheus.Metric, desc *prometheus.Desc, value pbs.OptionalFloat64, labels []string) {
	if !value.Set {
		return
	}

	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value.Value, labels...)
}

func ncpusRealUsage(cpuPercent pbs.OptionalFloat64) pbs.OptionalFloat64 {
	if !cpuPercent.Set {
		return pbs.OptionalFloat64{}
	}

	return pbs.OptionalFloat64{Value: cpuPercent.Value / 100, Set: true}
}

func jobInspectionLabelValues(job pbs.InspectedJob) []string {
	return []string{
		pbs.NormalizeJobID(job.JobID),
		job.Queue,
		job.Project,
		pbs.NormalizeJobOwner(job.JobOwner),
		job.JobState,
	}
}
