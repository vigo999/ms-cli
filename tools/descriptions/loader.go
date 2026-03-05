package descriptions

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loader loads tool descriptions from external files
type Loader struct {
	// embeddedDescriptions contains embedded description files
	// Use a pointer to allow nil (no embedded files)
	embeddedDescriptions *embed.FS

	// externalDir is an optional external directory for user-defined descriptions
	externalDir string
}

// NewLoader creates a new description loader
func NewLoader(embedded embed.FS, externalDir string) *Loader {
	return &Loader{
		embeddedDescriptions: &embedded,
		externalDir:          externalDir,
	}
}

// NewLoaderWithoutEmbed creates a loader without embedded files (for backward compatibility)
func NewLoaderWithoutEmbed(externalDir string) *Loader {
	return &Loader{
		embeddedDescriptions: nil,
		externalDir:          externalDir,
	}
}

// Load loads a description by tool name
// Searches in order: externalDir -> embedded
func (l *Loader) Load(toolName string) (string, error) {
	// First try external directory
	if l.externalDir != "" {
		path := filepath.Join(l.externalDir, fmt.Sprintf("%s.txt", toolName))
		if data, err := os.ReadFile(path); err == nil {
			return string(data), nil
		}
	}

	// Then try embedded
	if l.embeddedDescriptions != nil {
		path := fmt.Sprintf("%s.txt", toolName)
		if data, err := l.embeddedDescriptions.ReadFile(path); err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("description not found for tool: %s", toolName)
}

// LoadWithFallback loads description, falling back to provided default
func (l *Loader) LoadWithFallback(toolName, fallback string) string {
	desc, err := l.Load(toolName)
	if err != nil {
		return fallback
	}
	return desc
}

// ListAvailable lists all available description files
func (l *Loader) ListAvailable() []string {
	var names []string

	// List embedded
	if l.embeddedDescriptions != nil {
		entries, _ := l.embeddedDescriptions.ReadDir(".")
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".txt") {
				names = append(names, strings.TrimSuffix(entry.Name(), ".txt"))
			}
		}
	}

	// List external
	if l.externalDir != "" {
		entries, _ := os.ReadDir(l.externalDir)
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".txt") {
				name := strings.TrimSuffix(entry.Name(), ".txt")
				// Avoid duplicates
				found := false
				for _, n := range names {
					if n == name {
						found = true
						break
					}
				}
				if !found {
					names = append(names, name)
				}
			}
		}
	}

	return names
}
