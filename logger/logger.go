package logger

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	loggerLevel    = flag.String("logger.level", "INFO", "Log level: INFO, WARN, ERROR, FATAL, PANIC")
	loggerOutput   = flag.String("logger.output", "stdout", "Log output: stdout, stderr")
	loggerTimezone = flag.String("logger.timezone", "UTC", "Timezone (IANA) for timestamps")
)

var (
	timezone           = time.UTC
	out      io.Writer = os.Stdout
	mu       sync.Mutex
)

// Init applies logger flags. Call it after flag parsing.
func Init() {
	initInternal()
}

func initInternal() {
	setOutput()
	validateLevel()
	initTimezone()
	log.SetOutput(out)
	log.SetFlags(0)
}

// Info logs an info-level message.
func Info(msg string, kv ...any) { printLogSkipframes(0, "INFO", msg, kv...) }

// Warn logs a warning-level message.
func Warn(msg string, kv ...any) { printLogSkipframes(0, "WARN", msg, kv...) }

// Error logs an error-level message.
func Error(msg string, kv ...any) { printLogSkipframes(0, "ERROR", msg, kv...) }

// Fatal logs a fatal-level message and exits the process.
func Fatal(msg string, kv ...any) { printLogSkipframes(0, "FATAL", msg, kv...) }

// Panic logs a panic-level message and panics.
func Panic(msg string, kv ...any) { printLogSkipframes(0, "PANIC", msg, kv...) }

// InfoSkipframes logs info message and skips the given number of frames for the caller.
func InfoSkipframes(skipframes int, msg string, kv ...any) {
	printLogSkipframes(skipframes, "INFO", msg, kv...)
}

// WarnSkipframes logs warn message and skips the given number of frames for the caller.
func WarnSkipframes(skipframes int, msg string, kv ...any) {
	printLogSkipframes(skipframes, "WARN", msg, kv...)
}

// ErrorSkipframes logs error message and skips the given number of frames for the caller.
func ErrorSkipframes(skipframes int, msg string, kv ...any) {
	printLogSkipframes(skipframes, "ERROR", msg, kv...)
}

// FatalSkipframes logs fatal message and terminates the app.
func FatalSkipframes(skipframes int, msg string, kv ...any) {
	printLogSkipframes(skipframes, "FATAL", msg, kv...)
}

// PanicSkipframes logs panic message and panics.
func PanicSkipframes(skipframes int, msg string, kv ...any) {
	printLogSkipframes(skipframes, "PANIC", msg, kv...)
}

var stdErrorLogger = log.New(&logWriter{}, "", 0)

// StdErrorLogger returns a stdlib logger that writes through Error.
func StdErrorLogger() *log.Logger { return stdErrorLogger }

type logWriter struct{}

func (lw *logWriter) Write(p []byte) (int, error) {
	s := strings.TrimSuffix(string(p), "\n")
	if s != "" {
		Error(s)
	}
	return len(p), nil
}

func printLogSkipframes(skipframes int, level, msg string, kv ...any) {
	if skip(level) {
		return
	}
	ts := time.Now().In(timezone).Format("2006-01-02T15:04:05.000-07:00")
	// 3 is the base: printLogSkipframes -> caller of *Skipframes -> actual caller we want
	// Additional skipframes moves further up the stack
	caller := callerAt(3 + skipframes)

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s %s", ts, strings.ToLower(level), caller, msg)

	// key=value pairs; ignore trailing odd key
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok || k == "" {
			continue
		}
		vs := fmt.Sprint(kv[i+1])
		if vs == "" {
			continue
		}
		if strings.ContainsAny(vs, " \t") {
			vs = `"` + vs + `"`
		}
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(vs)
	}

	line := b.String()
	mu.Lock()
	if _, err := fmt.Fprintln(out, line); err != nil {
		log.Printf("logger: failed to write log line: %v", err)
	}
	mu.Unlock()

	switch level {
	case "FATAL":
		os.Exit(1)
	case "PANIC":
		panic(msg)
	}
}

func skip(level string) bool {
	switch *loggerLevel {
	case "INFO":
		return false
	case "WARN":
		return level == "INFO"
	case "ERROR":
		return level == "INFO" || level == "WARN"
	case "FATAL":
		return level != "FATAL" && level != "PANIC"
	case "PANIC":
		return level != "PANIC"
	default:
		return false
	}
}

func validateLevel() {
	switch *loggerLevel {
	case "INFO", "WARN", "ERROR", "FATAL", "PANIC":
	default:
		panic(fmt.Errorf("unsupported -logger.level=%q", *loggerLevel))
	}
}

func setOutput() {
	switch *loggerOutput {
	case "stdout":
		out = os.Stdout
	case "stderr":
		out = os.Stderr
	default:
		panic(fmt.Errorf("unsupported -logger.output=%q (stdout|stderr)", *loggerOutput))
	}
}

func initTimezone() {
	tz, err := time.LoadLocation(*loggerTimezone)
	if err != nil {
		log.Fatalf("cannot load timezone %q: %v", *loggerTimezone, err)
	}
	timezone = tz
}

func callerAt(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???:0"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// SetOutput changes the logger output writer.
func SetOutput(w io.Writer) { mu.Lock(); out = w; log.SetOutput(out); mu.Unlock() }

// ResetOutput restores stdout as the logger output writer.
func ResetOutput() { SetOutput(os.Stdout) }
