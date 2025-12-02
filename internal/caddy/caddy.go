// Package caddy provides a client for interacting with the Caddy API
package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/ontree-co/treeos/internal/logging"
)

// Client represents a client for interacting with Caddy's Admin API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Caddy API client
func NewClient() *Client {
	return &Client{
		baseURL: "http://localhost:2019",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RouteConfig represents the structure of a Caddy route configuration
type RouteConfig struct {
	ID       string      `json:"@id"`
	Match    []MatchRule `json:"match"`
	Handle   []Handler   `json:"handle"`
	Terminal bool        `json:"terminal"`
}

// MatchRule represents a matching rule for the route
type MatchRule struct {
	Host []string `json:"host"`
}

// Handler represents a handler configuration
type Handler struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams,omitempty"`
}

// Upstream represents an upstream server configuration
type Upstream struct {
	Dial string `json:"dial"`
}

// HealthCheck performs a health check on the Caddy Admin API
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/config/")
	if err != nil {
		return fmt.Errorf("failed to connect to Caddy Admin API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("caddy Admin API returned status %d (failed to read body: %w)", resp.StatusCode, err)
		}
		return fmt.Errorf("caddy Admin API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddOrUpdateRoute adds or updates a route in Caddy's configuration
func (c *Client) AddOrUpdateRoute(route *RouteConfig) error {
	// First ensure the HTTP app exists
	if err := c.ensureHTTPApp(); err != nil {
		return fmt.Errorf("failed to ensure HTTP app exists: %w", err)
	}

	jsonData, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("failed to marshal route config: %w", err)
	}

	logging.Infof("[Caddy] Adding/updating route %s with config: %s", route.ID, string(jsonData))

	url := c.baseURL + "/config/apps/http/servers/srv0/routes"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Caddy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logging.Errorf("[Caddy] ERROR: Failed to add/update route. Status: %d, failed to read response body: %v", resp.StatusCode, err)
			return fmt.Errorf("caddy returned status %d when adding/updating route (failed to read body: %w)", resp.StatusCode, err)
		}
		logging.Errorf("[Caddy] ERROR: Failed to add/update route. Status: %d, Response: %s", resp.StatusCode, string(body))
		return fmt.Errorf("caddy returned status %d when adding/updating route: %s", resp.StatusCode, string(body))
	}

	logging.Infof("[Caddy] Successfully added/updated route %s", route.ID)

	return nil
}

// ensureHTTPApp ensures that the HTTP app exists in Caddy's configuration
func (c *Client) ensureHTTPApp() error {
	// Check if HTTP app exists
	resp, err := c.httpClient.Get(c.baseURL + "/config/apps/http")
	if err != nil {
		return fmt.Errorf("failed to check HTTP app: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// If HTTP app exists, we're done
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Create the HTTP app with a default server
	logging.Infof("[Caddy] HTTP app not found, initializing...")

	// Define the HTTP app configuration
	httpApp := map[string]interface{}{
		"servers": map[string]interface{}{
			"srv0": map[string]interface{}{
				"listen": []string{":80", ":443"},
				"routes": []interface{}{},
			},
		},
	}

	jsonData, err := json.Marshal(httpApp)
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP app config: %w", err)
	}

	// Create the HTTP app
	req, err := http.NewRequest(http.MethodPut, c.baseURL+"/config/apps/http", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP app request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create HTTP app: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to create HTTP app, status %d (failed to read body: %w)", resp.StatusCode, err)
		}
		return fmt.Errorf("failed to create HTTP app, status %d: %s", resp.StatusCode, string(body))
	}

	logging.Infof("[Caddy] Successfully initialized HTTP app")
	return nil
}

// DeleteRoute deletes a route from Caddy's configuration by its ID
func (c *Client) DeleteRoute(routeID string) error {
	logging.Infof("[Caddy] Deleting route %s", routeID)
	url := c.baseURL + "/id/" + routeID
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request to Caddy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Caddy returns 200 for successful deletion, 404 if route doesn't exist
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logging.Errorf("[Caddy] ERROR: Failed to delete route. Status: %d, failed to read response body: %v", resp.StatusCode, err)
			return fmt.Errorf("caddy returned status %d when deleting route (failed to read body: %w)", resp.StatusCode, err)
		}
		logging.Errorf("[Caddy] ERROR: Failed to delete route. Status: %d, Response: %s", resp.StatusCode, string(body))
		return fmt.Errorf("caddy returned status %d when deleting route: %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode == http.StatusNotFound {
		logging.Infof("[Caddy] Route %s not found (already deleted)", routeID)
	} else {
		logging.Infof("[Caddy] Successfully deleted route %s", routeID)
	}

	return nil
}

// CreateRouteConfig creates a RouteConfig for an application
func CreateRouteConfig(appID, subdomain string, hostPort int, publicDomain, tailscaleDomain string) *RouteConfig {
	routeID := fmt.Sprintf("route-for-%s", appID)

	hosts := []string{}
	if publicDomain != "" {
		hosts = append(hosts, fmt.Sprintf("%s.%s", subdomain, publicDomain))
	}
	if tailscaleDomain != "" {
		hosts = append(hosts, fmt.Sprintf("%s.%s", subdomain, tailscaleDomain))
	}

	return &RouteConfig{
		ID: routeID,
		Match: []MatchRule{
			{
				Host: hosts,
			},
		},
		Handle: []Handler{
			{
				Handler: "reverse_proxy",
				Upstreams: []Upstream{
					{
						Dial: fmt.Sprintf("localhost:%d", hostPort),
					},
				},
			},
		},
		Terminal: true,
	}
}
