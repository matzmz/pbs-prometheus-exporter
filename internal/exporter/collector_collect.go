package exporter

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"pbs-exporter/internal/buildinfo"
	"pbs-exporter/internal/pbs"
)

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors() {
		ch <- desc
	}
}

func (c *Collector) descriptors() []*prometheus.Desc {
	descs := []*prometheus.Desc{
		c.exporterBuildInfoDesc,
		c.exporterUpDesc,
		c.collectErrorsTotalDesc,
		c.lastCollectDurationDesc,
		c.lastCollectTimestampDesc,
		c.lastCollectSuccessTimestampDesc,
		c.snapshotTimestampDesc,
		c.serverVersionInfoDesc,
		c.jobsDesc,
		c.jobsRunningByUserDesc,
		c.jobsQueuedByUserDesc,
		c.jobsRunningByQueueDesc,
		c.jobsTotalByQueueDesc,
		c.nodesDesc,
		c.nodeStateDesc,
		c.nodeJobsDesc,
		c.nodeCPUDesc,
		c.nodeGPUDesc,
		c.nodeMemoryDesc,
		c.nodeCPUUtilizationDesc,
		c.nodeGPUUtilizationDesc,
		c.nodeMemoryUtilizationDesc,
		c.clusterCPUDesc,
		c.clusterGPUDesc,
		c.clusterMemoryDesc,
		c.queueJobsDesc,
		c.queueJobWaitBucketDesc,
		c.queueJobWaitCountDesc,
		c.queueJobWaitSumDesc,
		c.queueOldestJobWaitDesc,
		c.queueEnabledDesc,
		c.queueStartedDesc,
		c.queueWalltimeDesc,
		c.serverActiveDesc,
		c.serverSchedulingEnabledDesc,
		c.serverJobsDesc,
		c.serverAssignedCPUsDesc,
		c.serverAssignedMemoryDesc,
		c.serverAssignedNodesDesc,
		c.serverLicensesDesc,
		c.serverMaxArraySizeDesc,
		c.serverJobHistoryEnabledDesc,
		c.serverJobHistoryDurationDesc,
	}
	descs = append(descs, c.jobInspection.descriptors()...)
	return append(descs, c.jobSampleHistograms.descriptors()...)
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	status := c.store.Status()
	ch <- prometheus.MustNewConstMetric(
		c.exporterBuildInfoDesc,
		prometheus.GaugeValue,
		1,
		buildinfo.Short(),
		buildinfo.RevisionValue(),
		buildinfo.BranchValue(),
	)
	ch <- prometheus.MustNewConstMetric(c.exporterUpDesc, prometheus.GaugeValue, boolFloat(status.Up))
	ch <- prometheus.MustNewConstMetric(c.collectErrorsTotalDesc, prometheus.CounterValue, float64(status.CollectErrorsTotal))
	ch <- prometheus.MustNewConstMetric(c.lastCollectDurationDesc, prometheus.GaugeValue, status.LastCollectDuration.Seconds())
	ch <- prometheus.MustNewConstMetric(c.lastCollectTimestampDesc, prometheus.GaugeValue, unixOrZero(status.LastCollectTimestamp))
	ch <- prometheus.MustNewConstMetric(c.lastCollectSuccessTimestampDesc, prometheus.GaugeValue, unixOrZero(status.LastCollectSuccessTimestamp))
	ch <- prometheus.MustNewConstMetric(c.snapshotTimestampDesc, prometheus.GaugeValue, unixOrZero(status.SnapshotTimestamp))
	c.jobInspection.collectStatus(ch, status)
	c.jobSampleHistograms.collect(ch, c.store.JobSampleHistograms())

	snapshot := c.store.Snapshot()
	if snapshot == nil {
		return
	}

	if snapshot.Version != "" {
		ch <- prometheus.MustNewConstMetric(c.serverVersionInfoDesc, prometheus.GaugeValue, 1, snapshot.Version)
	}

	c.collectJobs(ch, snapshot.Jobs)
	c.collectNodes(ch, snapshot.Nodes)
	c.collectQueues(ch, snapshot)
	c.collectServer(ch, snapshot.Server)
	if !c.includeJobInspectionMetrics {
		return
	}
	c.collectJobInspection(ch, snapshot.JobInspection)
}

