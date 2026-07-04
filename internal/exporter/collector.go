package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Options struct {
	IncludeUserMetrics bool
}

type Collector struct {
	store              *Store
	includeUserMetrics bool
	jobInspection      jobInspectionMetrics

	exporterBuildInfoDesc           *prometheus.Desc
	exporterUpDesc                  *prometheus.Desc
	collectErrorsTotalDesc          *prometheus.Desc
	lastCollectDurationDesc         *prometheus.Desc
	lastCollectTimestampDesc        *prometheus.Desc
	lastCollectSuccessTimestampDesc *prometheus.Desc
	snapshotTimestampDesc           *prometheus.Desc
	serverVersionInfoDesc           *prometheus.Desc
	jobsDesc                        *prometheus.Desc
	jobsRunningByUserDesc           *prometheus.Desc
	jobsQueuedByUserDesc            *prometheus.Desc
	jobsRunningByQueueDesc          *prometheus.Desc
	jobsTotalByQueueDesc            *prometheus.Desc
	nodesDesc                       *prometheus.Desc
	nodeStateDesc                   *prometheus.Desc
	nodeJobsDesc                    *prometheus.Desc
	nodeCPUDesc                     *prometheus.Desc
	nodeGPUDesc                     *prometheus.Desc
	nodeMemoryDesc                  *prometheus.Desc
	nodeCPUUtilizationDesc          *prometheus.Desc
	nodeGPUUtilizationDesc          *prometheus.Desc
	nodeMemoryUtilizationDesc       *prometheus.Desc
	clusterCPUDesc                  *prometheus.Desc
	clusterGPUDesc                  *prometheus.Desc
	clusterMemoryDesc               *prometheus.Desc
	queueJobsDesc                   *prometheus.Desc
	queueJobWaitBucketDesc          *prometheus.Desc
	queueJobWaitCountDesc           *prometheus.Desc
	queueJobWaitSumDesc             *prometheus.Desc
	queueOldestJobWaitDesc          *prometheus.Desc
	queueEnabledDesc                *prometheus.Desc
	queueStartedDesc                *prometheus.Desc
	queueWalltimeDesc               *prometheus.Desc
	serverActiveDesc                *prometheus.Desc
	serverSchedulingEnabledDesc     *prometheus.Desc
	serverJobsDesc                  *prometheus.Desc
	serverAssignedCPUsDesc          *prometheus.Desc
	serverAssignedMemoryDesc        *prometheus.Desc
	serverAssignedNodesDesc         *prometheus.Desc
	serverLicensesDesc              *prometheus.Desc
	serverMaxArraySizeDesc          *prometheus.Desc
	serverJobHistoryEnabledDesc     *prometheus.Desc
	serverJobHistoryDurationDesc    *prometheus.Desc
}

