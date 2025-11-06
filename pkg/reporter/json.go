package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/fluidstack/go-anta/pkg/test"
)

type JSONReporter struct {
	output io.Writer
	indent bool
}

func NewJSONReporter() *JSONReporter {
	return &JSONReporter{
		output: os.Stdout,
		indent: true,
	}
}

func (r *JSONReporter) SetOutput(w io.Writer) {
	r.output = w
}

func (r *JSONReporter) SetFormat(format string) {
	if format == "json-compact" {
		r.indent = false
	}
}

func (r *JSONReporter) Report(results []test.TestResult) error {
	report := struct {
		Results    []test.TestResult `json:"results"`
		Statistics map[string]int    `json:"statistics"`
	}{
		Results:    results,
		Statistics: r.calculateStats(results),
	}

	encoder := json.NewEncoder(r.output)
	if r.indent {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode JSON report: %w", err)
	}

	return nil
}

func (r *JSONReporter) calculateStats(results []test.TestResult) map[string]int {
	stats := map[string]int{
		"total":   len(results),
		"success": 0,
		"failure": 0,
		"error":   0,
		"skipped": 0,
	}

	for _, result := range results {
		switch result.Status {
		case test.TestSuccess:
			stats["success"]++
		case test.TestFailure:
			stats["failure"]++
		case test.TestError:
			stats["error"]++
		case test.TestSkipped:
			stats["skipped"]++
		}
	}

	return stats
}