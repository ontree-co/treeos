// Package embeds provides embedded static assets and templates for the OnTree application.
package embeds

import (
	"embed"
	"html/template"
	"io/fs"
	"strings"
)

//go:embed static templates templates/dashboard/_*.html app-templates
var content embed.FS

// StaticFS returns the embedded static files
func StaticFS() (fs.FS, error) {
	return fs.Sub(content, "static")
}

// TemplateFS returns the embedded HTML template files
func TemplateFS() (fs.FS, error) {
	return fs.Sub(content, "templates")
}

// AppTemplateFS returns the embedded application template files
func AppTemplateFS() (fs.FS, error) {
	return fs.Sub(content, "app-templates")
}

// ParseTemplate parses templates from the embedded filesystem with custom functions
func ParseTemplate(patterns ...string) (*template.Template, error) {
	// Define custom template functions
	funcMap := template.FuncMap{
		"extractHostPort": extractHostPort,
	}

	// Create template with custom functions
	tmpl := template.New("").Funcs(funcMap)

	// Parse templates from embedded filesystem
	return tmpl.ParseFS(content, patterns...)
}

// extractHostPort extracts the host port from a "hostPort:containerPort" string
// It handles malformed inputs gracefully (e.g., "3080:-3080}" returns "3080")
func extractHostPort(portMapping string) string {
	// Handle empty input
	if portMapping == "" {
		return ""
	}

	// Clean up any trailing special characters from malformed YAML
	portMapping = strings.TrimRight(portMapping, "}])\"'")

	// Split on colon to get host:container format
	parts := strings.Split(portMapping, ":")
	if len(parts) > 0 {
		// Clean up the host port part
		hostPort := strings.TrimSpace(parts[0])
		// Remove any non-numeric prefix (like minus sign)
		hostPort = strings.TrimLeft(hostPort, "-")
		// If the result is empty or not a valid port, return empty string
		if hostPort == "" || hostPort == "-" {
			return ""
		}
		return hostPort
	}
	return ""
}
