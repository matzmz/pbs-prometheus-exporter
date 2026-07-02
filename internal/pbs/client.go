package pbs

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	binaryDir            string
	timeout              time.Duration
	includeJobInspection bool
	logger               *slog.Logger
}

type Snapshot struct {
	CollectedAt   time.Time
	Version       string
	Jobs          *JobData
	JobInspection *JobInspectionData
	QueueWaits    *QueueWaitData
	Nodes         *NodeData
	Queues        *QueueData
	QueueSummary  QueueSummary
	Server        *ServerData
}

type QueueSummary struct {
	Running int
	Queued  int
}

type ClientOptions struct {
	IncludeJobInspection bool
}

type CollectionResult struct {
	Snapshot               *Snapshot
	JobInspectionAttempted bool
	JobInspectionError     error
}

type snapshotOutputs struct {
	jobs      string
	jobsFull  string
	nodes     string
	queues    string
	server    string
}

func NewClient(binaryDir string, timeout time.Duration, options ClientOptions, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		binaryDir:            binaryDir,
		timeout:              timeout,
		includeJobInspection: options.IncludeJobInspection,
		logger:               logger,
	}
}

func (c *Client) Collect(ctx context.Context) (*CollectionResult, error) {
	outputs, err := c.collectSnapshotOutputs(ctx)
	if err != nil {
		return nil, err
	}

	version := c.getVersion(ctx)
	queueRunning, queueQueued := c.ParseQstatQSummary(outputs.queues)
	collectedAt := time.Now().UTC()

	snapshot := &Snapshot{
		CollectedAt: collectedAt,
		Version:     version,
		Jobs:        c.ParseQstatOutput(outputs.jobs),
		QueueWaits:  c.ParseQstatFullQueueWait(outputs.jobsFull, collectedAt),
		Nodes:       c.ParsePbsnodesOutput(outputs.nodes),
		Queues:      c.ParseQstatQFull(outputs.queues),
		QueueSummary: QueueSummary{
			Running: queueRunning,
			Queued:  queueQueued,
		},
		Server: c.ParseQstatBf(outputs.server),
	}

	result := &CollectionResult{Snapshot: snapshot}
	if !c.includeJobInspection {
		return result, nil
	}

	result.JobInspectionAttempted = true
	jobInspectionOutput, err := c.run(ctx, "qstat", "-F", "json", "-f")
	if err != nil {
		result.JobInspectionError = err
		return result, nil
	}

	jobInspection, err := c.ParseJobInspectionOutput(jobInspectionOutput, collectedAt)
	if err != nil {
		result.JobInspectionError = err
		return result, nil
	}

	snapshot.JobInspection = jobInspection

	return result, nil
}

func (c *Client) collectSnapshotOutputs(ctx context.Context) (*snapshotOutputs, error) {
	jobsOutput, err := c.run(ctx, "qstat", "-t")
	if err != nil {
		return nil, err
	}

	jobsFullOutput, err := c.run(ctx, "qstat", "-f")
	if err != nil {
		return nil, err
	}

	nodesOutput, err := c.run(ctx, "pbsnodes", "-aSj")
	if err != nil {
		return nil, err
	}

	queuesOutput, err := c.run(ctx, "qstat", "-q")
	if err != nil {
		return nil, err
	}

	serverOutput, err := c.run(ctx, "qstat", "-Bf")
	if err != nil {
		return nil, err
	}

	return &snapshotOutputs{
		jobs:     jobsOutput,
		jobsFull: jobsFullOutput,
		nodes:    nodesOutput,
		queues:   queuesOutput,
		server:   serverOutput,
	}, nil
}

