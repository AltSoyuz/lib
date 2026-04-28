package buildinfo

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	f := func(version, expected string) {
		t.Helper()

		// Save original values
		origVersion := Version
		origOutput := flag.CommandLine.Output()
		defer func() {
			Version = origVersion
			flag.CommandLine.SetOutput(origOutput)
		}()

		// Setup
		Version = version
		var buf bytes.Buffer
		flag.CommandLine.SetOutput(&buf)

		// Execute
		printVersion()

		// Verify
		got := buf.String()
		if got != expected {
			t.Fatalf("unexpected output; got %q; want %q", got, expected)
		}
	}

	t.Run("with version", func(t *testing.T) { f("v1.2.3", "v1.2.3\n") })
	t.Run("empty version", func(t *testing.T) { f("", "\n") })
	t.Run("dev version", func(t *testing.T) { f("dev", "dev\n") })
}

func TestUsageShowsVersion(t *testing.T) {
	// Save original values
	origVersion := Version
	origOutput := flag.CommandLine.Output()
	defer func() {
		Version = origVersion
		flag.CommandLine.SetOutput(origOutput)
	}()

	// Setup
	Version = "v1.0.0"
	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)

	// Execute
	flag.Usage()

	// Verify
	output := buf.String()
	if !strings.Contains(output, "v1.0.0") {
		t.Fatalf("Usage() output should contain version; got %q", output)
	}
}

func TestInit(t *testing.T) {
	// Note: Testing Init() with -version flag would cause os.Exit(0)
	// which terminates the test process. In a real scenario, you might
	// want to refactor Init() to be more testable by extracting the
	// exit behavior, or use subprocess testing with exec.Command.
	// For now, we test the non-exit path.

	// Save original flag value
	origVersion := *version
	defer func() {
		*version = origVersion
	}()

	// Test that Init() doesn't panic when version flag is false
	*version = false
	Init() // Should not exit
}

func TestVersionFlagRegistered(t *testing.T) {
	f := flag.Lookup("version")
	if f == nil {
		t.Fatal("version flag not registered")
	}
	if f.DefValue != "false" {
		t.Fatalf("unexpected default value; got %q; want %q", f.DefValue, "false")
	}
}
