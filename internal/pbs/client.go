package pbs

import (
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	binaryDir              string
	timeout                time.Duration
	includeDetailedJobData bool
	logger                 *slog.Logger
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
	IncludeDetailedJobData bool
}

// CollectionResult always represents a successful base snapshot collection.
// Snapshot must be non-nil when Collect returns a nil error.
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
		binaryDir:              binaryDir,
		timeout:                timeout,
		includeDetailedJobData: options.IncludeDetailedJobData,
		logger:                 logger,
	}
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

func ParseMemoryBytes(memStr string) (float64, bool) {
	memBytes := parseMemoryToBytes(memStr)
	normalized := strings.ToLower(strings.TrimSpace(memStr))
	if memBytes == 0 && normalized != "0" && normalized != "0b" && normalized != "0kb" && normalized != "0mb" && normalized != "0gb" && normalized != "0tb" {
		return 0, false
	}
	return memBytes, true
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
