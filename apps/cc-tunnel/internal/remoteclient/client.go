package remoteclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// ErrCredentialsNotReady is returned by FinalizeCredentials when the login
// flow has not yet completed and credentials.json does not exist on the remote agent.
var ErrCredentialsNotReady = errors.New("credentials file not ready (login not completed)")

// AuthStatus mirrors cc-remote-agent's auth status response
type AuthStatus struct {
	LoggedIn         bool    `json:"loggedIn"`
	AuthMethod       string  `json:"authMethod"`
	LoginPending     bool    `json:"loginPending"`
	ApiProvider      *string `json:"apiProvider,omitempty"`
	Email            *string `json:"email,omitempty"`
	OrgName          *string `json:"orgName,omitempty"`
	SubscriptionType *string `json:"subscriptionType,omitempty"`
	ApiKeySource     *string `json:"apiKeySource,omitempty"`
	LoginUrl         *string `json:"loginUrl,omitempty"`
}

// LoginResponse mirrors cc-remote-agent's login response
type LoginResponse struct {
	Message  string  `json:"message"`
	LoginUrl *string `json:"loginUrl,omitempty"`
	LoggedIn *bool   `json:"loggedIn,omitempty"`
}

// Request mirrors cc-remote-agent's ExecuteRequest
type Request struct {
	Prompt                 string    `json:"prompt"`
	SessionID              string    `json:"session_id,omitempty"`
	ConversationID         string    `json:"conversation_id,omitempty"` // for per-session container routing
	Model                  string    `json:"model,omitempty"`
	SystemPrompt           string    `json:"system_prompt,omitempty"`
	ConversationHistory    []Message `json:"conversation_history,omitempty"`
	IncludePartialMessages bool      `json:"include_partial_messages,omitempty"`
	IncludeHookEvents      bool      `json:"include_hook_events,omitempty"`
	Credentials            []byte    `json:"-"` // decrypted credentials JSON; not sent to cc-remote-agent
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent is one line of cc-remote-agent ndjson output
type StreamEvent struct {
	Type      string `json:"type"`
	SubType   string `json:"subtype,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// type=assistant 用（既存）
	Message *struct {
		Content []ContentBlock `json:"content"`
	} `json:"message,omitempty"`

	// type=stream_event 用（新規）
	Event json.RawMessage `json:"event,omitempty"`

	// type=rate_limit_event 用（新規）
	RateLimitInfo *RateLimitInfo `json:"rate_limit_info,omitempty"`

	// type=result 用（拡張）
	Result     string  `json:"result,omitempty"`
	IsError    bool    `json:"is_error,omitempty"`
	CostUSD    float64 `json:"total_cost_usd,omitempty"`
	DurationMs int64   `json:"duration_ms,omitempty"`

	// type=system, subtype=init 用（新規）
	Model string `json:"model,omitempty"`

	// type=system, subtype=hook_* 用（新規）
	HookID    string `json:"hook_id,omitempty"`
	HookName  string `json:"hook_name,omitempty"`
	HookEvent string `json:"hook_event,omitempty"`
}

type RateLimitInfo struct {
	Status   string `json:"status"`
	ResetsAt int64  `json:"resetsAt"`
	Type     string `json:"rateLimitType"`
}

// ContentBlock represents a content block in an assistant message
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	// tool_use fields
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// tool_result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
}

// InnerEvent is the inner event inside stream_event's Event field
type InnerEvent struct {
	Type         string          `json:"type"`
	Index        int             `json:"index,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
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

// GetAuthStatus calls cc-remote-agent GET /auth/status and returns the auth status.
func (c *Client) GetAuthStatus(ctx context.Context) (*AuthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/auth/status", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cc-remote-agent returned %d", resp.StatusCode)
	}
	var status AuthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// InitiateLogin calls cc-remote-agent POST /auth/login and returns the login response.
func (c *Client) InitiateLogin(ctx context.Context, method string) (*LoginResponse, error) {
	bodyData, err := json.Marshal(map[string]string{"method": method})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login", bytes.NewReader(bodyData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cc-remote-agent returned %d: %s", resp.StatusCode, body)
	}
	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, err
	}
	return &loginResp, nil
}

