package db

// TODO(emergent-migration): Provider entity is being migrated to Emergent.
// Phase 1 (dual-write) is COMPLETE — see internal/store/provider_*.go.
// Remaining phases:
//   Phase 2: Switch reads from SQLite to Emergent (swap primary/secondary in DualWriteProviderStore)
//   Phase 3: Remove SQLite writes, drop DualWriteProviderStore, use EmergentProviderStore directly
//   Phase 4: Remove this file and all SQLite provider methods from *DB
//   Phase 5: Run one-time data migration script to seed Emergent from SQLite
// Tracking: docs/EMERGENT_MIGRATION_PLAN.md

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ProviderType represents different provider categories
type ProviderType string

const (
	ProviderTypeEmbedding ProviderType = "embedding"
	ProviderTypeLLM       ProviderType = "llm"
	ProviderTypeStorage   ProviderType = "storage"
)

// AuthType represents authentication methods
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

// Provider represents a configured service provider
type Provider struct {
	ID         int64
	Name       string
	Type       ProviderType
	Service    string // e.g., "vertex_ai", "openai", "ollama"
	Enabled    bool
	IsDefault  bool
	AuthType   AuthType
	AuthConfig map[string]any // API key, OAuth account, etc.
	Config     map[string]any // Provider-specific settings
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ProviderTemplate defines a template for a known service
type ProviderTemplate struct {
	Service      string        `json:"service"`
	Name         string        `json:"name"`
	Type         ProviderType  `json:"type"`
	AuthType     AuthType      `json:"auth_type"`
	OAuthScopes  []string      `json:"oauth_scopes,omitempty"`
	ConfigSchema []ConfigField `json:"config_schema"`
	Description  string        `json:"description"`
}

// ConfigField defines a configuration field
type ConfigField struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Type          string   `json:"type"` // string, int, bool, select, dynamic_select
	Required      bool     `json:"required"`
	Default       any      `json:"default,omitempty"`
	Options       []string `json:"options,omitempty"`
	DynamicSource string   `json:"dynamic_source,omitempty"` // For dynamic_select: provider ID from models.dev
	Description   string   `json:"description,omitempty"`
}

// GetProviderTemplates returns available provider templates
func GetProviderTemplates() []ProviderTemplate {
	return []ProviderTemplate{
		{
			Service:     "vertex_ai",
			Name:        "Google Vertex AI",
			Type:        ProviderTypeEmbedding,
			AuthType:    AuthTypeOAuth,
			OAuthScopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			Description: "Google Cloud Vertex AI for text embeddings",
			ConfigSchema: []ConfigField{
				{
					Key:         "project_id",
					Label:       "GCP Project ID",
					Type:        "string",
					Required:    true,
					Description: "Your Google Cloud project ID",
				},
				{
					Key:         "location",
					Label:       "Location",
					Type:        "select",
					Required:    true,
					Default:     "us-central1",
					Options:     []string{"global", "us-central1", "us-east1", "us-west1", "us-west4", "europe-west1", "europe-west4", "asia-east1", "asia-northeast1"},
					Description: "Vertex AI region (use 'global' for multi-region)",
				},
				{
					Key:         "model",
					Label:       "Embedding Model",
					Type:        "select",
					Required:    true,
					Default:     "text-embedding-005",
					Options:     []string{"text-embedding-005", "text-embedding-004", "text-multilingual-embedding-002"},
					Description: "Embedding model to use",
				},
			},
		},
		{
			Service:     "openai",
			Name:        "OpenAI",
			Type:        ProviderTypeEmbedding,
			AuthType:    AuthTypeAPIKey,
			Description: "OpenAI embeddings API",
			ConfigSchema: []ConfigField{
				{
					Key:         "model",
					Label:       "Embedding Model",
					Type:        "select",
					Required:    true,
					Default:     "text-embedding-3-small",
					Options:     []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"},
					Description: "Embedding model to use",
				},
				{
					Key:         "dimensions",
					Label:       "Dimensions",
					Type:        "int",
					Required:    false,
					Default:     1536,
					Description: "Output vector dimensions (only for embedding-3 models)",
				},
			},
		},
		{
			Service:     "ollama",
			Name:        "Ollama (Local)",
			Type:        ProviderTypeEmbedding,
			AuthType:    AuthTypeNone,
			Description: "Local Ollama instance for embeddings",
			ConfigSchema: []ConfigField{
				{
					Key:         "base_url",
					Label:       "Base URL",
					Type:        "string",
					Required:    true,
					Default:     "http://localhost:11434",
					Description: "Ollama server URL",
				},
				{
					Key:         "model",
					Label:       "Model",
					Type:        "string",
					Required:    true,
					Default:     "nomic-embed-text",
					Description: "Embedding model name",
				},
			},
		},
		// LLM Providers
		{
			Service:     "vertex_ai_llm",
			Name:        "Google Vertex AI (LLM)",
			Type:        ProviderTypeLLM,
			AuthType:    AuthTypeOAuth,
			OAuthScopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			Description: "Google Cloud Vertex AI for language model inference",
			ConfigSchema: []ConfigField{
				{
					Key:         "project_id",
					Label:       "GCP Project ID",
					Type:        "string",
					Required:    true,
					Description: "Your Google Cloud project ID",
				},
				{
					Key:         "location",
					Label:       "Location",
					Type:        "select",
					Required:    true,
					Default:     "us-central1",
					Options:     []string{"global", "us-central1", "us-east1", "us-west1", "us-west4", "europe-west1", "europe-west4", "asia-east1", "asia-northeast1"},
					Description: "Vertex AI region (use 'global' for multi-region)",
				},
				{
					Key:           "model",
					Label:         "Model",
					Type:          "dynamic_select",
					Required:      true,
					Default:       "gemini-2.5-flash",
					DynamicSource: "google-vertex",
					Description:   "LLM model to use (fetched from models.dev)",
				},
				{
					Key:         "max_tokens",
					Label:       "Max Output Tokens",
					Type:        "int",
					Required:    false,
					Default:     8192,
					Description: "Maximum number of tokens in the response",
				},
				{
					Key:         "temperature",
					Label:       "Temperature",
					Type:        "string",
					Required:    false,
					Default:     "1.0",
					Description: "Sampling temperature (0.0 to 2.0)",
				},
			},
		},
	}
}

