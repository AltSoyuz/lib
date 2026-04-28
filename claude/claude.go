package claude

import (
	"bufio"
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

const apiURL = "https://api.anthropic.com/v1/messages"

const (
	ModelHaiku = "claude-haiku-4-5"
)

type MessageRequest struct {
	Model       string    `json:"-"`
	MaxTokens   int       `json:"-"`
	System      string    `json:"-"`
	CacheSystem bool      `json:"-"`
	Messages    []Message `json:"-"`
	Stream      bool      `json:"-"`
}

// MarshalJSON serializes the request, using a structured system block with
// cache_control when CacheSystem is true.
func (r MessageRequest) MarshalJSON() ([]byte, error) {
	type wire struct {
		Model     string          `json:"model"`
		MaxTokens int             `json:"max_tokens"`
		System    json.RawMessage `json:"system,omitempty"`
		Messages  []Message       `json:"messages"`
		Stream    bool            `json:"stream,omitempty"`
	}
	w := wire{
		Model:     r.Model,
		MaxTokens: r.MaxTokens,
		Messages:  r.Messages,
		Stream:    r.Stream,
	}
	if r.System != "" {
		var err error
		if r.CacheSystem {
			type cacheControl struct {
				Type string `json:"type"`
			}
			type systemBlock struct {
				Type         string       `json:"type"`
				Text         string       `json:"text"`
				CacheControl cacheControl `json:"cache_control"`
			}
			w.System, err = json.Marshal([]systemBlock{{
				Type:         "text",
				Text:         r.System,
				CacheControl: cacheControl{Type: "ephemeral"},
			}})
		} else {
			w.System, err = json.Marshal(r.System)
		}
		if err != nil {
			return nil, err
		}
	}
	type plain wire
	return json.Marshal(plain(w))
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messageResponse struct {
	Content []contentBlock `json:"content"`
	Usage   usage          `json:"usage"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete sends a message request and returns the text response.
func (c *Client) Complete(ctx context.Context, req MessageRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("claude marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if req.CacheSystem {
		httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("claude call: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Info("claude close body error", "err", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("claude read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude status %d: %s", resp.StatusCode, respBody)
	}

	var msgResp messageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return "", fmt.Errorf("claude decode: %w", err)
	}

	logger.Info("claude complete",
		"model", req.Model,
		"input_tokens", msgResp.Usage.InputTokens,
		"output_tokens", msgResp.Usage.OutputTokens,
		"cache_creation_tokens", msgResp.Usage.CacheCreationInputTokens,
		"cache_read_tokens", msgResp.Usage.CacheReadInputTokens,
	)

	var text string
	for _, block := range msgResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text, nil
}

type streamDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type streamEvent struct {
	Type    string      `json:"type"`
	Delta   streamDelta `json:"delta"`
	Message struct {
		Usage usage `json:"usage"`
	} `json:"message"`
	Usage usage `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Stream sends a streaming message request and invokes onDelta for each text delta.
// It returns when the stream ends (message_stop), the context is canceled, or an error occurs.
func (c *Client) Stream(ctx context.Context, req MessageRequest, onDelta func(string) error) error {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("claude marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("claude request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if req.CacheSystem {
		httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
	}

	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("claude call: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Info("claude stream close body error", "err", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("claude status %d: %s", resp.StatusCode, respBody)
	}

	var inputTokens, outputTokens, cacheCreation, cacheRead int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var ev streamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return fmt.Errorf("claude decode event: %w", err)
		}

		switch ev.Type {
		case "message_start":
			inputTokens = ev.Message.Usage.InputTokens
			cacheCreation = ev.Message.Usage.CacheCreationInputTokens
			cacheRead = ev.Message.Usage.CacheReadInputTokens
		case "content_block_delta":
			if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
				if err := onDelta(ev.Delta.Text); err != nil {
					return err
				}
			}
		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				outputTokens = ev.Usage.OutputTokens
			}
		case "message_stop":
			logger.Info("claude stream",
				"model", req.Model,
				"input_tokens", inputTokens,
				"output_tokens", outputTokens,
				"cache_creation_tokens", cacheCreation,
				"cache_read_tokens", cacheRead,
			)
			return nil
		case "error":
			return fmt.Errorf("claude stream error: %s: %s", ev.Error.Type, ev.Error.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("claude stream read: %w", err)
	}
	return nil
}
