package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labs-alone/alone-main/internal/utils"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 30 * time.Second
)

// Client manages OpenAI API interactions
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *utils.Logger
	metrics    *Metrics
	mu         sync.RWMutex
}

// ClientConfig holds the configuration for the OpenAI client
type ClientConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// Metrics tracks API usage and performance
type Metrics struct {
	RequestCount   int64
	TokensUsed     int64
	ErrorCount     int64
	AverageLatency time.Duration
	LastRequest    time.Time
	mu            sync.RWMutex
}

// ChatMessage represents a message in the chat completion API
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request to the chat completion API
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
	MaxTokens   int          `json:"max_tokens"`
}

// ChatCompletionResponse represents a response from the chat completion API
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewClient creates a new OpenAI client
func NewClient(config *ClientConfig) (*Client, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &Client{
		apiKey:  config.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:  utils.NewLogger(),
		metrics: &Metrics{},
	}, nil
}

// CreateChatCompletion sends a chat completion request
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	startTime := time.Now()
	defer c.updateMetrics(startTime)

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.incrementErrorCount()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.incrementErrorCount()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.updateTokenUsage(result.Usage.TotalTokens)
	return &result, nil
}

// GetMetrics returns the current metrics
func (c *Client) GetMetrics() Metrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	return *c.metrics
}

// ResetMetrics resets all metrics to zero
func (c *Client) ResetMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics = &Metrics{}
}

func (c *Client) updateMetrics(startTime time.Time) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.RequestCount++
	c.metrics.LastRequest = time.Now()

	latency := time.Since(startTime)
	if c.metrics.RequestCount == 1 {
		c.metrics.AverageLatency = latency
	} else {
		c.metrics.AverageLatency = (c.metrics.AverageLatency + latency) / 2
	}
}

func (c *Client) updateTokenUsage(tokens int) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.TokensUsed += int64(tokens)
}

func (c *Client) incrementErrorCount() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.ErrorCount++
}

// Close performs any necessary cleanup
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}