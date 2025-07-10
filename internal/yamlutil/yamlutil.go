package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// AppEmojis is a curated list of app-appropriate emojis
var AppEmojis = []string{
	// Development & Technology
	"💻", "🖥️", "⌨️", "🖱️", "💾", "💿", "📱", "☁️", "🌐", "📡",
	"🔌", "🔋", "🛠️", "⚙️", "🔧", "🔨", "⚡", "🚀", "🛸", "🤖",

	// Data & Analytics
	"📊", "📈", "📉", "📋", "📌", "📍", "🗂️", "🗄️", "📁", "📂",
	"💹", "🔍", "🔎", "🧮", "💡", "🎯", "📐", "📏", "🗺️", "🧭",

	// Security & Monitoring
	"🔒", "🔓", "🔐", "🔑", "🛡️", "⚠️", "🚨", "📢", "🔔", "👁️",
	"🕵️", "🚦", "🚥", "⏰", "⏱️", "⌚", "📅", "📆", "🕐", "🌡️",

	// Communication & Media
	"📧", "📨", "📩", "💬", "💭", "🗨️", "📞", "☎️", "📠", "📻",
	"📺", "📷", "📹", "🎥", "🎬", "🎤", "🎧", "🎵", "🎶", "📣",

	// Storage & Database
	"🗃️", "🗳️", "📦", "📮", "📪", "📫", "📬", "📭", "🏗️", "🏭",
	"🏪", "🏬", "🏦", "💳", "💰", "💸", "🪙", "💎", "⚖️", "🔗",

	// Nature & Science
	"🌍", "🌎", "🌏", "🌐", "🪐", "🌙", "☀️", "⭐", "🌟", "✨",
	"🔬", "🔭", "🧬", "🧪", "⚗️", "🧫", "🦠", "🧲", "⚛️", "🌡️",
}

// IsValidEmoji checks if the given emoji is in the allowed list
func IsValidEmoji(emoji string) bool {
	if emoji == "" {
		// Empty emoji is valid (optional field)
		return true
	}

	for _, allowed := range AppEmojis {
		if emoji == allowed {
			return true
		}
	}
	return false
}

// fileLocks manages file-level locking for concurrent access
var fileLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{
	locks: make(map[string]*sync.Mutex),
}

// getFileLock returns a mutex for the given file path
func getFileLock(path string) *sync.Mutex {
	fileLocks.Lock()
	defer fileLocks.Unlock()

	absPath, _ := filepath.Abs(path)
	if lock, exists := fileLocks.locks[absPath]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	fileLocks.locks[absPath] = lock
	return lock
}

// OnTreeMetadata represents the OnTree-specific metadata stored in docker-compose.yml
type OnTreeMetadata struct {
	Subdomain string `yaml:"subdomain,omitempty"`
	HostPort  int    `yaml:"host_port,omitempty"`
	IsExposed bool   `yaml:"is_exposed"`
	Emoji     string `yaml:"emoji,omitempty"`
}

// ComposeFile represents a docker-compose.yml file structure
type ComposeFile struct {
	Version  string                 `yaml:"version"`
	Services map[string]interface{} `yaml:"services"`
	XOnTree  *OnTreeMetadata        `yaml:"x-ontree,omitempty"`
	// Preserve other fields as raw YAML nodes to maintain formatting
	raw map[string]*yaml.Node `yaml:"-"`
}

// ReadComposeWithMetadata reads a docker-compose.yml file preserving formatting and comments
func ReadComposeWithMetadata(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// First, unmarshal into our structured type
	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Also parse as a generic map to preserve all fields
	var genericMap map[string]interface{}
	if err := yaml.Unmarshal(data, &genericMap); err != nil {
		return nil, fmt.Errorf("failed to parse YAML as map: %w", err)
	}

	// Store any additional fields that aren't in our struct
	compose.raw = make(map[string]*yaml.Node)

	return &compose, nil
}

