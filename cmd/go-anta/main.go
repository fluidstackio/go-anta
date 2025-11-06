package main

import (
	"fmt"
	"os"

	"github.com/fluidstack/go-anta/internal/cli"
	_ "github.com/fluidstack/go-anta/tests"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}