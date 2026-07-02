package pbs

import (
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

type snapshotJSONPayloads struct {
	jobs   *jobsJSONPayload
	nodes  *nodesJSONPayload
	queues *queuesJSONPayload
	server *serverJSONPayload
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
	payloads, err := c.collectSnapshotPayloads(ctx)
	if err != nil {
		return nil, err
	}

	version := c.getVersion(ctx)
	collectedAt := time.Now().UTC()
	jobs := parseJobsJSON(payloads.jobs)
	queueWaits := parseQueueWaitJSON(payloads.jobs, collectedAt)
	queues := parseQueuesJSON(payloads.queues)

	snapshot := &Snapshot{
		CollectedAt:  collectedAt,
		Version:      version,
		Jobs:         jobs,
		QueueWaits:   queueWaits,
		Nodes:        parseNodesJSON(payloads.nodes),
		Queues:       queues,
		QueueSummary: summarizeQueues(queues),
		Server:       parseServerJSON(payloads.server),
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

	jobInspection, err := parseJobInspectionJSON(jobInspectionOutput, collectedAt)
	if err != nil {
		result.JobInspectionError = err
		return result, nil
	}

	snapshot.JobInspection = jobInspection

	return result, nil
}

func (c *Client) collectSnapshotPayloads(ctx context.Context) (*snapshotJSONPayloads, error) {
	jobsPayload, err := c.collectJobsJSON(ctx)
	if err != nil {
		return nil, err
	}

	nodesPayload, err := c.collectNodesJSON(ctx)
	if err != nil {
		return nil, err
	}

	queuesPayload, err := c.collectQueuesJSON(ctx)
	if err != nil {
		return nil, err
	}

	serverPayload, err := c.collectServerJSON(ctx)
	if err != nil {
		return nil, err
	}

	return &snapshotJSONPayloads{
		jobs:   jobsPayload,
		nodes:  nodesPayload,
		queues: queuesPayload,
		server: serverPayload,
	}, nil
}

func (c *Client) collectJobsJSON(ctx context.Context) (*jobsJSONPayload, error) {
	payload := &jobsJSONPayload{}
	if err := c.collectJSON(ctx, payload, "qstat", "-f", "-t", "-F", "json"); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) collectNodesJSON(ctx context.Context) (*nodesJSONPayload, error) {
	payload := &nodesJSONPayload{}
	if err := c.collectJSON(ctx, payload, "pbsnodes", "-aSj", "-F", "json"); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) collectQueuesJSON(ctx context.Context) (*queuesJSONPayload, error) {
	payload := &queuesJSONPayload{}
	if err := c.collectJSON(ctx, payload, "qstat", "-Q", "-f", "-F", "json"); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) collectServerJSON(ctx context.Context) (*serverJSONPayload, error) {
	payload := &serverJSONPayload{}
	if err := c.collectJSON(ctx, payload, "qstat", "-Bf", "-F", "json"); err != nil {
		return nil, err
	}
	return payload, nil
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