// WriteComposeWithMetadata writes a docker-compose.yml file preserving formatting and comments
func WriteComposeWithMetadata(path string, compose *ComposeFile) error {
	// Get file lock for this path
	lock := getFileLock(path)
	lock.Lock()
	defer lock.Unlock()

	// For preserving comments and formatting, we need to work with yaml.Node
	// Read the original file if it exists to preserve its structure
	var node yaml.Node

	if _, err := os.Stat(path); err == nil {
		// File exists, read it to preserve structure
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read existing file: %w", err)
		}

		if err := yaml.Unmarshal(data, &node); err != nil {
			return fmt.Errorf("failed to parse existing YAML: %w", err)
		}

		// Update the node with our new values
		if err := updateYAMLNode(&node, compose); err != nil {
			return fmt.Errorf("failed to update YAML node: %w", err)
		}
	} else {
		// File doesn't exist, create new structure
		if err := node.Encode(compose); err != nil {
			return fmt.Errorf("failed to encode compose file: %w", err)
		}
	}

	// Marshal the node back to YAML
	output, err := yaml.Marshal(&node)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file atomically
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Rename to final location
	if err := os.Rename(tempFile, path); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// GetOnTreeMetadata extracts OnTree metadata from a compose file
func GetOnTreeMetadata(compose *ComposeFile) *OnTreeMetadata {
	if compose.XOnTree == nil {
		return &OnTreeMetadata{}
	}
	return compose.XOnTree
}

// SetOnTreeMetadata sets OnTree metadata in a compose file
func SetOnTreeMetadata(compose *ComposeFile, metadata *OnTreeMetadata) {
	compose.XOnTree = metadata
}

// updateYAMLNode updates a YAML node with values from ComposeFile while preserving structure
func updateYAMLNode(node *yaml.Node, compose *ComposeFile) error {
	// This is a document node, we need to work with its content
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	// Ensure it's a mapping node
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got %v", node.Kind)
	}

	// Update or add x-ontree field
	updated := false
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Value == "x-ontree" {
			// Update existing x-ontree
			valueNode := node.Content[i+1]
			if err := valueNode.Encode(compose.XOnTree); err != nil {
				return fmt.Errorf("failed to encode x-ontree: %w", err)
			}
			updated = true
			break
		}
	}

	// If x-ontree wasn't found and we have metadata, add it
	if !updated && compose.XOnTree != nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "x-ontree"}
		valueNode := &yaml.Node{}
		if err := valueNode.Encode(compose.XOnTree); err != nil {
			return fmt.Errorf("failed to encode x-ontree: %w", err)
		}

		// Add after version field if possible
		insertIndex := len(node.Content)
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == "version" {
				insertIndex = i + 2
				break
			}
		}

		// Insert at the determined position
		node.Content = append(node.Content[:insertIndex],
			append([]*yaml.Node{keyNode, valueNode}, node.Content[insertIndex:]...)...)
	}

	return nil
}

// ReadComposeMetadata is a helper function that reads only the OnTree metadata from a compose file
func ReadComposeMetadata(appPath string) (*OnTreeMetadata, error) {
	composePath := filepath.Join(appPath, "docker-compose.yml")
	compose, err := ReadComposeWithMetadata(composePath)
	if err != nil {
		return nil, err
	}
	return GetOnTreeMetadata(compose), nil
}

// UpdateComposeMetadata is a helper function that updates only the OnTree metadata in a compose file
func UpdateComposeMetadata(appPath string, metadata *OnTreeMetadata) error {
	composePath := filepath.Join(appPath, "docker-compose.yml")

	// Note: WriteComposeWithMetadata handles its own locking
	compose, err := ReadComposeWithMetadata(composePath)
	if err != nil {
		return err
	}
	SetOnTreeMetadata(compose, metadata)
	return WriteComposeWithMetadata(composePath, compose)
}

// ValidateComposeFile validates the YAML syntax and structure of a docker-compose file
func ValidateComposeFile(content string) error {
	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	// Check required fields
	if compose.Version == "" {
		return fmt.Errorf("missing 'version' field")
	}

	if compose.Services == nil || len(compose.Services) == 0 {
		return fmt.Errorf("missing 'services' section")
	}

	// Validate each service has basic structure
	for name, service := range compose.Services {
		svcMap, ok := service.(map[string]interface{})
		if !ok {
			return fmt.Errorf("service '%s' has invalid structure", name)
		}

		// Check for image or build
		_, hasImage := svcMap["image"]
		_, hasBuild := svcMap["build"]
		if !hasImage && !hasBuild {
			return fmt.Errorf("service '%s' must have either 'image' or 'build' field", name)
		}
	}

	return nil
}