func (c *Collector) collectJobs(ch chan<- prometheus.Metric, jobs *pbs.JobData) {
	if jobs == nil {
		return
	}

	for status, count := range jobs.StatusCount {
		ch <- prometheus.MustNewConstMetric(c.jobsDesc, prometheus.GaugeValue, float64(count), status)
	}
	for queue, count := range jobs.QueueJobCount {
		ch <- prometheus.MustNewConstMetric(c.jobsRunningByQueueDesc, prometheus.GaugeValue, float64(count), queue)
	}
	for queue, count := range jobs.QueueTotalCount {
		ch <- prometheus.MustNewConstMetric(c.jobsTotalByQueueDesc, prometheus.GaugeValue, float64(count), queue)
	}
	if !c.includeUserMetrics {
		return
	}
	for user, count := range jobs.UserJobCount {
		ch <- prometheus.MustNewConstMetric(c.jobsRunningByUserDesc, prometheus.GaugeValue, float64(count), user)
	}
	for user, count := range jobs.QueuedJobsByUser {
		ch <- prometheus.MustNewConstMetric(c.jobsQueuedByUserDesc, prometheus.GaugeValue, float64(count), user)
	}
}

func (c *Collector) collectNodes(ch chan<- prometheus.Metric, nodes *pbs.NodeData) {
	if nodes == nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.nodesDesc, prometheus.GaugeValue, float64(nodes.CountFree), "free")
	ch <- prometheus.MustNewConstMetric(c.nodesDesc, prometheus.GaugeValue, float64(nodes.CountBusy), "job-busy")
	ch <- prometheus.MustNewConstMetric(c.nodesDesc, prometheus.GaugeValue, float64(nodes.CountOffline), "offline")
	ch <- prometheus.MustNewConstMetric(c.nodesDesc, prometheus.GaugeValue, float64(nodes.CountDown), "down")

	var totalCPUs, availableCPUs, usedCPUs int
	var totalGPUs, availableGPUs, usedGPUs int
	var totalMemory, availableMemory, usedMemory float64

	for nodeName, nodeInfo := range nodes.Nodes {
		ch <- prometheus.MustNewConstMetric(c.nodeStateDesc, prometheus.GaugeValue, 1, nodeName, nodeInfo.State)
		ch <- prometheus.MustNewConstMetric(c.nodeJobsDesc, prometheus.GaugeValue, float64(nodeInfo.Jobs), nodeName)

		usedCPU := nodeInfo.CPUsTotal - nodeInfo.CPUsAvailable
		ch <- prometheus.MustNewConstMetric(c.nodeCPUDesc, prometheus.GaugeValue, float64(nodeInfo.CPUsAvailable), nodeName, "available")
		ch <- prometheus.MustNewConstMetric(c.nodeCPUDesc, prometheus.GaugeValue, float64(usedCPU), nodeName, "used")
		ch <- prometheus.MustNewConstMetric(c.nodeCPUDesc, prometheus.GaugeValue, float64(nodeInfo.CPUsTotal), nodeName, "total")

		usedGPU := nodeInfo.GPUsTotal - nodeInfo.GPUsAvailable
		ch <- prometheus.MustNewConstMetric(c.nodeGPUDesc, prometheus.GaugeValue, float64(nodeInfo.GPUsAvailable), nodeName, "available")
		ch <- prometheus.MustNewConstMetric(c.nodeGPUDesc, prometheus.GaugeValue, float64(usedGPU), nodeName, "used")
		ch <- prometheus.MustNewConstMetric(c.nodeGPUDesc, prometheus.GaugeValue, float64(nodeInfo.GPUsTotal), nodeName, "total")

		usedMemoryOnNode := nodeInfo.MemoryTotal - nodeInfo.MemoryAvailable
		ch <- prometheus.MustNewConstMetric(c.nodeMemoryDesc, prometheus.GaugeValue, nodeInfo.MemoryAvailable, nodeName, "available")
		ch <- prometheus.MustNewConstMetric(c.nodeMemoryDesc, prometheus.GaugeValue, usedMemoryOnNode, nodeName, "used")
		ch <- prometheus.MustNewConstMetric(c.nodeMemoryDesc, prometheus.GaugeValue, nodeInfo.MemoryTotal, nodeName, "total")

		if nodeInfo.CPUsTotal > 0 {
			ch <- prometheus.MustNewConstMetric(c.nodeCPUUtilizationDesc, prometheus.GaugeValue, float64(usedCPU)/float64(nodeInfo.CPUsTotal), nodeName)
		}
		if nodeInfo.GPUsTotal > 0 {
			ch <- prometheus.MustNewConstMetric(c.nodeGPUUtilizationDesc, prometheus.GaugeValue, float64(usedGPU)/float64(nodeInfo.GPUsTotal), nodeName)
		}
		if nodeInfo.MemoryTotal > 0 {
			ch <- prometheus.MustNewConstMetric(c.nodeMemoryUtilizationDesc, prometheus.GaugeValue, usedMemoryOnNode/nodeInfo.MemoryTotal, nodeName)
		}

		totalCPUs += nodeInfo.CPUsTotal
		availableCPUs += nodeInfo.CPUsAvailable
		usedCPUs += usedCPU
		totalGPUs += nodeInfo.GPUsTotal
		availableGPUs += nodeInfo.GPUsAvailable
		usedGPUs += usedGPU
		totalMemory += nodeInfo.MemoryTotal
		availableMemory += nodeInfo.MemoryAvailable
		usedMemory += usedMemoryOnNode
	}

	ch <- prometheus.MustNewConstMetric(c.clusterCPUDesc, prometheus.GaugeValue, float64(availableCPUs), "available")
	ch <- prometheus.MustNewConstMetric(c.clusterCPUDesc, prometheus.GaugeValue, float64(usedCPUs), "used")
	ch <- prometheus.MustNewConstMetric(c.clusterCPUDesc, prometheus.GaugeValue, float64(totalCPUs), "total")
	ch <- prometheus.MustNewConstMetric(c.clusterGPUDesc, prometheus.GaugeValue, float64(availableGPUs), "available")
	ch <- prometheus.MustNewConstMetric(c.clusterGPUDesc, prometheus.GaugeValue, float64(usedGPUs), "used")
	ch <- prometheus.MustNewConstMetric(c.clusterGPUDesc, prometheus.GaugeValue, float64(totalGPUs), "total")
	ch <- prometheus.MustNewConstMetric(c.clusterMemoryDesc, prometheus.GaugeValue, availableMemory, "available")
	ch <- prometheus.MustNewConstMetric(c.clusterMemoryDesc, prometheus.GaugeValue, usedMemory, "used")
	ch <- prometheus.MustNewConstMetric(c.clusterMemoryDesc, prometheus.GaugeValue, totalMemory, "total")
}

