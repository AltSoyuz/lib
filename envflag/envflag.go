package envflag

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	prefix = flag.String("envflag.prefix", "", "Prefix for environment variables")
)

// Parse parses environment vars and command-line flags.
//
// Flags set via command-line override flags set via environment vars.
//
// This function must be called instead of flag.Parse() before using any flags in the program.
func Parse() {
	ParseFlagSet(flag.CommandLine, os.Args[1:])
}

// ParseFlagSet parses the given args into the given fs.
func ParseFlagSet(fs *flag.FlagSet, args []string) {
	// Keep existing behavior: fatal on error for backward compatibility.
	if err := ParseFlagSetErr(fs, args); err != nil {
		// Do not use lib/logger here, since it is uninitialized yet.
		log.Fatalf("%s", err)
	}
}

// ParseFlagSetErr behaves like ParseFlagSet but returns an error instead of calling log.Fatalf.
// Use this when the caller prefers to handle parse errors instead of exiting the process.
func ParseFlagSetErr(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("cannot parse flags %q: %w", args, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unprocessed command-line args left: %s; the most likely reason is missing `=` between boolean flag name and value; see https://pkg.go.dev/flag#hdr-Command_line_flag_syntax", fs.Args())
	}
	// Remember explicitly set command-line flags.
	flagsSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	// Obtain the remaining flag values from environment vars.
	var setErr error
	fs.VisitAll(func(f *flag.Flag) {
		if setErr != nil || flagsSet[f.Name] {
			return
		}
		fname := getEnvFlagName(f.Name)
		if v, ok := os.LookupEnv(fname); ok {
			if err := fs.Set(f.Name, v); err != nil {
				setErr = fmt.Errorf("cannot set flag %s to %q, which is read from env var %q: %w", f.Name, v, fname, err)
			}
		}
	})
	return setErr
}

func getEnvFlagName(s string) string {
	// Substitute dots with underscores, since env var names cannot contain dots.
	s = strings.ReplaceAll(s, ".", "_")
	return *prefix + s
}
