package pbs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type JobInspectionData struct {
	Jobs []InspectedJob
}

type InspectedJob struct {
	JobID            string
	Queue            string
	Project          string
	JobOwner         string
	JobState         string
	Requested        RequestedJobResources
	Used             UsedJobResources
	RuntimeSeconds   OptionalFloat64
	QueueWaitSeconds OptionalFloat64
}

type RequestedJobResources struct {
	MemoryBytes     OptionalFloat64
	WalltimeSeconds OptionalFloat64
	CPUCores        OptionalFloat64
	GPUDevices      OptionalFloat64
	MPIProcesses    OptionalFloat64
	Nodes           OptionalFloat64
}

type UsedJobResources struct {
	CPUPercent         OptionalFloat64
	CPUTimeSeconds     OptionalFloat64
	MemoryBytes        OptionalFloat64
	VirtualMemoryBytes OptionalFloat64
	CPUCores           OptionalFloat64
	GPUDevices         OptionalFloat64
	WalltimeSeconds    OptionalFloat64
}

type OptionalFloat64 struct {
	Value float64
	Set   bool
}

func newOptionalFloat64(value float64) OptionalFloat64 {
	return OptionalFloat64{Value: value, Set: true}
}

type jobInspectionPayload struct {
	Jobs map[string]jobInspectionRecord `json:"Jobs"`
}

type jobInspectionRecord struct {
	Queue         string                `json:"queue"`
	Project       string                `json:"project"`
	JobOwner      string                `json:"Job_Owner"`
	JobState      string                `json:"job_state"`
	Qtime         string                `json:"qtime"`
	Stime         string                `json:"stime"`
	ResourceList  jobInspectionResource `json:"Resource_List"`
	ResourcesUsed jobInspectionResource `json:"resources_used"`
}

type jobInspectionResource struct {
	Mem        jsonScalar `json:"mem"`
	MPIProcs   jsonScalar `json:"mpiprocs"`
	NCPUs      jsonScalar `json:"ncpus"`
	NGPUs      jsonScalar `json:"ngpus"`
	NodeCount  jsonScalar `json:"nodect"`
	Walltime   jsonScalar `json:"walltime"`
	CPUPercent jsonScalar `json:"cpupercent"`
	CPUTime    jsonScalar `json:"cput"`
	Vmem       jsonScalar `json:"vmem"`
}

type jsonScalar struct {
	text string
	set  bool
}

func (s *jsonScalar) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*s = jsonScalar{}
		return nil
	}

	if len(trimmed) == 0 {
		*s = jsonScalar{}
		return nil
	}

	if trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil
		}
		s.text = strings.TrimSpace(value)
		s.set = true
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(trimmed, &number); err == nil {
		s.text = number.String()
		s.set = true
		return nil
	}

	*s = jsonScalar{}
	return nil
}

func (s jsonScalar) optionalNumber() OptionalFloat64 {
	if !s.set || s.text == "" {
		return OptionalFloat64{}
	}

	value, err := strconv.ParseFloat(s.text, 64)
	if err != nil {
		return OptionalFloat64{}
	}

	return newOptionalFloat64(value)
}

func (s jsonScalar) optionalDurationSeconds() OptionalFloat64 {
	if !s.set || s.text == "" {
		return OptionalFloat64{}
	}

	seconds := parseWalltimeToSeconds(s.text)
	if seconds == 0 && strings.TrimSpace(s.text) != "0" && strings.TrimSpace(s.text) != "00:00" && strings.TrimSpace(s.text) != "00:00:00" {
		return OptionalFloat64{}
	}

	return newOptionalFloat64(float64(seconds))
}

func (s jsonScalar) optionalBytes() OptionalFloat64 {
	if !s.set || s.text == "" {
		return OptionalFloat64{}
	}

	value := parseMemoryToBytes(s.text)
	if value == 0 && strings.TrimSpace(strings.ToLower(s.text)) != "0" && strings.TrimSpace(strings.ToLower(s.text)) != "0b" && strings.TrimSpace(strings.ToLower(s.text)) != "0kb" && strings.TrimSpace(strings.ToLower(s.text)) != "0mb" && strings.TrimSpace(strings.ToLower(s.text)) != "0gb" && strings.TrimSpace(strings.ToLower(s.text)) != "0tb" {
		return OptionalFloat64{}
	}

	return newOptionalFloat64(value)
}

func (c *Client) ParseJobInspectionOutput(output string, collectedAt time.Time) (*JobInspectionData, error) {
	var payload jobInspectionPayload
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, fmt.Errorf("parse qstat -F json -f output: %w", err)
	}

	jobIDs := make([]string, 0, len(payload.Jobs))
	for jobID := range payload.Jobs {
		jobIDs = append(jobIDs, jobID)
	}
	sort.Strings(jobIDs)

	data := &JobInspectionData{
		Jobs: make([]InspectedJob, 0, len(jobIDs)),
	}

	for _, jobID := range jobIDs {
		record := payload.Jobs[jobID]
		job := InspectedJob{
			JobID:    jobID,
			Queue:    record.Queue,
			Project:  record.Project,
			JobOwner: record.JobOwner,
			JobState: record.JobState,
			Requested: RequestedJobResources{
				MemoryBytes:     record.ResourceList.Mem.optionalBytes(),
				WalltimeSeconds: record.ResourceList.Walltime.optionalDurationSeconds(),
				CPUCores:        record.ResourceList.NCPUs.optionalNumber(),
				GPUDevices:      record.ResourceList.NGPUs.optionalNumber(),
				MPIProcesses:    record.ResourceList.MPIProcs.optionalNumber(),
				Nodes:           record.ResourceList.NodeCount.optionalNumber(),
			},
		}

		if strings.EqualFold(record.JobState, "R") {
			job.Used = UsedJobResources{
				CPUPercent:         record.ResourcesUsed.CPUPercent.optionalNumber(),
				CPUTimeSeconds:     record.ResourcesUsed.CPUTime.optionalDurationSeconds(),
				MemoryBytes:        record.ResourcesUsed.Mem.optionalBytes(),
				VirtualMemoryBytes: record.ResourcesUsed.Vmem.optionalBytes(),
				CPUCores:           record.ResourcesUsed.NCPUs.optionalNumber(),
				GPUDevices:         record.ResourcesUsed.NGPUs.optionalNumber(),
				WalltimeSeconds:    record.ResourcesUsed.Walltime.optionalDurationSeconds(),
			}
			job.RuntimeSeconds = optionalElapsedSeconds(record.Stime, collectedAt)
		}

		if strings.EqualFold(record.JobState, "Q") {
			job.QueueWaitSeconds = optionalElapsedSeconds(record.Qtime, collectedAt)
		}

		data.Jobs = append(data.Jobs, job)
	}

	return data, nil
}

func optionalElapsedSeconds(rawTimestamp string, collectedAt time.Time) OptionalFloat64 {
	parsed, ok := parseQtime(rawTimestamp)
	if !ok {
		return OptionalFloat64{}
	}

	elapsed := collectedAt.Sub(parsed).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}

	return newOptionalFloat64(elapsed)
}