func NewCollector(store *Store, options Options) *Collector {
	return &Collector{
		store:              store,
		includeUserMetrics: options.IncludeUserMetrics,
		jobInspection:      newJobInspectionMetrics(),
		exporterBuildInfoDesc: prometheus.NewDesc(
			"pbs_exporter_build_info",
			"Build information for this pbs-exporter instance.",
			[]string{"version", "revision", "branch"}, nil,
		),
		exporterUpDesc: prometheus.NewDesc(
			"pbs_exporter_up",
			"Whether the exporter currently has a valid PBS snapshot.",
			nil, nil,
		),
		collectErrorsTotalDesc: prometheus.NewDesc(
			"pbs_exporter_collect_errors_total",
			"Total number of background PBS collection errors.",
			nil, nil,
		),
		lastCollectDurationDesc: prometheus.NewDesc(
			"pbs_exporter_last_collect_duration_seconds",
			"Duration of the last background PBS collection in seconds.",
			nil, nil,
		),
		lastCollectTimestampDesc: prometheus.NewDesc(
			"pbs_exporter_last_collect_timestamp_seconds",
			"Unix timestamp of the most recent background PBS collection attempt.",
			nil, nil,
		),
		lastCollectSuccessTimestampDesc: prometheus.NewDesc(
			"pbs_exporter_last_collect_success_timestamp_seconds",
			"Unix timestamp of the most recent successful background PBS collection.",
			nil, nil,
		),
		snapshotTimestampDesc: prometheus.NewDesc(
			"pbs_exporter_snapshot_timestamp_seconds",
			"Unix timestamp embedded in the currently exposed PBS snapshot.",
			nil, nil,
		),
		serverVersionInfoDesc: prometheus.NewDesc(
			"pbs_server_version_info",
			"PBS server version info from the latest successful cached collection.",
			[]string{"version"}, nil,
		),
		jobsDesc: prometheus.NewDesc(
			"pbs_jobs",
			"Number of PBS jobs by status from the latest successful cached collection.",
			[]string{"status"}, nil,
		),
		jobsRunningByUserDesc: prometheus.NewDesc(
			"pbs_jobs_running_by_user",
			"Running PBS jobs by user from the latest successful cached collection.",
			[]string{"user"}, nil,
		),
		jobsQueuedByUserDesc: prometheus.NewDesc(
			"pbs_jobs_queued_by_user",
			"Queued PBS jobs by user from the latest successful cached collection.",
			[]string{"user"}, nil,
		),
		jobsRunningByQueueDesc: prometheus.NewDesc(
			"pbs_jobs_running_by_queue",
			"Running PBS jobs by queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		jobsTotalByQueueDesc: prometheus.NewDesc(
			"pbs_jobs_total_by_queue",
			"Total PBS jobs by queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		nodesDesc: prometheus.NewDesc(
			"pbs_nodes",
			"Number of PBS nodes by state from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		nodeStateDesc: prometheus.NewDesc(
			"pbs_node_state",
			"State marker for a PBS node from the latest successful cached collection.",
			[]string{"node", "state"}, nil,
		),
		nodeJobsDesc: prometheus.NewDesc(
			"pbs_node_jobs",
			"Jobs assigned to a PBS node from the latest successful cached collection.",
			[]string{"node"}, nil,
		),
		nodeCPUDesc: prometheus.NewDesc(
			"pbs_node_cpu_cores",
			"CPU core counts for a PBS node from the latest successful cached collection.",
			[]string{"node", "state"}, nil,
		),
		nodeGPUDesc: prometheus.NewDesc(
			"pbs_node_gpu_devices",
			"GPU device counts for a PBS node from the latest successful cached collection.",
			[]string{"node", "state"}, nil,
		),
		nodeMemoryDesc: prometheus.NewDesc(
			"pbs_node_memory_bytes",
			"Memory bytes for a PBS node from the latest successful cached collection.",
			[]string{"node", "state"}, nil,
		),
		nodeCPUUtilizationDesc: prometheus.NewDesc(
			"pbs_node_cpu_utilization_ratio",
			"CPU utilization ratio for a PBS node from the latest successful cached collection.",
			[]string{"node"}, nil,
		),
		nodeGPUUtilizationDesc: prometheus.NewDesc(
			"pbs_node_gpu_utilization_ratio",
			"GPU utilization ratio for a PBS node from the latest successful cached collection.",
			[]string{"node"}, nil,
		),
		nodeMemoryUtilizationDesc: prometheus.NewDesc(
			"pbs_node_memory_utilization_ratio",
			"Memory utilization ratio for a PBS node from the latest successful cached collection.",
			[]string{"node"}, nil,
		),
		clusterCPUDesc: prometheus.NewDesc(
			"pbs_cluster_cpu_cores",
			"Cluster CPU core counts from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		clusterGPUDesc: prometheus.NewDesc(
			"pbs_cluster_gpu_devices",
			"Cluster GPU device counts from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		clusterMemoryDesc: prometheus.NewDesc(
			"pbs_cluster_memory_bytes",
			"Cluster memory bytes from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		queueJobsDesc: prometheus.NewDesc(
			"pbs_queue_jobs",
			"Jobs per PBS queue and state from the latest successful cached collection.",
			[]string{"queue", "state"}, nil,
		),
		queueJobWaitBucketDesc: prometheus.NewDesc(
			"pbs_queue_job_wait_seconds_bucket",
			"Cumulative snapshot bucket counts for current queued PBS job wait time by queue from the latest successful cached collection.",
			[]string{"queue", "le"}, nil,
		),
		queueJobWaitCountDesc: prometheus.NewDesc(
			"pbs_queue_job_wait_seconds_count",
			"Current queued PBS jobs included in wait-time buckets by queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		queueJobWaitSumDesc: prometheus.NewDesc(
			"pbs_queue_job_wait_seconds_sum",
			"Sum of current queued PBS job wait times by queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		queueOldestJobWaitDesc: prometheus.NewDesc(
			"pbs_queue_oldest_job_wait_seconds",
			"Oldest current queued PBS job wait time by queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		queueEnabledDesc: prometheus.NewDesc(
			"pbs_queue_enabled",
			"Whether a PBS queue is enabled.",
			[]string{"queue"}, nil,
		),
		queueStartedDesc: prometheus.NewDesc(
			"pbs_queue_started",
			"Whether a PBS queue is started.",
			[]string{"queue"}, nil,
		),
		queueWalltimeDesc: prometheus.NewDesc(
			"pbs_queue_walltime_seconds",
			"Configured walltime limit for a PBS queue from the latest successful cached collection.",
			[]string{"queue"}, nil,
		),
		serverActiveDesc: prometheus.NewDesc(
			"pbs_server_active",
			"Whether the PBS server reports itself as Active.",
			nil, nil,
		),
		serverSchedulingEnabledDesc: prometheus.NewDesc(
			"pbs_server_scheduling_enabled",
			"Whether PBS server scheduling is enabled.",
			nil, nil,
		),
		serverJobsDesc: prometheus.NewDesc(
			"pbs_server_jobs",
			"PBS server jobs by state from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		serverAssignedCPUsDesc: prometheus.NewDesc(
			"pbs_server_assigned_cpu_cores",
			"Assigned PBS server CPU cores from the latest successful cached collection.",
			nil, nil,
		),
		serverAssignedMemoryDesc: prometheus.NewDesc(
			"pbs_server_assigned_memory_bytes",
			"Assigned PBS server memory bytes from the latest successful cached collection.",
			nil, nil,
		),
		serverAssignedNodesDesc: prometheus.NewDesc(
			"pbs_server_assigned_nodes",
			"Assigned PBS server node count from the latest successful cached collection.",
			nil, nil,
		),
		serverLicensesDesc: prometheus.NewDesc(
			"pbs_server_licenses",
			"PBS server licenses by state from the latest successful cached collection.",
			[]string{"state"}, nil,
		),
		serverMaxArraySizeDesc: prometheus.NewDesc(
			"pbs_server_max_array_size",
			"Maximum PBS array size from the latest successful cached collection.",
			nil, nil,
		),
		serverJobHistoryEnabledDesc: prometheus.NewDesc(
			"pbs_server_job_history_enabled",
			"Whether PBS job history is enabled.",
			nil, nil,
		),
		serverJobHistoryDurationDesc: prometheus.NewDesc(
			"pbs_server_job_history_duration_seconds",
			"PBS job history duration in seconds from the latest successful cached collection.",
			nil, nil,
		),
	}
}
