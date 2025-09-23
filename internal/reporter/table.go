package reporter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/gavmckee/go-anta/internal/test"
)

type TableReporter struct {
	output io.Writer
}

func NewTableReporter() *TableReporter {
	return &TableReporter{
		output: os.Stdout,
	}
}

func (r *TableReporter) SetOutput(w io.Writer) {
	r.output = w
}

func (r *TableReporter) SetFormat(format string) {
}

func (r *TableReporter) Report(results []test.TestResult) error {
	if len(results) == 0 {
		fmt.Fprintln(r.output, "No test results to report")
		return nil
	}

	w := tabwriter.NewWriter(r.output, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "Device\tTest\tStatus\tMessage\tDuration")
	fmt.Fprintln(w, strings.Repeat("-", 80))

	for _, result := range results {
		status := r.formatStatus(result.Status)
		message := result.Message
		if message == "" {
			message = "-"
		}
		duration := fmt.Sprintf("%.2fs", result.Duration.Seconds())

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			result.DeviceName,
			result.TestName,
			status,
			message,
			duration,
		)
	}

	fmt.Fprintln(w, strings.Repeat("-", 80))
	
	stats := r.calculateStats(results)
	fmt.Fprintf(w, "\nSummary: Total: %d | Success: %d | Failure: %d | Error: %d | Skipped: %d\n",
		stats["total"], stats["success"], stats["failure"], stats["error"], stats["skipped"])

	return nil
}

func (r *TableReporter) formatStatus(status test.TestStatus) string {
	switch status {
	case test.TestSuccess:
		return "✅ SUCCESS"
	case test.TestFailure:
		return "❌ FAILURE"
	case test.TestError:
		return "⚠️  ERROR"
	case test.TestSkipped:
		return "⏭️  SKIPPED"
	default:
		return "❓ UNSET"
	}
}

func (r *TableReporter) calculateStats(results []test.TestResult) map[string]int {
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