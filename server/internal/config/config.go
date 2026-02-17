package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

// Config holds Diane server configuration.
// Loaded from ~/.diane/config.json with environment variable overrides.
type Config struct {
	// HTTP is the TCP address for the remote API listener (e.g., ":8080").
	// Env override: DIANE_HTTP_ADDR
	HTTP HTTPConfig `json:"http"`

	// Debug enables debug-level logging.
	// Env override: DIANE_DEBUG=1
	Debug bool `json:"debug"`

	// Slave configuration for connecting to a master server
	Slave SlaveConfig `json:"slave"`
}

// HTTPConfig holds settings for the optional TCP HTTP listener.
type HTTPConfig struct {
	// Port is the TCP port for the remote API listener (e.g., 8080).
	// If 0, the TCP listener is disabled.
	Port int `json:"port"`

	// APIKey is the secret key for authenticating remote API requests.
	// When set, all requests must include "Authorization: Bearer <api_key>".
	// When empty and Port is set, the listener is read-only (GET/HEAD only).
	APIKey string `json:"api_key"`
}

// SlaveConfig holds settings for connecting to a master server as a slave.
type SlaveConfig struct {
	// Enabled determines whether this instance should connect to a master as a slave.
	Enabled bool `json:"enabled"`

	// MasterURL is the WebSocket URL of the master server (e.g., "wss://master:8766").
	MasterURL string `json:"master_url"`
}

// Load reads configuration from the config file, then applies
// environment variable overrides. Config file locations checked in order:
//  1. DIANE_CONFIG env var (if set)
//  2. ~/.diane/config.json
//
// Missing file is not an error.
func Load() Config {
	var cfg Config

	configPath := os.Getenv("DIANE_CONFIG")
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Warn("Failed to get home directory for config", "error", err)
			applyEnvOverrides(&cfg)
			return cfg
		}
		configPath = filepath.Join(home, ".diane", "config.json")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read config file", "path", configPath, "error", err)
		}
		// No config file — env vars only
		applyEnvOverrides(&cfg)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("Failed to parse config file", "path", configPath, "error", err)
		// Fall through with zero config + env overrides
	}

	applyEnvOverrides(&cfg)
	return cfg
}

// applyEnvOverrides applies environment variable overrides to the config.
// Env vars take precedence over config file values.
func applyEnvOverrides(cfg *Config) {
	if os.Getenv("DIANE_DEBUG") == "1" {
		cfg.Debug = true
	}

	// DIANE_HTTP_ADDR overrides port (for backward compatibility)
	// e.g., DIANE_HTTP_ADDR=":8080" sets port to 8080
	if addr := os.Getenv("DIANE_HTTP_ADDR"); addr != "" {
		// Parse port from address like ":8080" or "0.0.0.0:8080"
		port := parsePort(addr)
		if port > 0 {
			cfg.HTTP.Port = port
		}
	}

	// DIANE_API_KEY overrides api_key
	if key := os.Getenv("DIANE_API_KEY"); key != "" {
		cfg.HTTP.APIKey = key
	}
}

// parsePort extracts the port number from an address string like ":8080" or "0.0.0.0:8080".
func parsePort(addr string) int {
	// Find the last colon
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			portStr := addr[i+1:]
			port := 0
			for _, c := range portStr {
				if c < '0' || c > '9' {
					return 0
				}
				port = port*10 + int(c-'0')
			}
			return port
		}
	}
	return 0
}

// HTTPAddr returns the TCP listen address string (e.g., ":8080") or empty string if disabled.
func (c *Config) HTTPAddr() string {
	if c.HTTP.Port == 0 {
		return ""
	}
	return ":" + portToString(c.HTTP.Port)
}

func portToString(port int) string {
	if port <= 0 {
		return "0"
	}
	s := ""
	for port > 0 {
		s = string(rune('0'+port%10)) + s
		port /= 10
	}
	return s
}
