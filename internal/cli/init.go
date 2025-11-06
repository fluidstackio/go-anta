package cli

import (
	"github.com/fluidstack/go-anta/internal/cli/commands"
)

func init() {
	rootCmd.AddCommand(commands.NrfuCmd)
	rootCmd.AddCommand(commands.CheckCmd)
	rootCmd.AddCommand(commands.InventoryCmd)
}