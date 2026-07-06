package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"pbs-exporter/internal/pbs"
)

var (
	defaultJobSampleCPUBucketsText      = []string{"1", "2", "4", "8", "16", "32", "64", "128", "256"}
	defaultJobSampleMemoryBucketsText   = []string{"1gb", "2gb", "4gb", "8gb", "16gb", "32gb", "64gb", "128gb", "256gb", "512gb"}
	defaultJobSampleWalltimeBucketsText = []string{"30m", "1h", "2h", "4h", "8h", "12h", "24h", "48h", "96h", "168h"}
	defaultJobSampleMPIBucketsText      = []string{"1", "2", "4", "8", "16", "32", "64", "128", "256", "512"}
	defaultJobSampleNodeBucketsText     = []string{"1", "2", "4", "8", "16", "32", "64"}
	defaultJobSampleGPUBucketsText      = []string{"1", "2", "4", "8", "16", "32"}
)

type numericBucketValues struct {
	values []float64
	set    bool
}

type memoryBucketValues struct {
	values []float64
	set    bool
}

type durationBucketValues struct {
	values []float64
	set    bool
}

func (b *numericBucketValues) UnmarshalYAML(node *yaml.Node) error {
	values, err := parseBucketSequenceNode(node, parseNumericBucketValue)
	if err != nil {
		return err
	}
	b.values = values
	b.set = true
	return nil
}

func (b *memoryBucketValues) UnmarshalYAML(node *yaml.Node) error {
	values, err := parseBucketSequenceNode(node, parseMemoryBucketValue)
	if err != nil {
		return err
	}
	b.values = values
	b.set = true
	return nil
}

func (b *durationBucketValues) UnmarshalYAML(node *yaml.Node) error {
	values, err := parseBucketSequenceNode(node, parseDurationBucketValue)
	if err != nil {
		return err
	}
	b.values = values
	b.set = true
	return nil
}

func defaultJobSampleCPUBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleCPUBucketsText, parseNumericBucketValue)
}

func defaultJobSampleMemoryBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleMemoryBucketsText, parseMemoryBucketValue)
}

func defaultJobSampleWalltimeBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleWalltimeBucketsText, parseDurationBucketValue)
}

func defaultJobSampleMPIBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleMPIBucketsText, parseNumericBucketValue)
}

func defaultJobSampleNodeBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleNodeBucketsText, parseNumericBucketValue)
}

func defaultJobSampleGPUBuckets() []float64 {
	return mustParseBucketTexts(defaultJobSampleGPUBucketsText, parseNumericBucketValue)
}

func parseNumericBuckets(raw string) ([]float64, error) {
	return parseBucketCSV(raw, parseNumericBucketValue)
}

func parseMemoryBuckets(raw string) ([]float64, error) {
	return parseBucketCSV(raw, parseMemoryBucketValue)
}

func parseDurationBuckets(raw string) ([]float64, error) {
	return parseBucketCSV(raw, parseDurationBucketValue)
}

func formatNumericBuckets(values []float64) string {
	return formatBuckets(values, func(value float64) string {
		return strconv.FormatFloat(value, 'f', -1, 64)
	})
}

func formatMemoryBuckets(values []float64) string {
	return formatBuckets(values, func(value float64) string {
		return strconv.FormatFloat(value, 'f', -1, 64)
	})
}

func formatDurationBuckets(values []float64) string {
	return formatBuckets(values, func(value float64) string {
		return time.Duration(value * float64(time.Second)).String()
	})
}

func validateBucketValues(name string, values []float64) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must contain at least one bucket", name)
	}

	previous := 0.0
	for index, value := range values {
		if value <= 0 {
			return fmt.Errorf("%s bucket %d must be greater than zero", name, index)
		}
		if index > 0 && value <= previous {
			return fmt.Errorf("%s buckets must be strictly increasing", name)
		}
		previous = value
	}

	return nil
}

func parseBucketSequenceNode(node *yaml.Node, parser func(string) (float64, error)) ([]float64, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("bucket list must be a sequence")
	}

	values := make([]float64, 0, len(node.Content))
	for _, item := range node.Content {
		value, err := parser(strings.TrimSpace(item.Value))
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func parseBucketCSV(raw string, parser func(string) (float64, error)) ([]float64, error) {
	parts := strings.Split(raw, ",")
	values := make([]float64, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("bucket values must not be empty")
		}

		value, err := parser(trimmed)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func parseNumericBucketValue(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric bucket %q: %w", raw, err)
	}
	return value, nil
}

func parseMemoryBucketValue(raw string) (float64, error) {
	value, ok := pbs.ParseMemoryBytes(raw)
	if !ok {
		return 0, fmt.Errorf("invalid memory bucket %q", raw)
	}
	return value, nil
}

func parseDurationBucketValue(raw string) (float64, error) {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid duration bucket %q: %w", raw, err)
	}
	return value.Seconds(), nil
}

func formatBuckets(values []float64, formatter func(float64) string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, formatter(value))
	}
	return strings.Join(parts, ",")
}

func mustParseBucketTexts(raw []string, parser func(string) (float64, error)) []float64 {
	values := make([]float64, 0, len(raw))
	for _, item := range raw {
		value, err := parser(item)
		if err != nil {
			panic(err)
		}
		values = append(values, value)
	}
	return values
}