func (c *Client) run(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx := ctx
	cancel := func() {}
	if c.timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(commandCtx, c.commandPath(name), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Error("PBS command failed", "command", name, "args", args, "err", err, "output", strings.TrimSpace(string(output)))
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return string(output), nil
}

func (c *Client) getVersion(ctx context.Context) string {
	output, err := c.run(ctx, "qstat", "--version")
	if err != nil {
		output, err = c.run(ctx, "pbsnodes", "--version")
		if err != nil {
			return "unknown"
		}
	}

	re := regexp.MustCompile(`\d{4}\.\d+\.\d+`)
	version := re.FindString(output)
	if version == "" {
		return "unknown"
	}
	return version
}

func (c *Client) commandPath(name string) string {
	if c.binaryDir == "" {
		return name
	}
	return filepath.Join(c.binaryDir, name)
}

type JobData struct {
	UserJobCount     map[string]int
	QueuedJobsByUser map[string]int
	QueueJobCount    map[string]int
	QueueTotalCount  map[string]int
	StatusCount      map[string]int
}

var queueWaitInfBucket = math.Inf(1)

var queueWaitBuckets = []float64{
	300,
	1800,
	3600,
	7200,
	21600,
	43200,
	86400,
	172800,
	432000,
	queueWaitInfBucket,
}

func QueueWaitBuckets() []float64 {
	buckets := make([]float64, len(queueWaitBuckets))
	copy(buckets, queueWaitBuckets)
	return buckets
}

func QueueWaitBucketLabel(bucket float64) string {
	if math.IsInf(bucket, 1) {
		return "+Inf"
	}
	return strconv.FormatFloat(bucket, 'f', -1, 64)
}

type QueueWaitData struct {
	Queues map[string]QueueWaitInfo
}

type QueueWaitInfo struct {
	Buckets map[float64]int
	Count   int
	Sum     float64
	Oldest  float64
}

type NodeData struct {
	Nodes        map[string]NodeInfo
	CountFree    int
	CountBusy    int
	CountOffline int
	CountDown    int
}

type NodeInfo struct {
	State           string
	Jobs            int
	CPUsAvailable   int
	CPUsTotal       int
	GPUsAvailable   int
	GPUsTotal       int
	MemoryAvailable float64
	MemoryTotal     float64
}

type QueueInfo struct {
	Running  int
	Queued   int
	Enabled  bool
	Started  bool
	Walltime int
}

type QueueData struct {
	Queues map[string]QueueInfo
}

type ServerData struct {
	State              string
	Scheduling         bool
	TotalJobs          int
	JobsRunning        int
	JobsQueued         int
	JobsHeld           int
	JobsWaiting        int
	JobsExiting        int
	ResourcesNcpus     int
	ResourcesMemBytes  float64
	ResourcesNodect    int
	LicensesAvailable  int
	LicensesUsed       int
	MaxArraySize       int
	JobHistoryEnabled  bool
	JobHistoryDuration int
}

func (c *Client) ParseQstatOutput(output string) *JobData {
	data := &JobData{
		UserJobCount:     make(map[string]int),
		QueuedJobsByUser: make(map[string]int),
		QueueJobCount:    make(map[string]int),
		QueueTotalCount:  make(map[string]int),
		StatusCount:      make(map[string]int),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	lineCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		if lineCount <= 2 || line == "" || strings.Contains(line, "----") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		user := fields[2]
		status := fields[4]
		queue := fields[5]
		statusLabel := mapStatusLabel(status)

		data.StatusCount[statusLabel]++
		data.QueueTotalCount[queue]++

		if status == "R" {
			data.UserJobCount[user]++
			data.QueueJobCount[queue]++
		}

		if status == "Q" {
			data.QueuedJobsByUser[user]++
		}
	}

	return data
}

func (c *Client) ParseQstatFullQueueWait(output string, collectedAt time.Time) *QueueWaitData {
	data := &QueueWaitData{Queues: make(map[string]QueueWaitInfo)}

	type jobRecord struct {
		state string
		queue string
		qtime string
	}

	var current *jobRecord
	flush := func() {
		if current == nil || !strings.EqualFold(current.state, "Q") || current.queue == "" || current.qtime == "" {
			return
		}

		queuedAt, ok := parseQtime(current.qtime)
		if !ok {
			return
		}

		wait := collectedAt.Sub(queuedAt).Seconds()
		if wait < 0 {
			wait = 0
		}
		addQueueWait(data, current.queue, wait)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Job Id:") {
			flush()
			current = &jobRecord{}
			continue
		}
		if current == nil {
			continue
		}

		key, value, ok := splitAttribute(line)
		if !ok {
			continue
		}
		switch key {
		case "job_state":
			current.state = value
		case "queue":
			current.queue = value
		case "qtime":
			current.qtime = value
		}
	}
	flush()

	return data
}

