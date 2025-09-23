package cli

import (
	"github.com/gavmckee/go-anta/internal/cli/commands"
)

func init() {
	rootCmd.AddCommand(commands.NrfuCmd)
	rootCmd.AddCommand(commands.CheckCmd)
	rootCmd.AddCommand(commands.InventoryCmd)
}