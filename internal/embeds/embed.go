package embeds

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed static templates
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
	return template.ParseFS(content, patterns...)
}
