package pbs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type jobsJSONPayload struct {
	Jobs map[string]jobsJSONRecord `json:"Jobs"`
}

type jobsJSONRecord struct {
	Queue         string `json:"queue"`
	JobState      string `json:"job_state"`
	JobOwner      string `json:"Job_Owner"`
	EffectiveUser string `json:"euser"`
	Qtime         string `json:"qtime"`
}

type queuesJSONPayload struct {
	Queue  map[string]queueJSONRecord `json:"Queue"`
	Queues map[string]queueJSONRecord `json:"Queues"`
}

type queueJSONRecord struct {
	StateCount        string     `json:"state_count"`
	Enabled           jsonScalar `json:"enabled"`
	Started           jsonScalar `json:"started"`
	MaxWalltime       jsonScalar `json:"resources_max.walltime"`
	DefaultWalltime   jsonScalar `json:"resources_default.walltime"`
	AvailableWalltime jsonScalar `json:"resources_available.walltime"`
}

type serverJSONPayload struct {
	Server  map[string]serverJSONRecord `json:"Server"`
	Servers map[string]serverJSONRecord `json:"Servers"`
}

type serverJSONRecord struct {
	ServerState        string     `json:"server_state"`
	Scheduling         jsonScalar `json:"scheduling"`
	TotalJobs          jsonScalar `json:"total_jobs"`
	StateCount         string     `json:"state_count"`
	AssignedCPUs       jsonScalar `json:"resources_assigned.ncpus"`
	AssignedMemory     jsonScalar `json:"resources_assigned.mem"`
	AssignedNodes      jsonScalar `json:"resources_assigned.nodect"`
	LicenseCount       string     `json:"license_count"`
	MaxArraySize       jsonScalar `json:"max_array_size"`
	JobHistoryEnabled  jsonScalar `json:"job_history_enable"`
	JobHistoryDuration jsonScalar `json:"job_history_duration"`
}

type nodesJSONPayload struct {
	Nodes    map[string]nodeJSONRecord `json:"nodes"`
	NodesAlt map[string]nodeJSONRecord `json:"Nodes"`
}

type nodeJSONRecord struct {
	State          string     `json:"state"`
	StateSummary   string     `json:"State"`
	Jobs           jsonScalar `json:"jobs"`
	JobsList       []string   `json:"-"`
	JobCount       jsonScalar `json:"njobs"`
	TotalJobs      jsonScalar `json:"Total Jobs"`
	RunningJobs    jsonScalar `json:"Running Jobs"`
	Memory         jsonScalar `json:"mem"`
	MemoryFraction jsonScalar `json:"mem f/t"`
	CPUs           jsonScalar `json:"ncpus"`
	CPUsFraction   jsonScalar `json:"ncpus f/t"`
	GPUs           jsonScalar `json:"ngpus"`
	GPUsFraction   jsonScalar `json:"ngpus f/t"`
}

