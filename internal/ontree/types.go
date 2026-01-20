package ontree

// ProgressEvent streams progress updates from long-running operations.
type ProgressEvent struct {
	Type    string      `json:"type"`
	Message string      `json:"message,omitempty"`
	Code    string      `json:"code,omitempty"`
	Percent int         `json:"percent,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SetupStatus reports the initial setup state.
type SetupStatus struct {
	Complete bool   `json:"complete"`
	NodeName string `json:"node_name,omitempty"`
	NodeIcon string `json:"node_icon,omitempty"`
}

// App represents a minimal app listing result.
type App struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Model represents a minimal model listing result.
type Model struct {
	Name string `json:"name"`
}
