// Command asciiflow is a fullscreen terminal screensaver: fish schools
// hunted by ASCII sharks. Go port of stackedlol/ascii-sharks
package main

import (
	"os"

	"github.com/iman1704/ascii-flow/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
