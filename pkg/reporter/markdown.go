package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fluidstack/go-anta/pkg/test"
)

type MarkdownReporter struct {
	output io.Writer
}

func NewMarkdownReporter() *MarkdownReporter {
	return &MarkdownReporter{
		output: os.Stdout,
	}
}

func (r *MarkdownReporter) SetOutput(w io.Writer) {
	r.output = w
}

func (r *MarkdownReporter) SetFormat(format string) {
}

func (r *MarkdownReporter) Report(results []test.TestResult) error {
	fmt.Fprintln(r.output, "# GANTA Test Report")
	fmt.Fprintln(r.output)
	
	if len(results) == 0 {
		fmt.Fprintln(r.output, "No test results to report.")
		return nil
	}

	stats := r.calculateStats(results)
	fmt.Fprintln(r.output, "## Summary")
	fmt.Fprintln(r.output)
	fmt.Fprintf(r.output, "- **Total Tests**: %d\n", stats["total"])
	fmt.Fprintf(r.output, "- **Success**: %d\n", stats["success"])
	fmt.Fprintf(r.output, "- **Failure**: %d\n", stats["failure"])
	fmt.Fprintf(r.output, "- **Error**: %d\n", stats["error"])
	fmt.Fprintf(r.output, "- **Skipped**: %d\n", stats["skipped"])
	fmt.Fprintln(r.output)

	fmt.Fprintln(r.output, "## Test Results")
	fmt.Fprintln(r.output)
	fmt.Fprintln(r.output, "| Device | Test | Status | Message | Duration |")
	fmt.Fprintln(r.output, "|--------|------|--------|---------|----------|")

	for _, result := range results {
		status := r.formatStatus(result.Status)
		message := result.Message
		if message == "" {
			message = "-"
		}
		message = strings.ReplaceAll(message, "|", "\\|")
		message = strings.ReplaceAll(message, "\n", " ")
		duration := fmt.Sprintf("%.2fs", result.Duration.Seconds())

		fmt.Fprintf(r.output, "| %s | %s | %s | %s | %s |\n",
			result.DeviceName,
			result.TestName,
			status,
			message,
			duration,
		)
	}

	fmt.Fprintln(r.output)

	if stats["failure"] > 0 || stats["error"] > 0 {
		fmt.Fprintln(r.output, "## Failed Tests")
		fmt.Fprintln(r.output)
		
		for _, result := range results {
			if result.Status == test.TestFailure || result.Status == test.TestError {
				fmt.Fprintf(r.output, "### %s - %s\n", result.DeviceName, result.TestName)
				fmt.Fprintf(r.output, "- **Status**: %s\n", result.Status.String())
				if result.Message != "" {
					fmt.Fprintf(r.output, "- **Message**: %s\n", result.Message)
				}
				fmt.Fprintf(r.output, "- **Duration**: %.2fs\n", result.Duration.Seconds())
				fmt.Fprintln(r.output)
			}
		}
	}

	return nil
}

func (r *MarkdownReporter) formatStatus(status test.TestStatus) string {
	switch status {
	case test.TestSuccess:
		return "✅ SUCCESS"
	case test.TestFailure:
		return "❌ FAILURE"
	case test.TestError:
		return "⚠️ ERROR"
	case test.TestSkipped:
		return "⏭️ SKIPPED"
	default:
		return "❓ UNSET"
	}
}

func (r *MarkdownReporter) calculateStats(results []test.TestResult) map[string]int {
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