// AuthInputRequest mirrors cc-remote-agent's auth input request
type AuthInputRequest struct {
	Input string `json:"input"`
}

// AuthInputResponse mirrors cc-remote-agent's auth input response
type AuthInputResponse struct {
	Message string `json:"message"`
}

// AuthOutputResponse mirrors cc-remote-agent's auth output response
type AuthOutputResponse struct {
	Data   string `json:"data"`
	Cursor int    `json:"cursor"`
}

// AuthCancelResponse mirrors cc-remote-agent's auth cancel response
type AuthCancelResponse struct {
	Message string `json:"message"`
}

// SubmitAuthInput calls cc-remote-agent POST /auth/input with arbitrary stdin input.
func (c *Client) SubmitAuthInput(ctx context.Context, input string) (*AuthInputResponse, error) {
	bodyData, err := json.Marshal(AuthInputRequest{Input: input})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/input", bytes.NewReader(bodyData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("submit auth input: status %d: %s", resp.StatusCode, body)
	}
	var result AuthInputResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAuthOutput calls cc-remote-agent GET /auth/output?since=N and returns new stdout lines.
func (c *Client) GetAuthOutput(ctx context.Context, since int) (*AuthOutputResponse, error) {
	url := fmt.Sprintf("%s/auth/output?since=%d", c.baseURL, since)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get auth output: status %d", resp.StatusCode)
	}
	var result AuthOutputResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelLogin calls cc-remote-agent POST /auth/cancel to kill the login PTY process.
func (c *Client) CancelLogin(ctx context.Context) (*AuthCancelResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/cancel", http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cancel login: status %d", resp.StatusCode)
	}
	var result AuthCancelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Logout calls cc-remote-agent POST /auth/logout and returns the updated auth status.
func (c *Client) Logout(ctx context.Context) (*AuthStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/logout", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cc-remote-agent returned %d", resp.StatusCode)
	}
	var status AuthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// InitCredentials calls cc-remote-agent POST /init to write credentials to the container.
func (c *Client) InitCredentials(ctx context.Context, credJSON []byte) error {
	bodyData, err := json.Marshal(map[string]string{"credentialsJson": string(credJSON)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/init", bytes.NewReader(bodyData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("init credentials: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// FinalizeCredentials calls cc-remote-agent POST /auth/finalize-credentials and returns
// the raw credentials JSON string. Returns ErrCredentialsNotReady when the remote agent
// responds with 404 (login not yet completed).
func (c *Client) FinalizeCredentials(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/finalize-credentials", nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrCredentialsNotReady
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var body struct {
		CredentialsJSON string `json:"credentialsJson"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if body.CredentialsJSON == "" {
		return "", errors.New("empty credentialsJson in response")
	}
	return body.CredentialsJSON, nil
}

// Execute calls cc-remote-agent /execute and streams ndjson events to the callback.
// Returns the session_id from the result event (for --resume in next call).
func (c *Client) Execute(ctx context.Context, req Request, onEvent func(StreamEvent)) (string, error) {
	var sessionID string
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	executeURL := c.baseURL + "/execute"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		executeURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	slog.Info("remoteclient execute", "url", executeURL)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("resp.Body.Close failed", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Error("remoteclient execute non-200", "status", resp.StatusCode)
		return "", fmt.Errorf("cc-remote-agent returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var event StreamEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			slog.Error("remoteclient ndjson parse error", "err", err, "line", scanner.Text())
			continue
		}
		onEvent(event)
		if event.Type == "result" {
			sessionID = event.SessionID
		}
	}
	slog.Info("remoteclient execute completed")
	return sessionID, scanner.Err()
}
