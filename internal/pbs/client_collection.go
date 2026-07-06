package pbs

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`\d{4}\.\d+\.\d+`)

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
	if !c.includeDetailedJobData {
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

	version := versionPattern.FindString(output)
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