func (c *Collector) collectQueues(ch chan<- prometheus.Metric, snapshot *pbs.Snapshot) {
	if snapshot.Queues == nil {
		return
	}

	for queueName, queueInfo := range snapshot.Queues.Queues {
		ch <- prometheus.MustNewConstMetric(c.queueJobsDesc, prometheus.GaugeValue, float64(queueInfo.Running), queueName, "running")
		ch <- prometheus.MustNewConstMetric(c.queueJobsDesc, prometheus.GaugeValue, float64(queueInfo.Queued), queueName, "queued")
		total := float64(queueInfo.Running + queueInfo.Queued)
		if snapshot.Jobs != nil {
			if totalCount, ok := snapshot.Jobs.QueueTotalCount[queueName]; ok {
				total = float64(totalCount)
			}
		}
		ch <- prometheus.MustNewConstMetric(c.queueJobsDesc, prometheus.GaugeValue, total, queueName, "total")
		ch <- prometheus.MustNewConstMetric(c.queueEnabledDesc, prometheus.GaugeValue, boolFloat(queueInfo.Enabled), queueName)
		ch <- prometheus.MustNewConstMetric(c.queueStartedDesc, prometheus.GaugeValue, boolFloat(queueInfo.Started), queueName)
		ch <- prometheus.MustNewConstMetric(c.queueWalltimeDesc, prometheus.GaugeValue, float64(queueInfo.Walltime), queueName)
		c.collectQueueWait(ch, queueName, snapshot.QueueWaits)
	}
}

