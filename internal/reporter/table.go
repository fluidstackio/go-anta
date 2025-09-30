package reporter

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/gavmckee/go-anta/internal/test"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
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

	// Group results by device
	deviceGroups := r.groupResultsByDevice(results)

	// Create main table with styling
	t := table.NewWriter()
	t.SetOutputMirror(r.output)
	t.SetStyle(table.StyleColoredBright)

	// Configure table appearance
	t.Style().Format.Header = text.FormatUpper
	t.Style().Options.DrawBorder = true
	t.Style().Options.SeparateColumns = true
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = false

	// Set headers
	t.AppendHeader(table.Row{"Device", "Test", "Status", "Message", "Duration"})

	// Sort devices for consistent output
	devices := make([]string, 0, len(deviceGroups))
	for device := range deviceGroups {
		devices = append(devices, device)
	}
	sort.Strings(devices)

	// Add rows grouped by device
	for i, deviceName := range devices {
		deviceResults := deviceGroups[deviceName]

		// Sort tests within device for consistent output
		sort.Slice(deviceResults, func(a, b int) bool {
			return deviceResults[a].TestName < deviceResults[b].TestName
		})

		for j, result := range deviceResults {
			status := r.formatStatus(result.Status)
			message := result.Message
			if message == "" {
				message = "-"
			}
			duration := fmt.Sprintf("%.2fs", result.Duration.Seconds())

			// Use device name only for first row of each device group
			device := ""
			if j == 0 {
				device = deviceName
			}

			row := table.Row{device, result.TestName, status, message, duration}

			// Apply row styling based on test status
			switch result.Status {
			case test.TestSuccess:
				t.AppendRow(row, table.RowConfig{AutoMerge: true})
			case test.TestFailure:
				t.AppendRow(row, table.RowConfig{AutoMerge: true})
			case test.TestError:
				t.AppendRow(row, table.RowConfig{AutoMerge: true})
			case test.TestSkipped:
				t.AppendRow(row, table.RowConfig{AutoMerge: true})
			default:
				t.AppendRow(row, table.RowConfig{AutoMerge: true})
			}
		}

		// Add separator between devices (except for last device)
		if i < len(devices)-1 {
			t.AppendSeparator()
		}
	}

	// Render the main table
	t.Render()

	// Add summary section
	r.renderSummary(results)

	return nil
}

func (r *TableReporter) formatStatus(status test.TestStatus) string {
	switch status {
	case test.TestSuccess:
		return text.FgGreen.Sprint("✓ SUCCESS")
	case test.TestFailure:
		return text.FgRed.Sprint("✗ FAILURE")
	case test.TestError:
		return text.FgYellow.Sprint("⚠ ERROR")
	case test.TestSkipped:
		return text.FgCyan.Sprint("⊝ SKIPPED")
	default:
		return text.FgMagenta.Sprint("? UNSET")
	}
}

func (r *TableReporter) groupResultsByDevice(results []test.TestResult) map[string][]test.TestResult {
	groups := make(map[string][]test.TestResult)
	for _, result := range results {
		groups[result.DeviceName] = append(groups[result.DeviceName], result)
	}
	return groups
}

func (r *TableReporter) renderSummary(results []test.TestResult) {
	stats := r.calculateStats(results)

	// Create summary table
	summaryTable := table.NewWriter()
	summaryTable.SetOutputMirror(r.output)
	summaryTable.SetStyle(table.StyleRounded)
	summaryTable.Style().Options.DrawBorder = true
	summaryTable.Style().Options.SeparateColumns = true
	summaryTable.Style().Format.Header = text.FormatUpper

	summaryTable.AppendHeader(table.Row{"Metric", "Count", "Percentage"})

	total := float64(stats["total"])
	if total > 0 {
		summaryTable.AppendRow(table.Row{
			text.FgWhite.Sprint("Total Tests"),
			fmt.Sprintf("%d", stats["total"]),
			"100.0%",
		})
		summaryTable.AppendSeparator()
		summaryTable.AppendRow(table.Row{
			text.FgGreen.Sprint("✓ Success"),
			fmt.Sprintf("%d", stats["success"]),
			fmt.Sprintf("%.1f%%", float64(stats["success"])/total*100),
		})
		summaryTable.AppendRow(table.Row{
			text.FgRed.Sprint("✗ Failure"),
			fmt.Sprintf("%d", stats["failure"]),
			fmt.Sprintf("%.1f%%", float64(stats["failure"])/total*100),
		})
		summaryTable.AppendRow(table.Row{
			text.FgYellow.Sprint("⚠ Error"),
			fmt.Sprintf("%d", stats["error"]),
			fmt.Sprintf("%.1f%%", float64(stats["error"])/total*100),
		})
		summaryTable.AppendRow(table.Row{
			text.FgCyan.Sprint("⊝ Skipped"),
			fmt.Sprintf("%d", stats["skipped"]),
			fmt.Sprintf("%.1f%%", float64(stats["skipped"])/total*100),
		})
	}

	fmt.Fprintln(r.output, "\nTest Execution Summary:")
	summaryTable.Render()

	// Add overall success rate
	if total > 0 {
		successRate := float64(stats["success"]) / total * 100
		fmt.Fprintln(r.output)
		if successRate == 100.0 {
			fmt.Fprintln(r.output, text.FgGreen.Sprintf("🎉 Success Rate: %.1f%% - All tests passed!", successRate))
		} else if successRate >= 90.0 {
			fmt.Fprintln(r.output, text.FgGreen.Sprintf("👍 Success Rate: %.1f%% - Most tests passed", successRate))
		} else if successRate >= 70.0 {
			fmt.Fprintln(r.output, text.FgYellow.Sprintf("⚠️  Success Rate: %.1f%% - Some tests failed", successRate))
		} else {
			fmt.Fprintln(r.output, text.FgRed.Sprintf("❌ Success Rate: %.1f%% - Many tests failed", successRate))
		}
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