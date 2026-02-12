// Package models provides a registry for AI model metadata from models.dev
package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// ModelsDevAPIURL is the URL for the models.dev API
	ModelsDevAPIURL = "https://models.dev/api.json"
	// CacheFileName is the name of the local cache file
	CacheFileName = "models-registry.json"
	// CacheTTL is how long the cache is valid
	CacheTTL = 24 * time.Hour
)

// Cost represents pricing for a model (per 1M tokens)
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

// Limit represents context and output limits
type Limit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// Modalities represents input/output modalities
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// Model represents a model's metadata
type Model struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Family      string     `json:"family,omitempty"`
	Attachment  bool       `json:"attachment,omitempty"`
	Reasoning   bool       `json:"reasoning,omitempty"`
	ToolCall    bool       `json:"tool_call,omitempty"`
	Temperature bool       `json:"temperature,omitempty"`
	Knowledge   string     `json:"knowledge,omitempty"`
	ReleaseDate string     `json:"release_date,omitempty"`
	LastUpdated string     `json:"last_updated,omitempty"`
	Modalities  Modalities `json:"modalities,omitempty"`
	OpenWeights bool       `json:"open_weights,omitempty"`
	Cost        Cost       `json:"cost,omitempty"`
	Limit       Limit      `json:"limit,omitempty"`
	LaunchStage string     `json:"launch_stage,omitempty"` // GA, preview, etc.
}

// Provider represents a provider with its models
type Provider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Doc    string           `json:"doc,omitempty"`
	NPM    string           `json:"npm,omitempty"`
	Env    []string         `json:"env,omitempty"`
	Models map[string]Model `json:"models"`
}

// Registry holds the model registry data
type Registry struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	lastFetched time.Time
	cacheDir    string
	httpClient  *http.Client
}

// CacheData represents the cached registry data
type CacheData struct {
	Providers map[string]Provider `json:"providers"`
	FetchedAt time.Time           `json:"fetched_at"`
}

// NewRegistry creates a new model registry
func NewRegistry(cacheDir string) *Registry {
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".diane")
	}
	return &Registry{
		providers:  make(map[string]Provider),
		cacheDir:   cacheDir,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// getCachePath returns the path to the cache file
func (r *Registry) getCachePath() string {
	return filepath.Join(r.cacheDir, CacheFileName)
}

// loadCache loads the registry from the local cache
func (r *Registry) loadCache() error {
	data, err := os.ReadFile(r.getCachePath())
	if err != nil {
		return err
	}

	var cache CacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	r.mu.Lock()
	r.providers = cache.Providers
	r.lastFetched = cache.FetchedAt
	r.mu.Unlock()

	return nil
}

// saveCache saves the registry to the local cache
func (r *Registry) saveCache() error {
	r.mu.RLock()
	cache := CacheData{
		Providers: r.providers,
		FetchedAt: r.lastFetched,
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.getCachePath(), data, 0644)
}

// Fetch downloads the latest registry from models.dev
func (r *Registry) Fetch() error {
	resp, err := r.httpClient.Get(ModelsDevAPIURL)
	if err != nil {
		return fmt.Errorf("failed to fetch models.dev: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("models.dev returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the response - it's a map of provider ID to provider data
	var rawProviders map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawProviders); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	providers := make(map[string]Provider)
	for id, raw := range rawProviders {
		var p Provider
		if err := json.Unmarshal(raw, &p); err != nil {
			// Skip providers that don't match our schema
			continue
		}
		p.ID = id
		if p.Models == nil {
			p.Models = make(map[string]Model)
		}
		providers[id] = p
	}

	r.mu.Lock()
	r.providers = providers
	r.lastFetched = time.Now()
	r.mu.Unlock()

	// Save to cache
	return r.saveCache()
}

// Load loads the registry, using cache if valid, otherwise fetching
func (r *Registry) Load() error {
	// Try to load from cache first
	if err := r.loadCache(); err == nil {
		// Check if cache is still valid
		r.mu.RLock()
		cacheValid := time.Since(r.lastFetched) < CacheTTL
		r.mu.RUnlock()

		if cacheValid {
			return nil
		}
	}

	// Fetch fresh data
	if err := r.Fetch(); err != nil {
		// If we have cached data (even if stale), use it
		r.mu.RLock()
		hasData := len(r.providers) > 0
		r.mu.RUnlock()
		if hasData {
			return nil // Use stale cache
		}
		return err
	}

	return nil
}

// GetProvider returns a provider by ID
func (r *Registry) GetProvider(providerID string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	return p, ok
}

// GetModel returns a model by provider ID and model ID
// If exact match isn't found, tries to strip version suffixes (e.g., -001, -002)
func (r *Registry) GetModel(providerID, modelID string) (Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	if !ok {
		return Model{}, false
	}

	// Try exact match first
	if m, ok := p.Models[modelID]; ok {
		return m, true
	}

	// Try without version suffix (e.g., gemini-2.0-flash-001 -> gemini-2.0-flash)
	modelIDWithoutVersion := stripVersionSuffix(modelID)
	if modelIDWithoutVersion != modelID {
		if m, ok := p.Models[modelIDWithoutVersion]; ok {
			return m, true
		}
	}

	return Model{}, false
}

// ListProviders returns all provider IDs
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

// ListModels returns all models for a provider
func (r *Registry) ListModels(providerID string) []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	if !ok {
		return nil
	}
	models := make([]Model, 0, len(p.Models))
	for _, m := range p.Models {
		models = append(models, m)
	}
	return models
}

// CalculateCost calculates the cost for a given usage
func (r *Registry) CalculateCost(providerID, modelID string, inputTokens, outputTokens int) (float64, error) {
	model, ok := r.GetModel(providerID, modelID)
	if !ok {
		return 0, fmt.Errorf("model not found: %s/%s", providerID, modelID)
	}

	// Cost is per 1M tokens
	inputCost := float64(inputTokens) / 1_000_000 * model.Cost.Input
	outputCost := float64(outputTokens) / 1_000_000 * model.Cost.Output

	return inputCost + outputCost, nil
}

// ProviderIDForService maps Diane service names to models.dev provider IDs
var ProviderIDForService = map[string]string{
	"vertex_ai":     "google-vertex",
	"vertex_ai_llm": "google-vertex",
	"openai":        "openai",
	"anthropic":     "anthropic",
	"google":        "google",
	"ollama":        "ollama-cloud",
}

// GetProviderIDForService returns the models.dev provider ID for a Diane service
func GetProviderIDForService(service string) string {
	if id, ok := ProviderIDForService[service]; ok {
		return id
	}
	return service
}

// stripVersionSuffix removes version suffixes like -001, -002, -v1, -v2 from model IDs
// Examples: gemini-2.0-flash-001 -> gemini-2.0-flash
//
//	gpt-4-0125-preview -> gpt-4-0125-preview (no change, not a simple version)
func stripVersionSuffix(modelID string) string {
	// Common version suffix patterns: -NNN (e.g., -001, -002)
	// Look for pattern: -\d{3,4}$ at the end
	if len(modelID) < 4 {
		return modelID
	}

	// Check if ends with -XXX where XXX is 3-4 digits
	for suffixLen := 3; suffixLen <= 4; suffixLen++ {
		if len(modelID) > suffixLen+1 {
			suffix := modelID[len(modelID)-suffixLen:]
			dash := modelID[len(modelID)-suffixLen-1]

			if dash == '-' && isAllDigits(suffix) {
				return modelID[:len(modelID)-suffixLen-1]
			}
		}
	}

	return modelID
}

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
