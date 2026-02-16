package mailchimp

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"funbussin/logging"
)

// Client is a Mailchimp Marketing API v3 client.
// Base URL: https://{server}.api.mailchimp.com/3.0/
type Client struct {
	apiKey     string
	server     string
	baseURL    string
	httpClient *http.Client
}

// Config holds Mailchimp client configuration.
// Server is the datacenter prefix (e.g. "us19" from https://us19.admin.mailchimp.com/).
// If empty, it will be parsed from the API key (format: "key-us19").
type Config struct {
	APIKey string
	Server string // e.g. "us19" - optional if API key includes it (key-us19)
}

// NewClient creates a Mailchimp API client.
// API key from https://us1.admin.mailchimp.com/account/api/
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("mailchimp: API key is required")
	}
	server := cfg.Server
	if server == "" {
		if idx := strings.LastIndex(cfg.APIKey, "-"); idx >= 0 && idx < len(cfg.APIKey)-1 {
			server = cfg.APIKey[idx+1:]
		}
	}
	if server == "" {
		return nil, fmt.Errorf("mailchimp: server/datacenter required (e.g. us19) - set MAILCHIMP_SERVER or use API key with suffix like -us19")
	}

	baseURL := "https://" + server + ".api.mailchimp.com/3.0"
	return &Client{
		apiKey:  cfg.APIKey,
		server:  server,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// do sends an authenticated request and decodes the JSON response into v.
// Uses Basic auth: username "anystring", password = API key.
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
	req.SetBasicAuth("anystring", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mailchimp API error %d: %s", resp.StatusCode, string(b))
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// PingResponse is the response from GET /ping.
type PingResponse struct {
	HealthStatus string `json:"health_status"`
}

// Ping checks API connectivity.
func (c *Client) Ping() (string, error) {
	var out PingResponse
	if err := c.do(http.MethodGet, "/ping", nil, nil, &out); err != nil {
		return "", err
	}
	return out.HealthStatus, nil
}

// Contact holds the fields for adding/updating a Mailchimp list member.
// Merge field tags: FN, LN (first/last name), PHONE, C1FN, C1LN, C1_BDAY, C2FN, C2_BDAY, C3FN, C3_BDAY.
// Birthday fields (C1_BDAY, C2_BDAY, C3_BDAY) are normalized to MM/DD/YYYY.
type Contact struct {
	EmailAddress     string `json:"EmailAddress"` // used for subscriber_hash
	FirstName        string
	LastName         string
	Phone            string `json:"Phone"`
	Child1FirstName  string
	Child1LastName   string
	Child1Bday       string // format: MM/DD/YYYY
	Child2FirstName  string
	Child2Bday       string // format: MM/DD/YYYY
	Child3FirstName  string
	Child3Bday       string   // format: MM/DD/YYYY
	MarketingAllowed bool     `json:"MarketingAllowed"`
	Tags             []string `json:"TAGS"`
}

// toBirthdayMMDDYYYY normalizes a date to MM/DD/YYYY format.
// Accepts: MM/DD/YYYY, YYYY-MM-DD, MM-DD-YYYY, etc.
func toBirthdayMMDDYYYY(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// MM/DD/YYYY or MM-DD-YYYY
	if re := regexp.MustCompile(`^(\d{1,2})[/-](\d{1,2})[/-](\d{4})$`); re.MatchString(s) {
		parts := regexp.MustCompile(`[/-]`).Split(s, 3)
		if len(parts) >= 3 {
			return fmt.Sprintf("%s/%s/%s", pad2(parts[0]), pad2(parts[1]), parts[2])
		}
	}
	// YYYY-MM-DD
	if re := regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`); re.MatchString(s) {
		parts := strings.Split(s, "-")
		if len(parts) == 3 {
			return fmt.Sprintf("%s/%s/%s", pad2(parts[1]), pad2(parts[2]), parts[0])
		}
	}
	// Already MM/DD (no year) - return as-is
	if re := regexp.MustCompile(`^\d{1,2}/\d{1,2}$`); re.MatchString(s) {
		return s
	}
	return s
}

func pad2(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// mergeFields builds the merge_fields map for the API.
// Birthday fields are normalized to MM/DD/YYYY.
func (ct *Contact) mergeFields() map[string]string {
	m := map[string]string{}
	if ct.FirstName != "" {
		m["FN"] = ct.FirstName
	}
	if ct.LastName != "" {
		m["LN"] = ct.LastName
	}
	if ct.Phone != "" {
		m["PHONE"] = ct.Phone
	}
	if ct.Child1FirstName != "" {
		m["C1FN"] = ct.Child1FirstName
	}
	if ct.Child1LastName != "" {
		m["C1LN"] = ct.Child1LastName
	}
	if b := toBirthdayMMDDYYYY(ct.Child1Bday); b != "" {
		m["C1_BDAY"] = b
	}
	if ct.Child2FirstName != "" {
		m["C2FN"] = ct.Child2FirstName
	}
	if b := toBirthdayMMDDYYYY(ct.Child2Bday); b != "" {
		m["C2_BDAY"] = b
	}
	if ct.Child3FirstName != "" {
		m["C3FN"] = ct.Child3FirstName
	}
	if b := toBirthdayMMDDYYYY(ct.Child3Bday); b != "" {
		m["C3_BDAY"] = b
	}
	if ct.MarketingAllowed {
		m["MMERGE17"] = "Yes"
	} else {
		m["MMERGE17"] = "No"
	}
	return m
}

// addMemberRequest is the body for PUT /lists/{list_id}/members/{subscriber_hash}.
type addMemberRequest struct {
	EmailAddress string            `json:"email_address"`
	Status       string            `json:"status"`
	MergeFields  map[string]string `json:"merge_fields"`
}

// AddOrUpdateMember adds or updates a contact in a Mailchimp list (audience).
// Uses PUT /lists/{list_id}/members/{subscriber_hash} for upsert.
// ListID can be found in Audience settings > Audience name and defaults.
// Status must be "subscribed", "unsubscribed", "cleaned", or "pending".
func (c *Client) AddOrUpdateMember(listID string, contact Contact, status string) error {
	if contact.EmailAddress == "" {
		return fmt.Errorf("mailchimp: email address is required")
	}
	if listID == "" {
		return fmt.Errorf("mailchimp: list ID is required")
	}
	if status == "" {
		status = "subscribed"
	}

	hash := md5.Sum([]byte(strings.ToLower(contact.EmailAddress)))
	subscriberHash := hex.EncodeToString(hash[:])

	body := addMemberRequest{
		EmailAddress: contact.EmailAddress,
		Status:       status,
		MergeFields:  contact.mergeFields(),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// Debug: show payload sent to Mailchimp
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, jsonBody, "", "  "); err == nil {
		logging.GetLogger().Info("Mailchimp payload for %s:\n%s", contact.EmailAddress, pretty.String())
	} else {
		logging.GetLogger().Info("Mailchimp payload for %s: %s", contact.EmailAddress, string(jsonBody))
	}

	path := fmt.Sprintf("/lists/%s/members/%s", url.PathEscape(listID), subscriberHash)
	if err := c.do(http.MethodPut, path, nil, bytes.NewReader(jsonBody), nil); err != nil {
		return err
	}
	// Add tags via separate Tags API (merge_fields don't support member tags)
	if len(contact.Tags) > 0 {
		return c.AddMemberTags(listID, contact.EmailAddress, contact.Tags)
	}
	return nil
}

// addTagsRequest is the body for POST /lists/{list_id}/members/{subscriber_hash}/tags.
type addTagsRequest struct {
	Tags []tagItem `json:"tags"`
}

type tagItem struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "active" to add, "inactive" to remove
}

// AddMemberTags adds tags to a list member via the Tags API.
// SubscriberHash is derived from the lowercase email.
func (c *Client) AddMemberTags(listID, email string, tags []string) error {
	if listID == "" || email == "" || len(tags) == 0 {
		return nil
	}
	hash := md5.Sum([]byte(strings.ToLower(email)))
	subscriberHash := hex.EncodeToString(hash[:])
	var items []tagItem
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			items = append(items, tagItem{Name: t, Status: "active"})
		}
	}
	if len(items) == 0 {
		return nil
	}
	body := addTagsRequest{Tags: items}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/lists/%s/members/%s/tags", url.PathEscape(listID), subscriberHash)
	return c.do(http.MethodPost, path, nil, bytes.NewReader(jsonBody), nil)
}