func splitAttribute(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func parseQtime(value string) (time.Time, bool) {
	localLayouts := []string{
		"Mon Jan _2 15:04:05 2006",
	}
	for _, layout := range localLayouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed.UTC(), true
		}
	}

	zoneLayouts := []string{
		"Mon Jan _2 15:04:05 MST 2006",
		time.RFC1123,
		time.RFC1123Z,
	}
	for _, layout := range zoneLayouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func addQueueWait(data *QueueWaitData, queue string, wait float64) {
	info := data.Queues[queue]
	if info.Buckets == nil {
		info.Buckets = make(map[float64]int, len(queueWaitBuckets))
		for _, bucket := range queueWaitBuckets {
			info.Buckets[bucket] = 0
		}
	}

	info.Count++
	info.Sum += wait
	if wait > info.Oldest {
		info.Oldest = wait
	}
	for _, bucket := range queueWaitBuckets {
		if wait <= bucket {
			info.Buckets[bucket]++
		}
	}
	data.Queues[queue] = info
}

func (c *Client) ParseQstatQSummary(output string) (totalRunning int, totalQueued int) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	seenSeparator := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "---") {
			seenSeparator = true
			continue
		}
		if seenSeparator {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.Atoi(fields[0]); err == nil {
					totalRunning = v
				}
				if v, err := strconv.Atoi(fields[1]); err == nil {
					totalQueued = v
				}
				return
			}
		}

		fields := strings.Fields(line)
		if len(fields) < 3 || strings.EqualFold(fields[0], "Queue") || strings.EqualFold(fields[0], "server:") {
			continue
		}
		var nums []int
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil {
				nums = append(nums, n)
			}
		}
		if len(nums) >= 2 {
			totalRunning += nums[len(nums)-2]
			totalQueued += nums[len(nums)-1]
		}
	}
	return
}

func (c *Client) ParseQstatQFull(output string) *QueueData {
	data := &QueueData{Queues: make(map[string]QueueInfo)}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "server:") || strings.HasPrefix(strings.ToLower(line), "queue ") || strings.HasPrefix(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		queueName := fields[0]
		walltimeStr := fields[2]
		stateStr := fields[len(fields)-1]
		if len(fields) >= 10 {
			stateStr = fields[len(fields)-2] + " " + fields[len(fields)-1]
		}

		var nums []int
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil {
				nums = append(nums, n)
			}
		}

		running := 0
		queued := 0
		if len(nums) >= 2 {
			running = nums[len(nums)-2]
			queued = nums[len(nums)-1]
		}

		data.Queues[queueName] = QueueInfo{
			Running:  running,
			Queued:   queued,
			Enabled:  strings.Contains(stateStr, "E"),
			Started:  strings.Contains(stateStr, "R"),
			Walltime: parseWalltimeToSeconds(walltimeStr),
		}
	}
	return data
}

func (c *Client) ParseQstatBf(output string) *ServerData {
	data := &ServerData{}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "server_state"):
			data.State = fieldValue(line)
		case strings.HasPrefix(line, "scheduling"):
			data.Scheduling = strings.EqualFold(fieldValue(line), "True")
		case strings.HasPrefix(line, "total_jobs"):
			data.TotalJobs, _ = strconv.Atoi(fieldValue(line))
		case strings.HasPrefix(line, "state_count"):
			parseStateCount(fieldValue(line), data)
		case strings.HasPrefix(line, "resources_assigned.ncpus"):
			data.ResourcesNcpus, _ = strconv.Atoi(fieldValue(line))
		case strings.HasPrefix(line, "resources_assigned.mem"):
			data.ResourcesMemBytes = parseMemoryToBytes(fieldValue(line))
		case strings.HasPrefix(line, "resources_assigned.nodect"):
			data.ResourcesNodect, _ = strconv.Atoi(fieldValue(line))
		case strings.HasPrefix(line, "license_count"):
			parseLicenseCount(fieldValue(line), data)
		case strings.HasPrefix(line, "max_array_size"):
			data.MaxArraySize, _ = strconv.Atoi(fieldValue(line))
		case strings.HasPrefix(line, "job_history_enable"):
			data.JobHistoryEnabled = strings.EqualFold(fieldValue(line), "True")
		case strings.HasPrefix(line, "job_history_duration"):
			data.JobHistoryDuration = parseWalltimeToSeconds(fieldValue(line))
		}
	}

	return data
}

