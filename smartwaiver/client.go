package smartwaiver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	BaseURL    = "https://api.smartwaiver.com"
	APIVersion = "v4"
)

// Client is a Smartwaiver API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config holds Smartwaiver client configuration.
type Config struct {
	APIKey  string
	BaseURL string // optional override, defaults to BaseURL
}

// NewClient creates a Smartwaiver API client.
// APIKey is required; get one at https://app.smartwaiver.com/user/api/api-keys
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("smartwaiver: API key is required")
	}
	base := BaseURL
	if cfg.BaseURL != "" {
		base = cfg.BaseURL
	}
	return &Client{
		apiKey:  cfg.APIKey,
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// do sends an authenticated request and decodes the JSON response into v.
func (c *Client) do(method, path string, params url.Values, body io.Reader, v any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("smartwaiver API error %d: %s", resp.StatusCode, string(b))
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// Ping checks API connectivity. Does not require authentication.
func (c *Client) Ping() (string, error) {
	u, err := url.Parse(c.baseURL + "/ping")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ListWaiversOptions are optional filters for listing waivers.
// See https://api.smartwaiver.com/api/docs#waivers-GETv4-waivers
// FilterSince: when set, filter client-side to waivers created on/after this time (avoids API fromDts issues).
type ListWaiversOptions struct {
	Limit       int       // 1-300, default 20
	Offset      int       // pagination offset
	TemplateID  string    // limit to template
	FromDts     string    // ISO 8601 date (API filter - may be unreliable)
	ToDts       string    // ISO 8601 date
	FilterSince *time.Time // client-side: only waivers created on/after this time
	FirstName   string
	LastName    string
	Tag         string
	Verified    string // "true" or "false"
	EventData   string // "true" or "false"
}

// WaiverSummary is a waiver from the list endpoint (partial data).
type WaiverSummary struct {
	WaiverID       string   `json:"waiverId"`
	TemplateID     string   `json:"templateId"`
	Title          string   `json:"title"`
	CreatedOn      string   `json:"createdOn"`
	ExpirationDate string   `json:"expirationDate"`
	Expired        bool     `json:"expired"`
	Verified       bool     `json:"verified"`
	Kiosk          bool     `json:"kiosk"`
	FirstName      string   `json:"firstName"`
	MiddleName     string   `json:"middleName"`
	LastName       string   `json:"lastName"`
	DOB            string   `json:"dob"`
	IsMinor        bool     `json:"isMinor"`
	AutoTag        string   `json:"autoTag"`
	Tags           []string `json:"tags"`
	PrefillID      string   `json:"prefillId,omitempty"`
}

// WaiversListResponse is the response from GET v4/waivers.
type WaiversListResponse struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Ts      string          `json:"ts"`
	Type    string          `json:"type"`
	Waivers []WaiverSummary `json:"waivers"`
}

// ListWaivers fetches a list of signed waivers.
// Uses GET v4/waivers with optional filters.
func (c *Client) ListWaivers(opts ListWaiversOptions) (*WaiversListResponse, error) {
	params := url.Values{}
	if opts.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.TemplateID != "" {
		params.Set("templateId", opts.TemplateID)
	}
	if opts.FromDts != "" {
		params.Set("fromDts", opts.FromDts)
	}
	if opts.ToDts != "" {
		params.Set("toDts", opts.ToDts)
	}
	if opts.FirstName != "" {
		params.Set("firstName", opts.FirstName)
	}
	if opts.LastName != "" {
		params.Set("lastName", opts.LastName)
	}
	if opts.Tag != "" {
		params.Set("tag", opts.Tag)
	}
	if opts.Verified != "" {
		params.Set("verified", opts.Verified)
	}
	if opts.EventData != "" {
		params.Set("eventData", opts.EventData)
	}

	var out WaiversListResponse
	if err := c.do(http.MethodGet, "/v4/waivers", params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WaiverResponse is the response from GET v4/waivers/{waiverId}.
// Waiver holds the full waiver JSON (participants, email, address, etc.).
type WaiverResponse struct {
	Version int             `json:"version"`
	ID      string          `json:"id"`
	Ts      string          `json:"ts"`
	Type    string          `json:"type"`
	Waiver  json.RawMessage `json:"waiver"`
}

// GetWaiver fetches a full waiver by ID.
// Uses GET v4/waivers/{waiverId} with pdf=false.
func (c *Client) GetWaiver(waiverID string) (*WaiverResponse, error) {
	params := url.Values{}
	params.Set("pdf", "false")

	var out WaiverResponse
	path := "/v4/waivers/" + url.PathEscape(waiverID)
	if err := c.do(http.MethodGet, path, params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Version returns the API version. Does not require authentication.
func (c *Client) Version() (string, error) {
	u, err := url.Parse(c.baseURL + "/version")
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
