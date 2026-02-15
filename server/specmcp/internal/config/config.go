package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the SpecMCP server.
type Config struct {
	Emergent EmergentConfig
	Server   ServerConfig
	Log      LogConfig
}

// EmergentConfig holds Emergent connection details.
type EmergentConfig struct {
	URL       string
	Token     string // Project-scoped token (emt_*) or standalone API key.
	ProjectID string // Optional: explicit project ID (X-Project-ID header).
}

// ServerConfig holds MCP server metadata.
type ServerConfig struct {
	Name    string
	Version string
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string // debug, info, warn, error
}

// Load creates a Config by reading environment variables with defaults.
// Precedence: environment variables > defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Emergent: EmergentConfig{
			URL:       envOr("EMERGENT_URL", "http://localhost:3002"),
			Token:     envOr("EMERGENT_TOKEN", os.Getenv("EMERGENT_API_KEY")),
			ProjectID: os.Getenv("EMERGENT_PROJECT_ID"),
		},
		Server: ServerConfig{
			Name:    "specmcp",
			Version: "0.1.0",
		},
		Log: LogConfig{
			Level: envOr("SPECMCP_LOG_LEVEL", "info"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required fields are present.
func (c *Config) Validate() error {
	if c.Emergent.Token == "" {
		return fmt.Errorf("missing required environment variable: EMERGENT_TOKEN or EMERGENT_API_KEY")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
