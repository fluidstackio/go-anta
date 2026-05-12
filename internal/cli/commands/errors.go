package commands

import "errors"

// ErrTestsFailed signals that the framework ran to completion but one or
// more tests reported a Failure or Error status. Callers should map this
// to a non-zero exit code distinct from framework-level errors so CI
// pipelines can distinguish "we found a problem" from "the runner broke".
var ErrTestsFailed = errors.New("one or more tests failed")
