// Package embeds provides embedded static assets and templates for the OnTree application.
package embeds

import (
	"embed"
	"fmt"
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

// ParseTemplate parses templates from the embedded filesystem
func ParseTemplate(patterns ...string) (*template.Template, error) {
	// Create a new template with custom functions
	tmpl := template.New("").Funcs(templateFuncs())
	return tmpl.ParseFS(content, patterns...)
}

// templateFuncs returns custom template functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatPort": formatPort,
	}
}

// formatPort formats a port string for display with larger host port
func formatPort(port string) template.HTML {
	// Split on colon
	parts := strings.SplitN(port, ":", 2)
	if len(parts) != 2 {
		// Return original if not in expected format
		return template.HTML(template.HTMLEscapeString(port))
	}
	
	// Format with larger host port
	formatted := fmt.Sprintf(`<span style="font-size: 1.1rem; font-weight: 500;">%s</span>:%s`,
		template.HTMLEscapeString(parts[0]),
		template.HTMLEscapeString(parts[1]))
	
	return template.HTML(formatted)
}
