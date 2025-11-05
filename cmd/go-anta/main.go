package main

import (
	"fmt"
	"os"

	"github.com/gavmckee/go-anta/internal/cli"
	_ "github.com/gavmckee/go-anta/tests"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}