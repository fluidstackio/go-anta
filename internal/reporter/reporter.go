package reporter

import (
	"io"

	"github.com/gavmckee/go-anta/internal/test"
)

type Reporter interface {
	Report(results []test.TestResult) error
	SetOutput(w io.Writer)
	SetFormat(format string)
}