func (c *Client) ParsePbsnodesOutput(output string) *NodeData {
	data := &NodeData{Nodes: make(map[string]NodeInfo)}

	scanner := bufio.NewScanner(strings.NewReader(output))
	lineCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		if lineCount <= 2 || line == "" || strings.Contains(line, "----") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		nodeName := fields[0]
		state := fields[1]
		njobs := fields[2]
		memField := fields[5]
		cpuField := fields[6]
		gpuField := fields[8]

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

		jobs, _ := strconv.Atoi(njobs)
		memParts := strings.Split(memField, "/")
		availableMem := float64(0)
		totalMem := float64(0)
		if len(memParts) == 2 {
			availableMem = parseMemoryToBytes(memParts[0])
			totalMem = parseMemoryToBytes(memParts[1])
		}

		freeCpus, totalCpus := parseFraction(cpuField)
		freeGpus, totalGpus := parseFraction(gpuField)

		data.Nodes[nodeName] = NodeInfo{
			State:           normalizedState,
			Jobs:            jobs,
			CPUsAvailable:   freeCpus,
			CPUsTotal:       totalCpus,
			GPUsAvailable:   freeGpus,
			GPUsTotal:       totalGpus,
			MemoryAvailable: availableMem,
			MemoryTotal:     totalMem,
		}
	}

	return data
}

func fieldValue(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func mapStatusLabel(status string) string {
	switch strings.ToUpper(status) {
	case "F":
		return "finished"
	case "H":
		return "held"
	case "R":
		return "running"
	case "Q":
		return "queued"
	case "E":
		return "exiting"
	case "B":
		return "array_job_running"
	default:
		return strings.ToLower(status)
	}
}

func normalizeNodeState(state string) string {
	if state == "<various>" {
		return "job-busy"
	}
	switch state {
	case "free", "job-busy", "offline", "down":
		return state
	default:
		return "down"
	}
}

func parseStateCount(s string, data *ServerData) {
	for _, part := range strings.Fields(s) {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		val, _ := strconv.Atoi(strings.TrimSpace(kv[1]))
		switch strings.TrimSpace(kv[0]) {
		case "Running":
			data.JobsRunning = val
		case "Queued":
			data.JobsQueued = val
		case "Held":
			data.JobsHeld = val
		case "Waiting":
			data.JobsWaiting = val
		case "Exiting":
			data.JobsExiting = val
		}
	}
}

func parseLicenseCount(s string, data *ServerData) {
	for _, part := range strings.Fields(s) {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		val, _ := strconv.Atoi(strings.TrimSpace(kv[1]))
		switch strings.TrimSpace(kv[0]) {
		case "Avail_Global":
			data.LicensesAvailable = val
		case "Used":
			data.LicensesUsed = val
		}
	}
}

func parseWalltimeToSeconds(s string) int {
	if s == "--" || s == "" {
		return 0
	}

	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		hours, _ := strconv.Atoi(parts[0])
		mins, _ := strconv.Atoi(parts[1])
		secs, _ := strconv.Atoi(parts[2])
		return hours*3600 + mins*60 + secs
	}
	if len(parts) == 2 {
		mins, _ := strconv.Atoi(parts[0])
		secs, _ := strconv.Atoi(parts[1])
		return mins*60 + secs
	}
	return 0
}

func parseMemoryToBytes(memStr string) float64 {
	memStr = strings.ToLower(strings.TrimSpace(memStr))
	if memStr == "--" || memStr == "" {
		return 0
	}

	type unit struct {
		suffix string
		factor float64
	}

	units := []unit{
		{"tb", 1024 * 1024 * 1024 * 1024},
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}

	for _, candidate := range units {
		if strings.HasSuffix(memStr, candidate.suffix) {
			numStr := strings.TrimSuffix(memStr, candidate.suffix)
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return val * candidate.factor
			}
			return 0
		}
	}

	if val, err := strconv.ParseFloat(memStr, 64); err == nil {
		return val
	}
	return 0
}

func parseFraction(fracStr string) (free, total int) {
	if fracStr == "--" || fracStr == "" {
		return 0, 0
	}
	parts := strings.Split(fracStr, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	free, _ = strconv.Atoi(parts[0])
	total, _ = strconv.Atoi(parts[1])
	return free, total
}