// =============================================================================
// Provider CRUD Operations
// =============================================================================

// CreateProvider creates a new provider
func (db *DB) CreateProvider(p *Provider) (int64, error) {
	authConfigJSON, err := json.Marshal(p.AuthConfig)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal auth config: %w", err)
	}

	configJSON, err := json.Marshal(p.Config)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal config: %w", err)
	}

	// If this is the first provider of its type, make it default
	if !p.IsDefault {
		var count int
		err := db.conn.QueryRow(
			"SELECT COUNT(*) FROM providers WHERE type = ? AND enabled = 1",
			p.Type,
		).Scan(&count)
		if err == nil && count == 0 {
			p.IsDefault = true
		}
	}

	result, err := db.conn.Exec(`
		INSERT INTO providers (name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Type, p.Service, p.Enabled, p.IsDefault, p.AuthType,
		string(authConfigJSON), string(configJSON), time.Now(), time.Now(),
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetProvider retrieves a provider by ID
func (db *DB) GetProvider(id int64) (*Provider, error) {
	p := &Provider{}
	var authConfigJSON, configJSON string

	err := db.conn.QueryRow(`
		SELECT id, name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at
		FROM providers WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Service, &p.Enabled, &p.IsDefault, &p.AuthType,
		&authConfigJSON, &configJSON, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if authConfigJSON != "" {
		if err := json.Unmarshal([]byte(authConfigJSON), &p.AuthConfig); err != nil {
			return nil, fmt.Errorf("failed to parse auth config: %w", err)
		}
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &p.Config); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	return p, nil
}

// GetProviderByName retrieves a provider by name
func (db *DB) GetProviderByName(name string) (*Provider, error) {
	p := &Provider{}
	var authConfigJSON, configJSON string

	err := db.conn.QueryRow(`
		SELECT id, name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at
		FROM providers WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Service, &p.Enabled, &p.IsDefault, &p.AuthType,
		&authConfigJSON, &configJSON, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if authConfigJSON != "" {
		if err := json.Unmarshal([]byte(authConfigJSON), &p.AuthConfig); err != nil {
			return nil, fmt.Errorf("failed to parse auth config: %w", err)
		}
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &p.Config); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	return p, nil
}

// ListProviders returns all providers
func (db *DB) ListProviders() ([]*Provider, error) {
	return db.listProvidersWithFilter("")
}

// ListProvidersByType returns providers of a specific type
func (db *DB) ListProvidersByType(ptype ProviderType) ([]*Provider, error) {
	return db.listProvidersWithFilter(fmt.Sprintf("WHERE type = '%s'", ptype))
}

// ListEnabledProvidersByType returns enabled providers of a specific type
func (db *DB) ListEnabledProvidersByType(ptype ProviderType) ([]*Provider, error) {
	return db.listProvidersWithFilter(fmt.Sprintf("WHERE type = '%s' AND enabled = 1", ptype))
}

func (db *DB) listProvidersWithFilter(filter string) ([]*Provider, error) {
	query := `
		SELECT id, name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at
		FROM providers ` + filter + ` ORDER BY is_default DESC, name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		var authConfigJSON, configJSON string

		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Service, &p.Enabled, &p.IsDefault, &p.AuthType,
			&authConfigJSON, &configJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}

		if authConfigJSON != "" {
			if err := json.Unmarshal([]byte(authConfigJSON), &p.AuthConfig); err != nil {
				return nil, fmt.Errorf("failed to parse auth config: %w", err)
			}
		}
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &p.Config); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
		}

		providers = append(providers, p)
	}

	return providers, rows.Err()
}

