package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Client represents a connection to the local Podman runtime.
type Client struct {
	podmanBinary string
}

// NewClient validates that Podman is available and returns a runtime client.
func NewClient() (*Client, error) {
	bin := os.Getenv("PODMAN_BINARY")
	if bin == "" {
		bin = "podman"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	versionCmd := exec.CommandContext(ctx, bin, "--version")
	if output, err := versionCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("podman client not available: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// Ensure we can talk to the Podman service (rootless or system-wide)
	infoCmd := exec.CommandContext(ctx, bin, "info", "--format", "json")
	if output, err := infoCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("podman client not available: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return &Client{podmanBinary: bin}, nil
}

// Close closes the client. Present for API compatibility.
func (c *Client) Close() error {
	return nil
}

// binary returns the configured Podman binary path.
func (c *Client) binary() string {
	return c.podmanBinary
}

// run executes the Podman CLI with the provided arguments.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	// #nosec G204 -- arguments originate from trusted configuration/internal inputs
	cmd := exec.CommandContext(ctx, c.podmanBinary, args...)
	return cmd.CombinedOutput()
}
