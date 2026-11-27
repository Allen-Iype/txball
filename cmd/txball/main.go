// Command txball is a brick-breaker for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Allen-Iype/txball/internal/app"
)

// version is set by the release build (-ldflags "-X main.version=...").
var version string

func main() {
	var opts app.Options
	flag.BoolVar(&opts.Debug, "debug", false, "show FPS/UPS counters; keys 1-7 drop a power-up")
	flag.BoolVar(&opts.Bell, "bell", false, "ring the terminal bell on hits")
	flag.BoolVar(&opts.Demo, "demo", false, "let the game play itself")
	flag.IntVar(&opts.Level, "level", 1, "level to start on")
	flag.DurationVar(&opts.PanicAfter, "panic-after", 0, "panic after this long (tests terminal restore)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("txball", buildVersion())
		return
	}

	if err := app.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "txball:", err)
		os.Exit(1)
	}
}

// buildVersion reports the release version, falling back to the module
// version that "go install" records, and then to "dev" for local builds.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