// GetDefaultProvider returns the default provider for a type
func (db *DB) GetDefaultProvider(ptype ProviderType) (*Provider, error) {
	p := &Provider{}
	var authConfigJSON, configJSON string

	// Try to find explicit default first
	err := db.conn.QueryRow(`
		SELECT id, name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at
		FROM providers WHERE type = ? AND enabled = 1 AND is_default = 1
		LIMIT 1`, ptype,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Service, &p.Enabled, &p.IsDefault, &p.AuthType,
		&authConfigJSON, &configJSON, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		// Fall back to first enabled provider of that type
		err = db.conn.QueryRow(`
			SELECT id, name, type, service, enabled, is_default, auth_type, auth_config, config, created_at, updated_at
			FROM providers WHERE type = ? AND enabled = 1
			ORDER BY created_at LIMIT 1`, ptype,
		).Scan(&p.ID, &p.Name, &p.Type, &p.Service, &p.Enabled, &p.IsDefault, &p.AuthType,
			&authConfigJSON, &configJSON, &p.CreatedAt, &p.UpdatedAt)
	}

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if authConfigJSON != "" {
		if err := json.Unmarshal([]byte(authConfigJSON), &p.AuthConfig); err != nil {
			return nil, fmt.Errorf("failed to parse auth config: %w", err)
		}
	}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &p.Config); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	return p, nil
}

// UpdateProvider updates an existing provider
func (db *DB) UpdateProvider(p *Provider) error {
	authConfigJSON, err := json.Marshal(p.AuthConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal auth config: %w", err)
	}

	configJSON, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	_, err = db.conn.Exec(`
		UPDATE providers SET
			name = ?, type = ?, service = ?, enabled = ?, is_default = ?,
			auth_type = ?, auth_config = ?, config = ?, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Type, p.Service, p.Enabled, p.IsDefault,
		p.AuthType, string(authConfigJSON), string(configJSON), time.Now(), p.ID,
	)
	return err
}

// DeleteProvider removes a provider
func (db *DB) DeleteProvider(id int64) error {
	_, err := db.conn.Exec("DELETE FROM providers WHERE id = ?", id)
	return err
}

// SetDefaultProvider sets a provider as the default for its type
func (db *DB) SetDefaultProvider(id int64) error {
	// Get the provider to find its type
	p, err := db.GetProvider(id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("provider not found: %d", id)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset default for all providers of this type
	_, err = tx.Exec("UPDATE providers SET is_default = 0, updated_at = ? WHERE type = ?",
		time.Now(), p.Type)
	if err != nil {
		return err
	}

	// Set the new default
	_, err = tx.Exec("UPDATE providers SET is_default = 1, updated_at = ? WHERE id = ?",
		time.Now(), id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// EnableProvider enables a provider
func (db *DB) EnableProvider(id int64) error {
	_, err := db.conn.Exec("UPDATE providers SET enabled = 1, updated_at = ? WHERE id = ?",
		time.Now(), id)
	return err
}

// DisableProvider disables a provider
func (db *DB) DisableProvider(id int64) error {
	_, err := db.conn.Exec("UPDATE providers SET enabled = 0, updated_at = ? WHERE id = ?",
		time.Now(), id)
	return err
}

// =============================================================================
// Helper functions for provider config
// =============================================================================

// GetConfigString returns a string value from provider config
func (p *Provider) GetConfigString(key string) string {
	if p.Config == nil {
		return ""
	}
	if v, ok := p.Config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetConfigInt returns an int value from provider config
func (p *Provider) GetConfigInt(key string) int {
	if p.Config == nil {
		return 0
	}
	if v, ok := p.Config[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetConfigBool returns a bool value from provider config
func (p *Provider) GetConfigBool(key string) bool {
	if p.Config == nil {
		return false
	}
	if v, ok := p.Config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetAuthString returns a string value from auth config
func (p *Provider) GetAuthString(key string) string {
	if p.AuthConfig == nil {
		return ""
	}
	if v, ok := p.AuthConfig[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
