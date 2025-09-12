// Package embeds provides embedded static assets and templates for the OnTree application.
package embeds

import (
	"embed"
	"html/template"
	"io/fs"
	"strings"
)

//go:embed static templates templates/dashboard/_*.html
var content embed.FS

// StaticFS returns the embedded static files
func StaticFS() (fs.FS, error) {
	return fs.Sub(content, "static")
}

// TemplateFS returns the embedded template files
func TemplateFS() (fs.FS, error) {
	return fs.Sub(content, "templates")
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
func extractHostPort(portMapping string) string {
	parts := strings.Split(portMapping, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
