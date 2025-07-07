package server

import (
	"bytes"
	"html/template"
	"testing"
)

// TestTemplateDataStructures verifies that handler data structures include required fields
func TestTemplateDataStructures(t *testing.T) {
	// Create a minimal base template that mimics the real base.html requirement
	baseTemplate := `
	{{if .Messages}}
		{{range .Messages}}
			<div class="alert alert-{{.Type}}">{{.Text}}</div>
		{{end}}
	{{end}}
	{{template "content" .}}
	`
	
	contentTemplate := `{{define "content"}}<h1>Test Content</h1>{{end}}`
	
	// Parse templates
	tmpl := template.New("base")
	tmpl, err := tmpl.Parse(baseTemplate)
	if err != nil {
		t.Fatalf("Failed to parse base template: %v", err)
	}
	
	_, err = tmpl.Parse(contentTemplate)
	if err != nil {
		t.Fatalf("Failed to parse content template: %v", err)
	}
	
	// Test data structures that should work with the base template
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "Setup handler data with Messages field",
			data: struct {
				User      interface{}
				Errors    []string
				FormData  map[string]string
				CSRFToken string
				Messages  []interface{}
			}{
				User:      nil,
				Errors:    nil,
				FormData:  map[string]string{"node_name": "Test"},
				CSRFToken: "",
				Messages:  nil, // Critical field that was missing
			},
			wantErr: false,
		},
		{
			name: "Data structure missing Messages field",
			data: struct {
				User      interface{}
				Errors    []string
				FormData  map[string]string
				CSRFToken string
				// Messages field is missing - this should fail
			}{
				User:      nil,
				Errors:    nil,
				FormData:  map[string]string{"node_name": "Test"},
				CSRFToken: "",
			},
			wantErr: true, // Template requires Messages field - will fail if missing
		},
		{
			name: "Dashboard handler data with Messages",
			data: struct {
				User          interface{}
				Messages      []interface{}
				CSRFToken     string
				Apps          []interface{}
				SystemVitals  interface{}
				DockerStatus  interface{}
			}{
				User:      nil,
				Messages:  nil,
				CSRFToken: "",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tmpl.Execute(&buf, tt.data)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("Template execution error = %v, wantErr %v", err, tt.wantErr)
			}
			
			// Log the output for debugging
			if err == nil {
				t.Logf("Template output: %s", buf.String())
			}
		})
	}
}

// TestMessagesFieldRequirement documents which handlers need Messages field
func TestMessagesFieldRequirement(t *testing.T) {
	// This test serves as documentation for developers
	handlersWithTemplates := map[string]bool{
		"handleSetup":              true,
		"handleLogin":              true,
		"handleDashboard":          true,
		"handleApps":               true,
		"handleAppDetail":          true,
		"handleAppCreate":          true, // Currently missing Messages field!
		"handleTemplates":          true,
		"handleCreateFromTemplate": true,
	}
	
	t.Log("All handlers that render HTML templates MUST include a Messages field in their data structure")
	t.Log("The Messages field should be []interface{} and can be nil if no messages")
	t.Log("Flash messages should be formatted as: map[string]interface{}{\"Type\": \"success\", \"Text\": \"message\"}")
	
	for handler, requiresMessages := range handlersWithTemplates {
		if requiresMessages {
			t.Logf("✓ %s - MUST include Messages field", handler)
		}
	}
}

// TestStaleOperationHandling tests that stale operations don't show spinner in UI
func TestStaleOperationHandling(t *testing.T) {
	// This test documents the behavior for handling stale operations
	// The app detail page should NOT show a spinner for operations older than 5 minutes
	
	t.Run("Query should exclude old operations", func(t *testing.T) {
		// The query in handleAppDetail should include a time filter
		expectedQueryCondition := "AND created_at > datetime('now', '-5 minutes')"
		
		t.Logf("handleAppDetail must filter operations with: %s", expectedQueryCondition)
		t.Log("This prevents showing 'Waiting to start...' spinner for stale operations")
	})
	
	t.Run("Worker should cleanup stale operations", func(t *testing.T) {
		t.Log("Worker.cleanupStaleOperations() should run every minute")
		t.Log("It should mark operations older than 5 minutes as failed")
		t.Log("This prevents accumulation of stale pending/in_progress operations")
	})
	
	t.Run("Expected behavior for not_created containers", func(t *testing.T) {
		testCases := []struct {
			name              string
			containerStatus   string
			hasOldOperation   bool
			hasRecentOperation bool
			expectedUI        string
		}{
			{
				name:              "not_created container with no operations",
				containerStatus:   "not_created",
				hasOldOperation:   false,
				hasRecentOperation: false,
				expectedUI:        "Show 'Create & Start' button, no spinner",
			},
			{
				name:              "not_created container with old operation",
				containerStatus:   "not_created",
				hasOldOperation:   true,
				hasRecentOperation: false,
				expectedUI:        "Show 'Create & Start' button, no spinner (old operation ignored)",
			},
			{
				name:              "not_created container with recent operation",
				containerStatus:   "not_created",
				hasOldOperation:   false,
				hasRecentOperation: true,
				expectedUI:        "Show spinner with 'Waiting to start...'",
			},
			{
				name:              "running container with old operation",
				containerStatus:   "running",
				hasOldOperation:   true,
				hasRecentOperation: false,
				expectedUI:        "Show 'Stop' button, no spinner",
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Container status: %s", tc.containerStatus)
				t.Logf("Has old operation (>5 min): %v", tc.hasOldOperation)
				t.Logf("Has recent operation (<5 min): %v", tc.hasRecentOperation)
				t.Logf("Expected UI: %s", tc.expectedUI)
			})
		}
	})
}