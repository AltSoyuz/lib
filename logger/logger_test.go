package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	f := func(level, logFunc, expectedLevel string, shouldLog bool) {
		t.Helper()

		var buf bytes.Buffer
		SetOutput(&buf)
		defer ResetOutput()

		oldLevel := *loggerLevel
		*loggerLevel = level
		defer func() { *loggerLevel = oldLevel }()

		switch logFunc {
		case "Info":
			Info("test message")
		case "Warn":
			Warn("test message")
		case "Error":
			Error("test message")
		}

		output := buf.String()
		if shouldLog {
			if output == "" {
				t.Fatalf("expected log output for level=%s func=%s", level, logFunc)
			}
			if !strings.Contains(output, expectedLevel) {
				t.Fatalf("expected level=%s in output; got %q", expectedLevel, output)
			}
			if !strings.Contains(output, "test message") {
				t.Fatalf("expected message in output; got %q", output)
			}
		} else {
			if output != "" {
				t.Fatalf("expected no output for level=%s func=%s; got %q", level, logFunc, output)
			}
		}
	}

	t.Run("INFO level logs all", func(t *testing.T) {
		f("INFO", "Info", "info", true)
		f("INFO", "Warn", "warn", true)
		f("INFO", "Error", "error", true)
	})

	t.Run("WARN level filters INFO", func(t *testing.T) {
		f("WARN", "Info", "", false)
		f("WARN", "Warn", "warn", true)
		f("WARN", "Error", "error", true)
	})

	t.Run("ERROR level filters INFO and WARN", func(t *testing.T) {
		f("ERROR", "Info", "", false)
		f("ERROR", "Warn", "", false)
		f("ERROR", "Error", "error", true)
	})
}

func TestLogFormatting(t *testing.T) {
	f := func(msg string, kv []any, expectedPairs []string) {
		t.Helper()

		var buf bytes.Buffer
		SetOutput(&buf)
		defer ResetOutput()

		Info(msg, kv...)

		output := buf.String()
		if !strings.Contains(output, msg) {
			t.Fatalf("expected message %q in output; got %q", msg, output)
		}

		for _, pair := range expectedPairs {
			if !strings.Contains(output, pair) {
				t.Fatalf("expected %q in output; got %q", pair, output)
			}
		}
	}

	t.Run("simple key-value pairs", func(t *testing.T) {
		f("test", []any{"key", "value"}, []string{"key=value"})
	})

	t.Run("multiple pairs", func(t *testing.T) {
		f("test", []any{"k1", "v1", "k2", "v2"}, []string{"k1=v1", "k2=v2"})
	})

	t.Run("values with spaces are quoted", func(t *testing.T) {
		f("test", []any{"key", "value with spaces"}, []string{`key="value with spaces"`})
	})

	t.Run("numeric values", func(t *testing.T) {
		f("test", []any{"count", 42, "ratio", 3.14}, []string{"count=42", "ratio=3.14"})
	})

	t.Run("trailing key is ignored", func(t *testing.T) {
		var buf bytes.Buffer
		SetOutput(&buf)
		defer ResetOutput()

		Info("test", "key1", "value1", "key2")
		output := buf.String()

		if !strings.Contains(output, "key1=value1") {
			t.Fatalf("expected key1=value1 in output; got %q", output)
		}
		// key2 without value should not appear as key2=
		if strings.Contains(output, "key2=") {
			t.Fatalf("unexpected key2= in output; got %q", output)
		}
	})

	t.Run("empty key is skipped", func(t *testing.T) {
		var buf bytes.Buffer
		SetOutput(&buf)
		defer ResetOutput()

		Info("test", "", "value", "valid", "ok")
		output := buf.String()

		if !strings.Contains(output, "valid=ok") {
			t.Fatalf("expected valid=ok in output; got %q", output)
		}
	})
}

