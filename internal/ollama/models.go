package ollama

import (
	"database/sql"
	"time"
)

// OllamaModel represents an Ollama model in the system
type OllamaModel struct { //nolint:revive // intentional naming for clarity
	Name         string         `json:"name"`          // Primary key, e.g., "llama3:8b"
	DisplayName  string         `json:"display_name"`  // User-friendly name
	SizeEstimate string         `json:"size_estimate"` // Estimated download size
	Description  string         `json:"description"`
	Category     string         `json:"category"` // chat, code, vision, etc.
	Status       string         `json:"status"`   // not_downloaded, queued, downloading, completed, failed
	Progress     int            `json:"progress"` // 0-100
	LastError    sql.NullString `json:"last_error,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  sql.NullTime   `json:"completed_at,omitempty"`
}

// DownloadJob represents a download job in the queue
type DownloadJob struct {
	ID        string
	ModelName string
	Status    string
	CreatedAt time.Time
	StartedAt sql.NullTime
}

// ProgressUpdate represents a progress update for SSE broadcasting
type ProgressUpdate struct {
	ModelName string `json:"model_name"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Error     string `json:"error,omitempty"`
}

// ModelStatus constants
const (
	StatusNotDownloaded = "not_downloaded"
	StatusQueued        = "queued"
	StatusDownloading   = "downloading"
	StatusCompleted     = "completed"
	StatusFailed        = "failed"
)

// CuratedModels contains the list of recommended models
var CuratedModels = []OllamaModel{
	// Chat Models
	{
		Name:         "gemma:2b",
		DisplayName:  "Gemma 2B",
		SizeEstimate: "1.7 GB",
		Description:  "Google's lightweight model for basic tasks",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "llama3.2:3b",
		DisplayName:  "Llama 3.2 3B",
		SizeEstimate: "2.0 GB",
		Description:  "Meta's latest compact model, excellent for general chat",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "llama3.2:1b",
		DisplayName:  "Llama 3.2 1B",
		SizeEstimate: "1.3 GB",
		Description:  "Ultra-lightweight Llama model for basic tasks",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "gemma2:2b",
		DisplayName:  "Gemma 2 2B",
		SizeEstimate: "1.6 GB",
		Description:  "Google's efficient small language model",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "gemma2:9b",
		DisplayName:  "Gemma 2 9B",
		SizeEstimate: "5.5 GB",
		Description:  "Google's larger Gemma model with better performance",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "mistral:7b",
		DisplayName:  "Mistral 7B",
		SizeEstimate: "4.1 GB",
		Description:  "High-performance open model from Mistral AI",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "phi3:mini",
		DisplayName:  "Phi-3 Mini",
		SizeEstimate: "2.3 GB",
		Description:  "Microsoft's small but capable language model",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},

	// Code Models
	{
		Name:         "qwen2.5-coder:7b",
		DisplayName:  "Qwen 2.5 Coder 7B",
		SizeEstimate: "4.7 GB",
		Description:  "Specialized model for code generation and analysis",
		Category:     "code",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "codellama:7b",
		DisplayName:  "Code Llama 7B",
		SizeEstimate: "3.8 GB",
		Description:  "Meta's code-specialized Llama variant",
		Category:     "code",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "deepseek-coder:6.7b",
		DisplayName:  "DeepSeek Coder 6.7B",
		SizeEstimate: "3.8 GB",
		Description:  "Strong code model trained on diverse programming languages",
		Category:     "code",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "starcoder2:3b",
		DisplayName:  "StarCoder2 3B",
		SizeEstimate: "1.7 GB",
		Description:  "Compact code model from BigCode",
		Category:     "code",
		Status:       StatusNotDownloaded,
	},

	// Vision Models
	{
		Name:         "llava:7b",
		DisplayName:  "LLaVA 7B",
		SizeEstimate: "4.5 GB",
		Description:  "Multimodal model for image understanding",
		Category:     "vision",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "bakllava:7b",
		DisplayName:  "BakLLaVA 7B",
		SizeEstimate: "4.5 GB",
		Description:  "Enhanced LLaVA with better image comprehension",
		Category:     "vision",
		Status:       StatusNotDownloaded,
	},

	// Specialized Models
	{
		Name:         "llama3.2:11b-vision",
		DisplayName:  "Llama 3.2 11B Vision",
		SizeEstimate: "7.9 GB",
		Description:  "Meta's multimodal model with vision capabilities",
		Category:     "vision",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "dolphin-mistral:7b",
		DisplayName:  "Dolphin Mistral 7B",
		SizeEstimate: "4.1 GB",
		Description:  "Uncensored Mistral fine-tune for diverse tasks",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "solar:10.7b",
		DisplayName:  "Solar 10.7B",
		SizeEstimate: "6.1 GB",
		Description:  "Upstage's powerful Korean-English bilingual model",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
}

// GetCuratedModel returns a curated model by name
func GetCuratedModel(name string) (*OllamaModel, bool) {
	for _, model := range CuratedModels {
		if model.Name == name {
			return &model, true
		}
	}
	return nil, false
}

// GetModelsByCategory returns all models in a specific category
func GetModelsByCategory(category string) []OllamaModel {
	var models []OllamaModel
	for _, model := range CuratedModels {
		if model.Category == category {
			models = append(models, model)
		}
	}
	return models
}