func (c *Client) collectJSON(ctx context.Context, target any, name string, args ...string) error {
	output, err := c.run(ctx, name, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("parse %s %s JSON output: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func parseJobsJSON(payload *jobsJSONPayload) *JobData {
	data := &JobData{
		UserJobCount:     make(map[string]int),
		QueuedJobsByUser: make(map[string]int),
		QueueJobCount:    make(map[string]int),
		QueueTotalCount:  make(map[string]int),
		StatusCount:      make(map[string]int),
	}
	if payload == nil {
		return data
	}

	for _, record := range payload.Jobs {
		statusLabel := mapStatusLabel(record.JobState)
		data.StatusCount[statusLabel]++
		data.QueueTotalCount[record.Queue]++

		user := jobUser(record)
		switch strings.ToUpper(record.JobState) {
		case "R":
			if user != "" {
				data.UserJobCount[user]++
			}
			data.QueueJobCount[record.Queue]++
		case "Q":
			if user != "" {
				data.QueuedJobsByUser[user]++
			}
		}
	}

	return data
}

func parseQueueWaitJSON(payload *jobsJSONPayload, collectedAt time.Time) *QueueWaitData {
	data := &QueueWaitData{Queues: make(map[string]QueueWaitInfo)}
	if payload == nil {
		return data
	}

	for _, record := range payload.Jobs {
		if !strings.EqualFold(record.JobState, "Q") || record.Queue == "" || record.Qtime == "" {
			continue
		}

		queuedAt, ok := parseQtime(record.Qtime)
		if !ok {
			continue
		}

		waitSeconds := collectedAt.Sub(queuedAt).Seconds()
		if waitSeconds < 0 {
			waitSeconds = 0
		}
		addQueueWait(data, record.Queue, waitSeconds)
	}

	return data
}

func parseQueuesJSON(payload *queuesJSONPayload) *QueueData {
	data := &QueueData{Queues: make(map[string]QueueInfo)}
	if payload == nil {
		return data
	}

	for _, queueName := range sortedQueueNames(payload.records()) {
		record := payload.records()[queueName]
		stateCounts := parseNamedCounts(record.StateCount)
		data.Queues[queueName] = QueueInfo{
			Running:  stateCounts["Running"],
			Queued:   stateCounts["Queued"],
			Enabled:  record.Enabled.boolValue(),
			Started:  record.Started.boolValue(),
			Walltime: firstDurationSeconds(record.MaxWalltime, record.DefaultWalltime, record.AvailableWalltime),
		}
	}

	return data
}

func parseServerJSON(payload *serverJSONPayload) *ServerData {
	if payload == nil {
		return &ServerData{}
	}

	_, record, ok := payload.firstRecord()
	if !ok {
		return &ServerData{}
	}

	stateCounts := parseNamedCounts(record.StateCount)
	licenseCounts := parseNamedCounts(record.LicenseCount)

	return &ServerData{
		State:              record.ServerState,
		Scheduling:         record.Scheduling.boolValue(),
		TotalJobs:          record.TotalJobs.intValue(),
		JobsRunning:        stateCounts["Running"],
		JobsQueued:         stateCounts["Queued"],
		JobsHeld:           stateCounts["Held"],
		JobsWaiting:        stateCounts["Waiting"],
		JobsExiting:        stateCounts["Exiting"],
		ResourcesNcpus:     record.AssignedCPUs.intValue(),
		ResourcesMemBytes:  record.AssignedMemory.bytesValue(),
		ResourcesNodect:    record.AssignedNodes.intValue(),
		LicensesAvailable:  licenseCounts["Avail_Global"],
		LicensesUsed:       licenseCounts["Used"],
		MaxArraySize:       record.MaxArraySize.intValue(),
		JobHistoryEnabled:  record.JobHistoryEnabled.boolValue(),
		JobHistoryDuration: record.JobHistoryDuration.durationSecondsValue(),
	}
}

func parseNodesJSON(payload *nodesJSONPayload) *NodeData {
	data := &NodeData{Nodes: make(map[string]NodeInfo)}
	if payload == nil {
		return data
	}

	for _, nodeName := range sortedNodeNames(payload.records()) {
		record := payload.records()[nodeName]
		state := firstText(record.State, record.StateSummary)
		if state == "state-unknown" {
			continue
		}

		normalizedState := normalizeNodeState(state)
		switch normalizedState {
		case "free":
			data.CountFree++
		case "job-busy":
			data.CountBusy++
		case "offline":
			data.CountOffline++
		default:
			data.CountDown++
		}

		availableMemory, totalMemory := parseFractionalMemory(firstScalarText(record.Memory, record.MemoryFraction))
		availableCPUs, totalCPUs := parseFraction(firstScalarText(record.CPUs, record.CPUsFraction))
		availableGPUs, totalGPUs := parseFraction(firstScalarText(record.GPUs, record.GPUsFraction))

		jobCount := firstPositiveInt(record.Jobs, record.JobCount, record.TotalJobs, record.RunningJobs)
		if jobCount == 0 {
			jobCount = len(record.JobsList)
		}

		data.Nodes[nodeName] = NodeInfo{
			State:           normalizedState,
			Jobs:            jobCount,
			CPUsAvailable:   availableCPUs,
			CPUsTotal:       totalCPUs,
			GPUsAvailable:   availableGPUs,
			GPUsTotal:       totalGPUs,
			MemoryAvailable: availableMemory,
			MemoryTotal:     totalMemory,
		}
	}

	return data
}

func (r *nodeJSONRecord) UnmarshalJSON(data []byte) error {
	type alias nodeJSONRecord
	var raw struct {
		alias
		Jobs []string `json:"jobs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = nodeJSONRecord(raw.alias)
	r.JobsList = raw.Jobs
	return nil
}

func summarizeQueues(data *QueueData) QueueSummary {
	summary := QueueSummary{}
	if data == nil {
		return summary
	}

	for _, queueInfo := range data.Queues {
		summary.Running += queueInfo.Running
		summary.Queued += queueInfo.Queued
	}

	return summary
}

func (p *queuesJSONPayload) records() map[string]queueJSONRecord {
	if len(p.Queue) > 0 {
		return p.Queue
	}
	return p.Queues
}

func (p *serverJSONPayload) firstRecord() (string, serverJSONRecord, bool) {
	records := p.Server
	if len(records) == 0 {
		records = p.Servers
	}
	if len(records) == 0 {
		return "", serverJSONRecord{}, false
	}

	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)

	name := names[0]
	return name, records[name], true
}

func (p *nodesJSONPayload) records() map[string]nodeJSONRecord {
	if len(p.Nodes) > 0 {
		return p.Nodes
	}
	return p.NodesAlt
}

func jobUser(record jobsJSONRecord) string {
	if record.EffectiveUser != "" {
		return record.EffectiveUser
	}
	if record.JobOwner == "" {
		return ""
	}

	parts := strings.SplitN(record.JobOwner, "@", 2)
	return parts[0]
}

func parseNamedCounts(raw string) map[string]int {
	counts := make(map[string]int)
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == ','
	}) {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		parsedValue, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		counts[strings.TrimSpace(key)] = parsedValue
	}
	return counts
}

func firstDurationSeconds(values ...jsonScalar) int {
	for _, value := range values {
		if !value.set {
			continue
		}
		seconds := value.durationSecondsValue()
		if seconds > 0 || value.textValue() == "0" || value.textValue() == "00:00" || value.textValue() == "00:00:00" {
			return seconds
		}
	}
	return 0
}

func parseFractionalMemory(raw string) (float64, float64) {
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseMemoryToBytes(parts[0]), parseMemoryToBytes(parts[1])
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstScalarText(values ...jsonScalar) string {
	for _, value := range values {
		if value.set && value.textValue() != "" {
			return value.textValue()
		}
	}
	return ""
}

func firstPositiveInt(values ...jsonScalar) int {
	for _, value := range values {
		if !value.set {
			continue
		}
		parsed := value.intValue()
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}

func sortedQueueNames(records map[string]queueJSONRecord) []string {
	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedNodeNames(records map[string]nodeJSONRecord) []string {
	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
