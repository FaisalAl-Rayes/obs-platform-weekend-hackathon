// Package gitea provides a lightweight client for the Gitea API.
//
// It covers webhook management — the subset of the API that RIA needs.
// File content operations are handled via local git clones instead of the
// REST API.
package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is a thin wrapper around the Gitea REST API.
type Client struct {
	baseURL    string
	user       string
	pass       string
	httpClient *http.Client
}

// NewClient creates a Gitea API client with basic-auth credentials.
func NewClient(baseURL, user, pass string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		user:       user,
		pass:       pass,
		httpClient: &http.Client{},
	}
}

// ----- Webhook types -------------------------------------------------------

// Hook represents a Gitea webhook.
type Hook struct {
	ID     int64      `json:"id"`
	Type   string     `json:"type"`
	Active bool       `json:"active"`
	Config HookConfig `json:"config"`
	Events []string   `json:"events"`
}

// HookConfig holds webhook configuration.
type HookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
}

// CreateHookOption is the request body for creating a webhook.
type CreateHookOption struct {
	Type         string            `json:"type"`
	Config       map[string]string `json:"config"`
	Events       []string          `json:"events"`
	Active       bool              `json:"active"`
	BranchFilter string            `json:"branch_filter,omitempty"`
}

// ----- Webhook operations --------------------------------------------------

// ListWebhooks returns all webhooks on a repository.
func (c *Client) ListWebhooks(owner, repo string) ([]Hook, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/hooks", c.baseURL, owner, repo)

	resp, err := c.doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("listing webhooks %s/%s: status %d: %s", owner, repo, resp.StatusCode, body)
	}

	var hooks []Hook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return nil, fmt.Errorf("decoding webhooks: %w", err)
	}
	return hooks, nil
}

// CreateWebhook creates a new webhook on a repository.
func (c *Client) CreateWebhook(owner, repo string, opt CreateHookOption) error {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/hooks", c.baseURL, owner, repo)

	body, err := json.Marshal(opt)
	if err != nil {
		return fmt.Errorf("marshalling webhook options: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating webhook %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("creating webhook %s/%s: status %d: %s", owner, repo, resp.StatusCode, respBody)
	}
	return nil
}

// ----- Internal helpers ----------------------------------------------------

func (c *Client) doRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.pass)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}
