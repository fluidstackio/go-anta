package reporter

import (
	"io"

	"github.com/fluidstack/go-anta/pkg/test"
)

type Reporter interface {
	Report(results []test.TestResult) error
	SetOutput(w io.Writer)
	SetFormat(format string)
}