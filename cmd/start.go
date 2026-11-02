package cmd

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/iman1704/ascii-flow/internal/engine"
)

func newStartCmd() *cobra.Command {
	var fps int
	var noPredators string

	c := &cobra.Command{
		Use:   "start",
		Short: "Start the screensaver (any key exits)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			noSharks, noPelagics, err := parseNoPredators(noPredators)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return engine.Run(ctx, engine.Options{
				FPS:        fps,
				NoSharks:   noSharks,
				NoPelagics: noPelagics,
			})
		},
	}

	c.Flags().IntVar(&fps, "fps", 30, "render frames per second")
	c.Flags().StringVar(&noPredators, "no-predators", "", "leave hunters out: all, sharks, or pelagics (tuna, mackerel)")
	// A bare --no-predators keeps the old screensaver meaning: all
	// hunters stay out of the water.
	c.Flags().Lookup("no-predators").NoOptDefVal = "all"
	return c
}

// parseNoPredators maps --no-predators to the two hunter toggles. Empty
// means the full ocean; "all" leaves out sharks and pelagics, "sharks"
// only sharks, "pelagics" only tuna and mackerel.
func parseNoPredators(flag string) (noSharks, noPelagics bool, err error) {
	switch flag {
	case "":
		return false, false, nil
	case "all":
		return true, true, nil
	case "sharks":
		return true, false, nil
	case "pelagics":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("--no-predators must be one of all, sharks, pelagics, got %q", flag)
	}
}
