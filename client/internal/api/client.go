package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Heartbeat struct {
	Entity           string         `json:"entity"`
	Type             string         `json:"type,omitempty"`
	Project          string         `json:"project,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Language         string         `json:"language,omitempty"`
	AgentType        string         `json:"agent_type,omitempty"`
	Time             float64        `json:"time"`
	Duration         float64        `json:"duration,omitempty"`
	SessionStartedAt *time.Time     `json:"session_started_at,omitempty"`
	SessionEndedAt   *time.Time     `json:"session_ended_at,omitempty"`
	SessionDurationSeconds *int     `json:"session_duration_seconds,omitempty"`
	AgentActiveSeconds *int         `json:"agent_active_seconds,omitempty"`
	HumanActiveSeconds *int         `json:"human_active_seconds,omitempty"`
	IdleSeconds      *int           `json:"idle_seconds,omitempty"`
	IsWrite          bool           `json:"is_write,omitempty"`
	TokensUsed       int            `json:"tokens_used,omitempty"`
	LinesAdded       int            `json:"lines_added,omitempty"`
	LinesDeleted     int            `json:"lines_deleted,omitempty"`
	CostUSD          float64        `json:"cost_usd,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Machine          string         `json:"machine,omitempty"`
	OperatingSystem  string         `json:"operating_system,omitempty"`

	// Conversation tracking
	ConversationID   string         `json:"conversation_id,omitempty"`
	MessageID        string         `json:"message_id,omitempty"`
	PromptTokens     int            `json:"prompt_tokens,omitempty"`
	CompletionTokens int            `json:"completion_tokens,omitempty"`
	TotalTokens      int            `json:"total_tokens,omitempty"`
	Model            string         `json:"model,omitempty"`
}

type DeviceAuthorization struct {
	UserCode            string
	VerificationURI     string
	VerificationURL     string
	DeviceCode          string
	PollURL             string
	IntervalSeconds     int
	ExpiresInSeconds    int
	AuthenticatedEmail  string
	AuthenticatedName   string
	AuthenticatedUserID string
}

type DeviceAuthorizationStatus struct {
	State               string
	APIKey              string
	AuthenticatedEmail  string
	AuthenticatedName   string
	AuthenticatedUserID string
}

type CurrentUser struct {
	ID       string
	Email    string
	Name     string
	Timezone string
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) SendHeartbeats(ctx context.Context, heartbeats []Heartbeat) error {
	if len(heartbeats) == 0 {
		return nil
	}

	payload, err := json.Marshal(heartbeats)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/heartbeats/bulk", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	res, err := c.do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return responseError(res)
	}
	return nil
}

func (c *Client) CurrentUser(ctx context.Context) (CurrentUser, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/users/current", nil)
	if err != nil {
		return CurrentUser{}, err
	}

	res, err := c.do(req)
	if err != nil {
		return CurrentUser{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return CurrentUser{}, responseError(res)
	}

	body, err := decodeEnvelopeObject(res.Body)
	if err != nil {
		return CurrentUser{}, err
	}

	return CurrentUser{
		ID:       getString(body, "id"),
		Email:    getString(body, "email"),
		Name:     getString(body, "name"),
		Timezone: getString(body, "timezone"),
	}, nil
}

func (c *Client) CreateDeviceAuthorization(ctx context.Context, machineName string) (DeviceAuthorization, error) {
	payload, err := json.Marshal(map[string]any{
		"machine_name": machineName,
	})
	if err != nil {
		return DeviceAuthorization{}, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/device_authorizations", bytes.NewReader(payload))
	if err != nil {
		return DeviceAuthorization{}, err
	}

	res, err := c.do(req)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return DeviceAuthorization{}, responseError(res)
	}

	body, err := decodeEnvelopeObject(res.Body)
	if err != nil {
		return DeviceAuthorization{}, err
	}

	user := getNestedObject(body, "user")
	verificationURI := getString(body, "verification_uri", "verification_url", "approval_url")

	return DeviceAuthorization{
		UserCode:            getString(body, "user_code"),
		VerificationURI:     verificationURI,
		VerificationURL:     verificationURI,
		DeviceCode:          getString(body, "device_code"),
		PollURL:             getString(body, "poll_url"),
		IntervalSeconds:     getInt(body, "interval", "interval_seconds", "poll_interval"),
		ExpiresInSeconds:    getInt(body, "expires_in", "expires_in_seconds"),
		AuthenticatedEmail:  getString(user, "email"),
		AuthenticatedName:   getString(user, "name"),
		AuthenticatedUserID: getString(user, "id"),
	}, nil
}

func (c *Client) PollDeviceAuthorization(ctx context.Context, auth DeviceAuthorization) (DeviceAuthorizationStatus, error) {
	if auth.PollURL != "" {
		if status, err := c.pollDeviceAuthorizationURL(ctx, auth.PollURL); err == nil {
			return status, nil
		}
	}

	payload, err := json.Marshal(map[string]any{
		"device_code": auth.DeviceCode,
	})
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/device_authorizations/poll", bytes.NewReader(payload))
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}

	res, err := c.do(req)
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}
	defer res.Body.Close()

	return decodeDeviceAuthorizationPollResponse(res)
}

