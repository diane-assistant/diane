package emergent

import (
	"fmt"
	"sync"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
)

var (
	globalClient *sdk.Client
	globalConfig *Config
	mu           sync.Mutex
)

// GetClient returns a shared Emergent SDK client instance.
// The client is created lazily on first call and reused for all
// subsequent calls. It is safe for concurrent use.
func GetClient() (*sdk.Client, error) {
	mu.Lock()
	defer mu.Unlock()

	if globalClient != nil {
		return globalClient, nil
	}

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	globalClient = client
	globalConfig = cfg
	return globalClient, nil
}

// NewClient creates a new Emergent SDK client from the given config.
// Use this when you need a separate client instance (e.g., for testing).
// For normal usage, prefer GetClient() which returns a shared singleton.
func NewClient(cfg *Config) (*sdk.Client, error) {
	client, err := sdk.New(sdk.Config{
		ServerURL: cfg.BaseURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: cfg.APIKey,
		},
		// ProjectID omitted — API key is project-scoped
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Emergent client: %w", err)
	}
	return client, nil
}

// ResetClient clears the shared client singleton. This is primarily
// useful for testing or when configuration changes at runtime.
func ResetClient() {
	mu.Lock()
	defer mu.Unlock()
	globalClient = nil
	globalConfig = nil
}
