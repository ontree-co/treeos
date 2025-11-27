package ollama

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"treeos/internal/logging"

	"treeos/internal/config"
)

var sharedModelsDir atomic.Value

func init() {
	sharedModelsDir.Store("")
}

type manifestMetadata struct {
	Model string `json:"model"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
}

// SharedModelsDirectory returns the last discovered models directory on disk.
// This is primarily used for display purposes in the UI when no models are installed yet.
func SharedModelsDirectory() string {
	if v, ok := sharedModelsDir.Load().(string); ok {
		return v
	}
	return ""
}

// SyncDatabaseWithSharedModels ensures the database reflects the models that exist on disk
// and vice versa. It promotes any discovered models to completed status and resets entries
// for models that are missing from shared storage.
func SyncDatabaseWithSharedModels(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	existingRecords, err := fetchAllModelRecords(db)
	if err != nil {
		return fmt.Errorf("failed to load existing model records: %w", err)
	}

	existingByName := make(map[string]OllamaModel, len(existingRecords))
	existingByLower := make(map[string]string, len(existingRecords))
	for _, record := range existingRecords {
		existingByName[record.Name] = record
		existingByLower[strings.ToLower(record.Name)] = record.Name
	}

	curatedByLower := make(map[string]string, len(CuratedModels))
	for _, model := range CuratedModels {
		curatedByLower[strings.ToLower(model.Name)] = model.Name
	}

	sharedBase := config.GetSharedOllamaPath()
	installedOnDisk, modelsDir, err := discoverModelsOnDisk(sharedBase)
	if err != nil {
		return fmt.Errorf("failed to discover models in shared storage: %w", err)
	}

	if modelsDir == "" {
		modelsDir = filepath.Join(sharedBase, "models")
	}
	sharedModelsDir.Store(modelsDir)

	installedCanonical := make(map[string]struct{}, len(installedOnDisk))

	for rawName := range installedOnDisk {
		candidate := strings.TrimSpace(rawName)
		if candidate == "" {
			continue
		}

		lower := strings.ToLower(candidate)
		if curatedName, ok := curatedByLower[lower]; ok {
			candidate = curatedName
		} else if existingName, ok := existingByLower[lower]; ok {
			candidate = existingName
		}

		installedCanonical[candidate] = struct{}{}

		if _, exists := existingByName[candidate]; !exists {
			modelRecord := buildModelRecordForInstalled(candidate)
			if err := CreateModel(db, modelRecord); err != nil {
				logging.Errorf("Failed to create database record for discovered model %s: %v", candidate, err)
				continue
			}
			existingByName[candidate] = *modelRecord
			existingByLower[strings.ToLower(candidate)] = candidate
		}

		if err := UpdateModelStatus(db, candidate, StatusCompleted, 100); err != nil {
			logging.Errorf("Failed to mark model %s as completed during sync: %v", candidate, err)
		}
	}

	// Any models previously marked as completed but missing on disk should be reset.
	for _, record := range existingRecords {
		if record.Status != StatusCompleted {
			continue
		}
		if _, ok := installedCanonical[record.Name]; ok {
			continue
		}

		if err := UpdateModelStatus(db, record.Name, StatusNotDownloaded, 0); err != nil {
			logging.Errorf("Failed to reset model %s to not_downloaded during sync: %v", record.Name, err)
		}
	}

	return nil
}

func buildModelRecordForInstalled(name string) *OllamaModel {
	if curated, ok := GetCuratedModel(name); ok {
		curatedCopy := *curated
		curatedCopy.Status = StatusCompleted
		curatedCopy.Progress = 100
		curatedCopy.LastError = sql.NullString{}
		curatedCopy.CompletedAt = sql.NullTime{}
		return &curatedCopy
	}

	return &OllamaModel{
		Name:         name,
		DisplayName:  name,
		SizeEstimate: "Unknown",
		Description:  fmt.Sprintf("Custom model %s discovered on disk", name),
		Category:     "custom",
		Status:       StatusCompleted,
		Progress:     100,
		LastError:    sql.NullString{},
		CompletedAt:  sql.NullTime{},
	}
}

func discoverModelsOnDisk(sharedBase string) (map[string]struct{}, string, error) {
	candidates := uniqueStrings([]string{
		filepath.Join(sharedBase, "models", "manifests"),
		filepath.Join(sharedBase, "manifests"),
	})

	discovered := make(map[string]struct{})
	var modelsDir string

	for _, manifestsRoot := range candidates {
		info, err := os.Stat(manifestsRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if errors.Is(err, fs.ErrPermission) {
				logging.Errorf("Skipping shared Ollama manifest path %s due to permission error: %v", manifestsRoot, err)
				continue
			}
			return nil, "", err
		}
		if !info.IsDir() {
			continue
		}

		candidateModelsDir := filepath.Dir(manifestsRoot)
		if modelsDir == "" {
			modelsDir = candidateModelsDir
		}

		walkErr := filepath.WalkDir(manifestsRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) || errors.Is(walkErr, fs.ErrPermission) {
					if errors.Is(walkErr, fs.ErrPermission) {
						logging.Errorf("Skipping %s during model discovery due to permission error: %v", path, walkErr)
					}
					return nil
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			name := extractModelNameFromManifest(path, manifestsRoot)
			if name != "" {
				discovered[name] = struct{}{}
			}
			return nil
		})

		if walkErr != nil {
			return nil, "", walkErr
		}
	}

	if modelsDir == "" {
		modelsDir = filepath.Join(sharedBase, "models")
	}

	return discovered, modelsDir, nil
}

func extractModelNameFromManifest(path, manifestsRoot string) string {
	data, err := os.ReadFile(path) //nolint:gosec // File path from Ollama manifests directory
	if err != nil {
		logging.Errorf("Failed to read manifest %s: %v", path, err)
		return fallbackModelNameFromPath(path, manifestsRoot)
	}

	var metadata manifestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		logging.Errorf("Failed to parse manifest %s: %v", path, err)
		return fallbackModelNameFromPath(path, manifestsRoot)
	}

	candidate := strings.TrimSpace(metadata.Model)
	if candidate == "" {
		candidate = strings.TrimSpace(metadata.Name)
	}

	tag := strings.TrimSpace(metadata.Tag)

	candidate = normalizeModelIdentifier(candidate)

	if candidate != "" && tag != "" && !strings.Contains(candidate, ":") {
		candidate = fmt.Sprintf("%s:%s", candidate, tag)
	}

	if candidate == "" {
		candidate = fallbackModelNameFromPath(path, manifestsRoot)
	}

	return normalizeModelIdentifier(candidate)
}

func fallbackModelNameFromPath(path, manifestsRoot string) string {
	rel, err := filepath.Rel(manifestsRoot, path)
	if err != nil {
		return ""
	}

	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")

	var cleanParts []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleanParts = append(cleanParts, part)
	}

	if len(cleanParts) < 2 {
		return ""
	}

	tag := cleanParts[len(cleanParts)-1]
	modelParts := cleanParts[:len(cleanParts)-1]

	for len(modelParts) > 0 {
		lower := strings.ToLower(modelParts[0])
		if lower == "registry.ollama.ai" || lower == "models" || lower == "manifests" || lower == "library" {
			modelParts = modelParts[1:]
			continue
		}
		break
	}

	if len(modelParts) == 0 {
		return ""
	}

	base := strings.Join(modelParts, "/")
	if base == "" || tag == "" {
		return ""
	}

	identifier := fmt.Sprintf("%s:%s", base, tag)
	return normalizeModelIdentifier(identifier)
}

func normalizeModelIdentifier(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimPrefix(trimmed, "registry.ollama.ai/")
	trimmed = strings.TrimPrefix(trimmed, "library/")
	trimmed = strings.TrimPrefix(trimmed, "models/")

	// Remove any repeated prefixes that might occur
	for {
		switched := false
		for _, prefix := range []string{"registry.ollama.ai/", "library/", "models/"} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimPrefix(trimmed, prefix)
				switched = true
			}
		}
		if !switched {
			break
		}
	}

	return trimmed
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, value := range values {
		if value == "" {
			continue
		}
		clean := filepath.Clean(value)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