func (c *Collector) collectQueueWait(ch chan<- prometheus.Metric, queueName string, queueWaits *pbs.QueueWaitData) {
	var info pbs.QueueWaitInfo
	if queueWaits != nil {
		info = queueWaits.Queues[queueName]
	}

	for _, bucket := range pbs.QueueWaitBuckets() {
		ch <- prometheus.MustNewConstMetric(
			c.queueJobWaitBucketDesc,
			prometheus.GaugeValue,
			float64(info.Buckets[bucket]),
			queueName,
			pbs.QueueWaitBucketLabel(bucket),
		)
	}
	ch <- prometheus.MustNewConstMetric(c.queueJobWaitCountDesc, prometheus.GaugeValue, float64(info.Count), queueName)
	ch <- prometheus.MustNewConstMetric(c.queueJobWaitSumDesc, prometheus.GaugeValue, info.Sum, queueName)
	ch <- prometheus.MustNewConstMetric(c.queueOldestJobWaitDesc, prometheus.GaugeValue, info.Oldest, queueName)
}

func (c *Collector) collectServer(ch chan<- prometheus.Metric, server *pbs.ServerData) {
	if server == nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(c.serverActiveDesc, prometheus.GaugeValue, boolFloat(server.State == "Active"))
	ch <- prometheus.MustNewConstMetric(c.serverSchedulingEnabledDesc, prometheus.GaugeValue, boolFloat(server.Scheduling))
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.TotalJobs), "total")
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.JobsRunning), "running")
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.JobsQueued), "queued")
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.JobsHeld), "held")
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.JobsWaiting), "waiting")
	ch <- prometheus.MustNewConstMetric(c.serverJobsDesc, prometheus.GaugeValue, float64(server.JobsExiting), "exiting")
	ch <- prometheus.MustNewConstMetric(c.serverAssignedCPUsDesc, prometheus.GaugeValue, float64(server.ResourcesNcpus))
	ch <- prometheus.MustNewConstMetric(c.serverAssignedMemoryDesc, prometheus.GaugeValue, server.ResourcesMemBytes)
	ch <- prometheus.MustNewConstMetric(c.serverAssignedNodesDesc, prometheus.GaugeValue, float64(server.ResourcesNodect))
	ch <- prometheus.MustNewConstMetric(c.serverLicensesDesc, prometheus.GaugeValue, float64(server.LicensesAvailable), "available")
	ch <- prometheus.MustNewConstMetric(c.serverLicensesDesc, prometheus.GaugeValue, float64(server.LicensesUsed), "used")
	ch <- prometheus.MustNewConstMetric(c.serverMaxArraySizeDesc, prometheus.GaugeValue, float64(server.MaxArraySize))
	ch <- prometheus.MustNewConstMetric(c.serverJobHistoryEnabledDesc, prometheus.GaugeValue, boolFloat(server.JobHistoryEnabled))
	ch <- prometheus.MustNewConstMetric(c.serverJobHistoryDurationDesc, prometheus.GaugeValue, float64(server.JobHistoryDuration))
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func unixOrZero(timestamp time.Time) float64 {
	if timestamp.IsZero() {
		return 0
	}
	return float64(timestamp.Unix())
}
