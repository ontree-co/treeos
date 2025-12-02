// template-check validates all HTML templates for syntax errors
package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"github.com/ontree-co/treeos/internal/logging"
)

// extractHostPort extracts the host port from a "hostPort:containerPort" string
func extractHostPort(portMapping string) string {
	parts := strings.Split(portMapping, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func main() {
	templatesDir := "templates"
	if len(os.Args) > 1 {
		templatesDir = os.Args[1]
	}

	// Check if templates directory exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		logging.Fatalf("Templates directory not found: %s", templatesDir)
	}

	baseTemplatePath := filepath.Join(templatesDir, "layouts", "base.html")

	// Check if base template exists
	if _, err := os.Stat(baseTemplatePath); os.IsNotExist(err) {
		logging.Fatalf("Base template not found: %s", baseTemplatePath)
	}

	errors := []string{}
	checked := 0

	// Walk through all template files
	err := filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-HTML files
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Skip the base template itself
		if path == baseTemplatePath {
			return nil
		}

		// Skip pattern library templates (they might not use base)
		if strings.Contains(path, "pattern_library") {
			return nil
		}

		checked++

		// Try to parse the template with base
		funcMap := template.FuncMap{
			"extractHostPort": extractHostPort,
		}
		tmpl := template.New("test").Funcs(funcMap)
		_, err = tmpl.ParseFiles(baseTemplatePath, path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		// For component templates, also check if they parse standalone
		if strings.Contains(path, "components/") {
			tmpl = template.New("test").Funcs(funcMap)
			_, err = tmpl.ParseFiles(path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s (standalone): %v", path, err))
			}
		}

		fmt.Printf("✓ %s\n", path)
		return nil
	})

	if err != nil {
		logging.Fatalf("Error walking templates: %v", err)
	}

	// Report results
	fmt.Printf("\nChecked %d templates\n", checked)

	if len(errors) > 0 {
		fmt.Printf("\n❌ Found %d template errors:\n\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
		os.Exit(1)
	}
	fmt.Printf("✅ All templates are valid!\n")
}
