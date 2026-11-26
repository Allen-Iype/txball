// Command txball is a brick-breaker for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Allen-Iype/txball/internal/app"
)

func main() {
	var opts app.Options
	flag.BoolVar(&opts.Debug, "debug", false, "show FPS/UPS counters; keys 1-7 drop a power-up")
	flag.IntVar(&opts.Level, "level", 1, "level to start on")
	flag.DurationVar(&opts.PanicAfter, "panic-after", 0, "panic after this long (tests terminal restore)")
	flag.Parse()

	if err := app.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "txball:", err)
		os.Exit(1)
	}
}
