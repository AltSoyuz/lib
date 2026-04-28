package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/logger"
)

const apiBase = "https://api.telegram.org/bot"

// Update is a Telegram getUpdates item.
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is the message payload attached to a Telegram update.
type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	From      *User  `json:"from"`
	Text      string `json:"text"`
}

// Chat identifies the Telegram chat where a message was sent.
type Chat struct {
	ID int64 `json:"id"`
}

// User identifies a Telegram user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Client is a Telegram Bot API client.
type Client struct {
	token      string
	httpClient *http.Client
	allowed    map[int64]bool
}

// New creates a Telegram bot client. allowedUserIDs restricts who can interact
// with the bot. If empty, all users are allowed.
func New(token string, allowedUserIDs []int64) *Client {
	allowed := make(map[int64]bool, len(allowedUserIDs))
	for _, id := range allowedUserIDs {
		allowed[id] = true
	}
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		allowed:    allowed,
	}
}

// IsAllowed returns true if the user is allowed to use the bot.
func (c *Client) IsAllowed(userID int64) bool {
	if len(c.allowed) == 0 {
		return true
	}
	return c.allowed[userID]
}

// SendMessage sends a text message to a chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url("sendMessage"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send status %d: %s", resp.StatusCode, b)
	}
	return nil
}

// Poll starts long-polling getUpdates and dispatches each message to handler.
// Blocks until ctx is cancelled.
func (c *Client) Poll(ctx context.Context, handler func(context.Context, Update)) {
	var offset int
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := c.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("telegram poll", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			handler(ctx, u)
		}
	}
}

func (c *Client) getUpdates(ctx context.Context, offset int) ([]Update, error) {
	payload := map[string]any{
		"offset":  offset,
		"timeout": 30,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("telegram marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url("getUpdates"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram getUpdates status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("telegram decode: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram getUpdates not ok")
	}
	return result.Result, nil
}

func (c *Client) url(method string) string {
	return apiBase + c.token + "/" + method
}

// ParseCommand extracts the command and arguments from a message text.
// Returns empty strings if the message is not a command.
// Example: "/anki some text here" → "anki", "some text here"
func ParseCommand(text string) (cmd, args string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}
	text = text[1:] // strip leading /
	// Handle /command@botname format
	parts := strings.SplitN(text, " ", 2)
	cmd = strings.SplitN(parts[0], "@", 2)[0]
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args
}
