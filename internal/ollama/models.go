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
		Name:         "gpt-oss:20b",
		DisplayName:  "GPT-OSS 20B",
		SizeEstimate: "12.0 GB",
		Description:  "Open-source GPT-style model with strong reasoning capabilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "gpt-oss:120b",
		DisplayName:  "GPT-OSS 120B",
		SizeEstimate: "68.0 GB",
		Description:  "Large-scale open GPT variant for complex understanding tasks",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "deepseek-r1:70b",
		DisplayName:  "DeepSeek R1 70B",
		SizeEstimate: "40.0 GB",
		Description:  "DeepSeek's advanced reasoning model with strong analytical abilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "gemma3:270m",
		DisplayName:  "Gemma 3 270M",
		SizeEstimate: "160 MB",
		Description:  "Google's ultra-compact model for edge deployment",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "gemma3:27b",
		DisplayName:  "Gemma 3 27B",
		SizeEstimate: "16.0 GB",
		Description:  "Google's powerful mid-size model balancing performance and efficiency",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "qwen3:32b",
		DisplayName:  "Qwen 3 32B",
		SizeEstimate: "19.0 GB",
		Description:  "Alibaba's multilingual model with strong Chinese-English capabilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "qwen3-coder:30b",
		DisplayName:  "Qwen 3 Coder 30B",
		SizeEstimate: "18.0 GB",
		Description:  "Alibaba's specialized coding model with advanced programming capabilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "llama3.1:8b",
		DisplayName:  "Llama 3.1 8B",
		SizeEstimate: "4.7 GB",
		Description:  "Meta's efficient model with extended context window",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "llama3.1:70b",
		DisplayName:  "Llama 3.1 70B",
		SizeEstimate: "40.0 GB",
		Description:  "Meta's flagship open model with state-of-the-art performance",
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
		Name:         "codestral:22b",
		DisplayName:  "Codestral 22B",
		SizeEstimate: "13.0 GB",
		Description:  "Mistral's specialized code generation model",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "MichelRosselli/GLM-4.5-Air:Q4_K_M",
		DisplayName:  "GLM-4.5 Air Q4",
		SizeEstimate: "5.8 GB",
		Description:  "Quantized bilingual model with strong Chinese-English capabilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "phi3:3.8b",
		DisplayName:  "Phi-3 3.8B",
		SizeEstimate: "2.3 GB",
		Description:  "Microsoft's compact model with impressive reasoning abilities",
		Category:     "chat",
		Status:       StatusNotDownloaded,
	},
	{
		Name:         "phi3:14b",
		DisplayName:  "Phi-3 14B",
		SizeEstimate: "7.9 GB",
		Description:  "Microsoft's larger Phi model with enhanced capabilities",
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
