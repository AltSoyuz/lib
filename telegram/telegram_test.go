package telegram

import "testing"

func TestParseCommand(t *testing.T) {
	t.Run("plain command", func(t *testing.T) {
		cmd, args := ParseCommand("/ping")
		if cmd != "ping" || args != "" {
			t.Fatalf("unexpected parse result: cmd=%q args=%q", cmd, args)
		}
	})

	t.Run("command with args", func(t *testing.T) {
		cmd, args := ParseCommand("/anki review the spec")
		if cmd != "anki" || args != "review the spec" {
			t.Fatalf("unexpected parse result: cmd=%q args=%q", cmd, args)
		}
	})

	t.Run("command with bot suffix", func(t *testing.T) {
		cmd, args := ParseCommand("/anki@example_bot cards")
		if cmd != "anki" || args != "cards" {
			t.Fatalf("unexpected parse result: cmd=%q args=%q", cmd, args)
		}
	})

	t.Run("non command", func(t *testing.T) {
		cmd, args := ParseCommand("hello")
		if cmd != "" || args != "" {
			t.Fatalf("unexpected parse result: cmd=%q args=%q", cmd, args)
		}
	})
}
