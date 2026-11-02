// Package cmd wires the Cobra command tree. The binary has a single
// entry point: `asciiflow start`.
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "asciiflow",
	Short: "ASCII ocean screensaver",
	Long:  "A fullscreen terminal screensaver: fish schools hunted by ASCII sharks. Any key exits.",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newStartCmd())
	rootCmd.SilenceUsage = true
}
