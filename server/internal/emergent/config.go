// Package emergent provides a shared Emergent SDK client for all Diane components.
//
// Configuration is read from ~/.diane/secrets/emergent-config.json first,
// falling back to environment variables:
//   - EMERGENT_BASE_URL (default: http://localhost:3002)
//   - EMERGENT_API_KEY (required)
//
// The API key is project-scoped, so no project ID is needed.
package emergent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultBaseURL is the default Emergent server URL for local development.
	DefaultBaseURL = "http://localhost:3002"

	// configFileName is the name of the config file under ~/.diane/secrets/
	configFileName = "emergent-config.json"
)

// Config holds Emergent connection configuration.
// The API key is project-scoped — Emergent automatically determines
// which project the key belongs to, so no ProjectID is needed.
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// configFile represents the JSON structure of the config file.
// It supports a legacy project_id field for backward compatibility,
// but this field is ignored.
type configFile struct {
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	ProjectID string `json:"project_id,omitempty"` // Ignored — API key determines project
}

// LoadConfig reads Emergent configuration from the config file and
// environment variables. File values take precedence; env vars fill
// in any missing values.
//
// Returns an error only if the API key is missing from both sources.
func LoadConfig() (*Config, error) {
	return loadConfigFrom("", "")
}

// loadConfigFrom is the internal implementation that accepts overrides
// for testing. Pass empty strings to use real sources.
func loadConfigFrom(configPath, envPrefix string) (*Config, error) {
	cfg := &Config{}

	// Determine config file path
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(home, ".diane", "secrets", configFileName)
		}
	}

	// Try config file first
	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var f configFile
			if err := json.Unmarshal(data, &f); err == nil {
				cfg.BaseURL = f.BaseURL
				cfg.APIKey = f.APIKey
			}
		}
	}

	// Fall back to env vars for any missing values
	baseURLEnv := "EMERGENT_BASE_URL"
	apiKeyEnv := "EMERGENT_API_KEY"
	if envPrefix != "" {
		baseURLEnv = envPrefix + "_BASE_URL"
		apiKeyEnv = envPrefix + "_API_KEY"
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv(baseURLEnv)
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv(apiKeyEnv)
	}

	// Apply defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}

	// Validate
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Emergent API key not configured. Create ~/.diane/secrets/%s or set EMERGENT_API_KEY", configFileName)
	}

	return cfg, nil
}
