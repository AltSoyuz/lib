package buildinfo

import (
	"flag"
	"fmt"
	"os"

	"github.com/VictoriaMetrics/metrics"
)

var version = flag.Bool("version", false, "Show app version")

// Version must be set via -ldflags '-X'
var Version string

// Init must be called after flag.Parse call.
func Init() {
	if *version {
		printVersion()
		os.Exit(0)
	}
	// Expose build version as a Prometheus info metric (value always 1).
	// Allows correlating metrics with the deployed binary version.
	v := Version
	if v == "" {
		v = "dev"
	}
	metrics.NewGauge(fmt.Sprintf(`app_build_info{version=%q}`, v), func() float64 { return 1 })
}

func init() {
	oldUsage := flag.Usage
	flag.Usage = func() {
		printVersion()
		oldUsage()
	}
}

func printVersion() {
	if _, err := fmt.Fprintf(flag.CommandLine.Output(), "%s\n", Version); err != nil {
		fmt.Fprintf(os.Stderr, "failed to print version: %v\n", err)
	}
}