func (c *Client) pollDeviceAuthorizationURL(ctx context.Context, rawURL string) (DeviceAuthorizationStatus, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}

	if !parsed.IsAbs() {
		base, err := url.Parse(c.BaseURL)
		if err != nil {
			return DeviceAuthorizationStatus{}, err
		}
		parsed = base.ResolveReference(parsed)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.do(req)
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}
	defer res.Body.Close()

	return decodeDeviceAuthorizationPollResponse(res)
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("base URL is not configured")
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}

	base.Path = path.Join(base.Path, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, base.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return httpClient.Do(req)
}

func decodeDeviceAuthorizationStatus(r io.Reader) (DeviceAuthorizationStatus, error) {
	body, err := decodeEnvelopeObject(r)
	if err != nil {
		return DeviceAuthorizationStatus{}, err
	}

	user := getNestedObject(body, "user")
	status := getString(body, "status", "state")
	if status == "" && getString(body, "api_key", "token") != "" {
		status = "approved"
	}

	return DeviceAuthorizationStatus{
		State:               status,
		APIKey:              getString(body, "api_key", "token"),
		AuthenticatedEmail:  getString(user, "email"),
		AuthenticatedName:   getString(user, "name"),
		AuthenticatedUserID: getString(user, "id"),
	}, nil
}

func decodeDeviceAuthorizationPollResponse(res *http.Response) (DeviceAuthorizationStatus, error) {
	body, readErr := io.ReadAll(io.LimitReader(res.Body, 4*1024))
	if readErr != nil {
		return DeviceAuthorizationStatus{}, readErr
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return decodeDeviceAuthorizationStatus(bytes.NewReader(body))
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		if data, ok := parsed["data"].(map[string]any); ok {
			parsed = data
		}

		if state := getString(parsed, "error"); state != "" {
			return DeviceAuthorizationStatus{State: state}, nil
		}
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(res.StatusCode)
	}
	return DeviceAuthorizationStatus{}, fmt.Errorf("request failed: %s: %s", res.Status, message)
}

func decodeEnvelopeObject(r io.Reader) (map[string]any, error) {
	var body map[string]any
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return nil, err
	}

	if data, ok := body["data"].(map[string]any); ok {
		return data, nil
	}
	return body, nil
}

func responseError(res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4*1024))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(res.StatusCode)
	}
	return fmt.Errorf("request failed: %s: %s", res.Status, message)
}

func getNestedObject(body map[string]any, key string) map[string]any {
	if nested, ok := body[key].(map[string]any); ok {
		return nested
	}
	return map[string]any{}
}

func getString(body map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := body[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case int:
			return strconv.Itoa(typed)
		case json.Number:
			return typed.String()
		}
	}
	return ""
}

func getInt(body map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := body[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
		case string:
			if parsed, err := strconv.Atoi(typed); err == nil {
				return parsed
			}
		}
	}
	return 0
}
