// Package caddy provides a client for interacting with the Caddy API
package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Caddy Admin API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddOrUpdateRoute adds or updates a route in Caddy's configuration
func (c *Client) AddOrUpdateRoute(route *RouteConfig) error {
	jsonData, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("failed to marshal route config: %w", err)
	}

	log.Printf("[Caddy] Adding/updating route %s with config: %s", route.ID, string(jsonData))

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
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Caddy] ERROR: Failed to add/update route. Status: %d, Response: %s", resp.StatusCode, string(body))
		return fmt.Errorf("Caddy returned status %d when adding/updating route: %s", resp.StatusCode, string(body))
	}

	log.Printf("[Caddy] Successfully added/updated route %s", route.ID)

	return nil
}

// DeleteRoute deletes a route from Caddy's configuration by its ID
func (c *Client) DeleteRoute(routeID string) error {
	log.Printf("[Caddy] Deleting route %s", routeID)
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
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Caddy] ERROR: Failed to delete route. Status: %d, Response: %s", resp.StatusCode, string(body))
		return fmt.Errorf("Caddy returned status %d when deleting route: %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[Caddy] Route %s not found (already deleted)", routeID)
	} else {
		log.Printf("[Caddy] Successfully deleted route %s", routeID)
	}

	return nil
}

// CreateRouteConfig creates a RouteConfig for an application
func CreateRouteConfig(appID, subdomain string, hostPort int, publicDomain, tailscaleDomain string) *RouteConfig {
	routeID := fmt.Sprintf("route-for-app-%s", appID)

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
