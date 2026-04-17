package remoteclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Request mirrors cc-remote-agent's ExecuteRequest
type Request struct {
	Prompt              string    `json:"prompt"`
	SessionID           string    `json:"session_id,omitempty"`
	Model               string    `json:"model,omitempty"`
	SystemPrompt        string    `json:"system_prompt,omitempty"`
	ConversationHistory []Message `json:"conversation_history,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent is one line of cc-remote-agent ndjson output
type StreamEvent struct {
	Type    string `json:"type"`
	SubType string `json:"subtype,omitempty"`
	// assistant event fields
	Message *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	} `json:"message,omitempty"`
	// result event fields
	Result    string  `json:"result,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
	CostUSD   float64 `json:"total_cost_usd,omitempty"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Execute calls cc-remote-agent /execute and streams ndjson events to the callback.
// Returns the session_id from the result event (for --resume in next call).
func (c *Client) Execute(ctx context.Context, req Request, onEvent func(StreamEvent)) (sessionID string, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/execute", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cc-remote-agent returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var event StreamEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // 不正な行はスキップ
		}
		onEvent(event)
		if event.Type == "result" {
			sessionID = event.SessionID
		}
	}
	return sessionID, scanner.Err()
}
