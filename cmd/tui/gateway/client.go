package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wolink-core/internal/plugins"
	"wolink-core/internal/services"
)

// APIError represents an error response from the gateway admin API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return e.Message
}

// Client is an HTTP client for the gateway admin API.
// All methods send X-Admin-Token for authentication.
type Client struct {
	gwURL      string
	adminToken string
	httpClient *http.Client
}

// NewClient creates a new GatewayClient with the given gateway URL, admin token,
// and HTTP client timeout. timeout is used for the underlying http.Client.
func NewClient(gwURL string, adminToken string, timeout time.Duration) *Client {
	// Strip trailing slash from base URL for consistent path joining
	gwURL = strings.TrimRight(gwURL, "/")
	return &Client{
		gwURL:      gwURL,
		adminToken: adminToken,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// doRequest performs an HTTP request with the X-Admin-Token header set.
// It reads and closes the response body, returning the raw bytes.
func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, c.gwURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if c.adminToken != "" {
		req.Header.Set("X-Admin-Token", c.adminToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return data, nil
}

// Health checks the gateway health endpoint.
// Returns (true, "", nil) if the gateway responds with {"status":"ok"}.
// Returns (false, errorMessage, nil) if the gateway responds but is not OK.
// Returns (false, "", err) if the request itself fails.
func (c *Client) Health() (bool, string, error) {
	data, err := c.doRequest("GET", "/health", nil)
	if err != nil {
		return false, "", err
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, "", fmt.Errorf("failed to parse health response: %w", err)
	}

	if result.Status == "ok" {
		return true, "", nil
	}
	return false, fmt.Sprintf("unexpected health status: %s", result.Status), nil
}

// NodeStatus fetches the gateway node status.
// GET /admin/node/status with X-Admin-Token header.
func (c *Client) NodeStatus() (*services.NodeStatus, error) {
	data, err := c.doRequest("GET", "/admin/node/status", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data services.NodeStatus `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse node status response: %w", err)
	}

	return &result.Data, nil
}

// ListPlugins returns the list of loaded plugins.
// GET /admin/plugins with X-Admin-Token header.
func (c *Client) ListPlugins() ([]plugins.PluginInfo, error) {
	data, err := c.doRequest("GET", "/admin/plugins", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Plugins []plugins.PluginInfo `json:"plugins"`
		Count   int                  `json:"count"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse plugins response: %w", err)
	}

	return result.Plugins, nil
}

// ReloadPlugin reloads a plugin by protocol name.
// POST /admin/plugins/:protocol/reload with X-Admin-Token header.
func (c *Client) ReloadPlugin(protocol string) error {
	path := fmt.Sprintf("/admin/plugins/%s/reload", protocol)
	_, err := c.doRequest("POST", path, nil)
	return err
}

// UnloadPlugin unloads a plugin by protocol name.
// DELETE /admin/plugins/:protocol with X-Admin-Token header.
func (c *Client) UnloadPlugin(protocol string) error {
	path := fmt.Sprintf("/admin/plugins/%s", protocol)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}

// HealthCheckPlugins triggers a health check on all plugins and returns their status.
// POST /admin/plugins/health-check with X-Admin-Token header.
func (c *Client) HealthCheckPlugins() ([]plugins.PluginInfo, error) {
	data, err := c.doRequest("POST", "/admin/plugins/health-check", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Message string              `json:"message"`
		Plugins []plugins.PluginInfo `json:"plugins"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse health-check response: %w", err)
	}

	return result.Plugins, nil
}
