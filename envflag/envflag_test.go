package envflag

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlagSet(t *testing.T) {
	f := func(envVars map[string]string, args []string, expectedVal string, expectError bool) {
		t.Helper()

		// Setup environment
		for k, v := range envVars {
			if err := os.Setenv(k, v); err != nil {
				t.Fatalf("failed to set env var %q: %v", k, err)
			}
			defer func(key string) {
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("failed to unset env var %q: %v", key, err)
				}
			}(k)
		}

		// Create a new FlagSet to avoid conflicts
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		testFlag := fs.String("test.flag", "default", "test flag")

		err := ParseFlagSetErr(fs, args)
		if expectError && err == nil {
			t.Fatal("expected error but got none")
		}
		if !expectError && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expectError {
			return
		}

		if *testFlag != expectedVal {
			t.Fatalf("unexpected flag value; got %q; want %q", *testFlag, expectedVal)
		}
	}

	t.Run("default value", func(t *testing.T) {
		f(nil, []string{}, "default", false)
	})

	t.Run("cli flag overrides default", func(t *testing.T) {
		f(nil, []string{"-test.flag=cli"}, "cli", false)
	})

	t.Run("env var overrides default", func(t *testing.T) {
		f(map[string]string{"test_flag": "env"}, []string{}, "env", false)
	})

	t.Run("cli flag overrides env var", func(t *testing.T) {
		f(map[string]string{"test_flag": "env"}, []string{"-test.flag=cli"}, "cli", false)
	})

	t.Run("env var with prefix", func(t *testing.T) {
		oldPrefix := *prefix
		*prefix = "APP_"
		defer func() { *prefix = oldPrefix }()

		f(map[string]string{"APP_test_flag": "prefixed"}, []string{}, "prefixed", false)
	})
}

func TestParseFlagSet_BooleanFlags(t *testing.T) {
	f := func(args []string, expectedVal bool, expectError bool) {
		t.Helper()

		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		boolFlag := fs.Bool("enable", false, "enable feature")

		err := ParseFlagSetErr(fs, args)
		if expectError && err == nil {
			t.Fatal("expected error but got none")
		}
		if !expectError && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expectError {
			return
		}

		if *boolFlag != expectedVal {
			t.Fatalf("unexpected flag value; got %v; want %v", *boolFlag, expectedVal)
		}
	}

	t.Run("boolean flag with equals sign", func(t *testing.T) {
		f([]string{"-enable=true"}, true, false)
	})

	t.Run("boolean flag without value", func(t *testing.T) {
		f([]string{"-enable"}, true, false)
	})

	t.Run("boolean flag false", func(t *testing.T) {
		f([]string{"-enable=false"}, false, false)
	})
}

func TestGetEnvFlagName(t *testing.T) {
	f := func(input, testPrefix, expected string) {
		t.Helper()

		oldPrefix := *prefix
		*prefix = testPrefix
		defer func() { *prefix = oldPrefix }()

		result := getEnvFlagName(input)
		if result != expected {
			t.Fatalf("unexpected env flag name; got %q; want %q", result, expected)
		}
	}

	t.Run("simple flag name", func(t *testing.T) {
		f("flag", "", "flag")
	})

	t.Run("flag with dots", func(t *testing.T) {
		f("my.flag.name", "", "my_flag_name")
	})

	t.Run("flag with prefix", func(t *testing.T) {
		f("flag", "APP_", "APP_flag")
	})

	t.Run("flag with dots and prefix", func(t *testing.T) {
		f("my.flag.name", "APP_", "APP_my_flag_name")
	})
}

func TestParseFlagSet_InvalidValue(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Int("number", 0, "number flag")

	if err := os.Setenv("number", "not-a-number"); err != nil {
		t.Fatalf("failed to set env var: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("number"); err != nil {
			t.Fatalf("failed to unset env var: %v", err)
		}
	}()

	err := ParseFlagSetErr(fs, []string{})
	if err == nil {
		t.Fatal("expected error for invalid env var value but got none")
	}
	if got, want := err.Error(), `cannot set flag number to "not-a-number", which is read from env var "number": parse error`; got != want {
		t.Fatalf("unexpected error: got %q, want %q", got, want)
	}
}