func TestTimestamp(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	oldTZ := *loggerTimezone
	*loggerTimezone = "UTC"
	defer func() { *loggerTimezone = oldTZ }()
	initTimezone()

	Info("test")
	output := buf.String()

	// Check timestamp format YYYY-MM-DDTHH:MM:SS.mmm+00:00 (or -07:00)
	if !strings.Contains(output, "T") {
		t.Fatalf("expected ISO8601 timestamp with T separator; got %q", output)
	}
	// UTC should produce +00:00 offset
	if !strings.Contains(output, "+00:00") && !strings.Contains(output, "-00:00") {
		t.Fatalf("expected timezone offset in timestamp; got %q", output)
	}
}

func TestCaller(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	Info("test")
	output := buf.String()

	// Should contain file:line
	if !strings.Contains(output, "logger_test.go:") {
		t.Fatalf("expected caller info with file:line; got %q", output)
	}
}

func TestValidateLevel(t *testing.T) {
	f := func(level string, shouldPanic bool) {
		t.Helper()

		oldLevel := *loggerLevel
		*loggerLevel = level
		defer func() { *loggerLevel = oldLevel }()

		defer func() {
			r := recover()
			if shouldPanic && r == nil {
				t.Fatalf("expected panic for level=%s", level)
			}
			if !shouldPanic && r != nil {
				t.Fatalf("unexpected panic for level=%s: %v", level, r)
			}
		}()

		validateLevel()
	}

	t.Run("valid levels", func(t *testing.T) {
		f("INFO", false)
		f("WARN", false)
		f("ERROR", false)
		f("FATAL", false)
		f("PANIC", false)
	})

	t.Run("invalid level", func(t *testing.T) {
		f("INVALID", true)
		f("DEBUG", true)
	})
}

func TestSetOutput(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	Info("test message")

	if buf.Len() == 0 {
		t.Fatal("expected output to be written to buffer")
	}

	if !strings.Contains(buf.String(), "test message") {
		t.Fatalf("expected message in buffer; got %q", buf.String())
	}
}

func TestLogWriter(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	lw := &logWriter{}
	n, err := lw.Write([]byte("test log line\n"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len("test log line\n") {
		t.Fatalf("expected n=%d; got %d", len("test log line\n"), n)
	}

	output := buf.String()
	if !strings.Contains(output, "test log line") {
		t.Fatalf("expected message in output; got %q", output)
	}
	if !strings.Contains(output, "error") {
		t.Fatalf("expected error level in output; got %q", output)
	}
}

func TestInitTimezone(t *testing.T) {
	f := func(tz string) {
		t.Helper()

		oldTZ := *loggerTimezone
		*loggerTimezone = tz
		defer func() { *loggerTimezone = oldTZ }()

		initTimezone()

		// Verify timezone was set
		if timezone == nil {
			t.Fatalf("expected timezone to be set for %s", tz)
		}
	}

	t.Run("valid timezones", func(t *testing.T) {
		f("UTC")
		f("America/New_York")
		f("Europe/Paris")
	})

	// Note: invalid timezone causes log.Fatalf which exits the process
	// Cannot test this without subprocess testing
}

func TestInit(t *testing.T) {
	// Save original flags
	oldLevel := *loggerLevel
	oldOutput := *loggerOutput
	oldTZ := *loggerTimezone
	defer func() {
		*loggerLevel = oldLevel
		*loggerOutput = oldOutput
		*loggerTimezone = oldTZ
		initInternal() // restore state
	}()

	*loggerLevel = "WARN"
	*loggerOutput = "stdout"
	*loggerTimezone = "UTC"

	Init()

	// Verify init worked by checking that WARN level filtering works
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	Info("should not appear")
	if buf.Len() > 0 {
		t.Fatal("expected INFO to be filtered at WARN level")
	}

	buf.Reset()
	Warn("should appear")
	if buf.Len() == 0 {
		t.Fatal("expected WARN to be logged at WARN level")
	}
}
