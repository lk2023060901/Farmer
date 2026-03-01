package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientConfig holds LLM API configuration.
type ClientConfig struct {
	BaseURL string        // e.g., "https://api.openai.com/v1" or any compatible endpoint
	APIKey  string
	Model   string        // e.g., "gpt-4o-mini" or "claude-haiku-4-5-20251001"
	Timeout time.Duration
}

// DefaultClientConfig returns config for a no-op/mock LLM (safe when no API key is set).
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  "",
		Model:   "gpt-4o-mini",
		Timeout: 8 * time.Second,
	}
}

// message mirrors the OpenAI chat message format.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float32   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Client is an OpenAI-compatible LLM client.
type Client struct {
	cfg  ClientConfig
	http *http.Client
}

// NewClient constructs an LLM Client.
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ErrNoAPIKey is returned when no API key is configured.
var ErrNoAPIKey = fmt.Errorf("LLM API key not configured")

// ErrTimeout is returned when the LLM request times out.
var ErrTimeout = fmt.Errorf("LLM request timed out")

// Complete sends a chat completion request and returns the assistant's text reply.
func (c *Client) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.cfg.APIKey == "" {
		return "", ErrNoAPIKey
	}

	payload := chatRequest{
		Model: c.cfg.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   200,
		Temperature: 0.8,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, raw)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return cr.Choices[0].Message.Content, nil
}
