package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fluidstackio/go-anta/internal/cli"
	"github.com/fluidstackio/go-anta/internal/cli/commands"
	_ "github.com/fluidstackio/go-anta/tests"
)

func main() {
	err := cli.Execute()
	switch {
	case err == nil:
		os.Exit(0)
	case errors.Is(err, commands.ErrTestsFailed):
		// Test results have already been printed by the reporter; exit 1
		// to signal failure to CI without an extra error line.
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}
