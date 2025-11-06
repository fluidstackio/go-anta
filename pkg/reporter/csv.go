package reporter

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/fluidstack/go-anta/pkg/test"
)

type CSVReporter struct {
	output io.Writer
}

func NewCSVReporter() *CSVReporter {
	return &CSVReporter{
		output: os.Stdout,
	}
}

func (r *CSVReporter) SetOutput(w io.Writer) {
	r.output = w
}

func (r *CSVReporter) SetFormat(format string) {
}

func (r *CSVReporter) Report(results []test.TestResult) error {
	writer := csv.NewWriter(r.output)
	defer writer.Flush()

	headers := []string{"Device", "Test", "Status", "Message", "Duration", "Timestamp", "Categories"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	for _, result := range results {
		categories := ""
		if len(result.Categories) > 0 {
			categories = fmt.Sprintf("%v", result.Categories)
		}

		row := []string{
			result.DeviceName,
			result.TestName,
			result.Status.String(),
			result.Message,
			fmt.Sprintf("%.3f", result.Duration.Seconds()),
			result.Timestamp.Format("2006-01-02 15:04:05"),
			categories,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}