package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/models"
	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"golang.org/x/oauth2"
)

// ProvidersAPI handles provider-related API endpoints
type ProvidersAPI struct {
	db       *db.DB
	registry *models.Registry
}

// NewProvidersAPI creates a new ProvidersAPI
func NewProvidersAPI(database *db.DB, registry *models.Registry) *ProvidersAPI {
	return &ProvidersAPI{db: database, registry: registry}
}

// RegisterRoutes registers the provider API routes
func (api *ProvidersAPI) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/providers", api.handleProviders)
	mux.HandleFunc("/providers/templates", api.handleProviderTemplates)
	mux.HandleFunc("/providers/models", api.handleListModels)
	mux.HandleFunc("/providers/", api.handleProviderAction)

	// Models registry endpoints (from models.dev)
	mux.HandleFunc("/models", api.handleModelsRegistry)
	mux.HandleFunc("/models/", api.handleModelDetails)

	// Usage tracking endpoints
	mux.HandleFunc("/usage", api.handleUsage)
	mux.HandleFunc("/usage/summary", api.handleUsageSummary)

	// Google OAuth endpoints
	mux.HandleFunc("/google/auth", api.handleGoogleAuth)
	mux.HandleFunc("/google/auth/start", api.handleGoogleAuthStart)
	mux.HandleFunc("/google/auth/poll", api.handleGoogleAuthPoll)
}

// ProviderResponse is the API response format for a provider
type ProviderResponse struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Service    string         `json:"service"`
	Enabled    bool           `json:"enabled"`
	IsDefault  bool           `json:"is_default"`
	AuthType   string         `json:"auth_type"`
	AuthConfig map[string]any `json:"auth_config,omitempty"`
	Config     map[string]any `json:"config"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// CreateProviderRequest is the request format for creating a provider
type CreateProviderRequest struct {
	Name       string         `json:"name"`
	Service    string         `json:"service"`
	Type       string         `json:"type,omitempty"` // Optional, can be inferred from service
	AuthType   string         `json:"auth_type,omitempty"`
	AuthConfig map[string]any `json:"auth_config,omitempty"`
	Config     map[string]any `json:"config"`
}

// UpdateProviderRequest is the request format for updating a provider
type UpdateProviderRequest struct {
	Name       *string         `json:"name,omitempty"`
	Enabled    *bool           `json:"enabled,omitempty"`
	IsDefault  *bool           `json:"is_default,omitempty"`
	AuthConfig *map[string]any `json:"auth_config,omitempty"`
	Config     *map[string]any `json:"config,omitempty"`
}

func providerToResponse(p *db.Provider, maskSecrets bool) ProviderResponse {
	authConfig := p.AuthConfig
	if maskSecrets && authConfig != nil {
		// Mask sensitive fields
		masked := make(map[string]any)
		for k, v := range authConfig {
			if k == "api_key" || strings.Contains(strings.ToLower(k), "secret") {
				if s, ok := v.(string); ok && len(s) > 8 {
					masked[k] = s[:4] + "****" + s[len(s)-4:]
				} else {
					masked[k] = "****"
				}
			} else {
				masked[k] = v
			}
		}
		authConfig = masked
	}

	return ProviderResponse{
		ID:         p.ID,
		Name:       p.Name,
		Type:       string(p.Type),
		Service:    p.Service,
		Enabled:    p.Enabled,
		IsDefault:  p.IsDefault,
		AuthType:   string(p.AuthType),
		AuthConfig: authConfig,
		Config:     p.Config,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}

// handleProviders handles GET /providers and POST /providers
func (api *ProvidersAPI) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listProviders(w, r)
	case http.MethodPost:
		api.createProvider(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (api *ProvidersAPI) listProviders(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")

	var providers []*db.Provider
	var err error

	if typeFilter != "" {
		providers, err = api.db.ListProvidersByType(db.ProviderType(typeFilter))
	} else {
		providers, err = api.db.ListProviders()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]ProviderResponse, len(providers))
	for i, p := range providers {
		response[i] = providerToResponse(p, true)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (api *ProvidersAPI) createProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Service == "" {
		http.Error(w, "service is required", http.StatusBadRequest)
		return
	}

	// Try to infer type and auth from template if not provided
	providerType := db.ProviderType(req.Type)
	authType := db.AuthType(req.AuthType)

	if req.Type == "" || req.AuthType == "" {
		for _, tmpl := range db.GetProviderTemplates() {
			if tmpl.Service == req.Service {
				if req.Type == "" {
					providerType = tmpl.Type
				}
				if req.AuthType == "" {
					authType = tmpl.AuthType
				}
				break
			}
		}
	}

	if providerType == "" {
		http.Error(w, "type is required (could not infer from service)", http.StatusBadRequest)
		return
	}

	provider := &db.Provider{
		Name:       req.Name,
		Type:       providerType,
		Service:    req.Service,
		Enabled:    true,
		AuthType:   authType,
		AuthConfig: req.AuthConfig,
		Config:     req.Config,
	}

	id, err := api.db.CreateProvider(provider)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, "Provider with this name already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	provider.ID = id
	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(providerToResponse(provider, true))
}

// handleProviderTemplates returns available provider templates
func (api *ProvidersAPI) handleProviderTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates := db.GetProviderTemplates()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

// handleProviderAction handles /providers/{id}/* actions
func (api *ProvidersAPI) handleProviderAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/providers/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Provider ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid provider ID", http.StatusBadRequest)
		return
	}

	// /providers/{id}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			api.getProvider(w, r, id)
		case http.MethodPut, http.MethodPatch:
			api.updateProvider(w, r, id)
		case http.MethodDelete:
			api.deleteProvider(w, r, id)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /providers/{id}/{action}
	action := parts[1]
	switch action {
	case "enable":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		api.enableProvider(w, r, id)
	case "disable":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		api.disableProvider(w, r, id)
	case "set-default":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		api.setDefaultProvider(w, r, id)
	case "test":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		api.testProvider(w, r, id)
	default:
		http.Error(w, "Unknown action", http.StatusNotFound)
	}
}

func (api *ProvidersAPI) getProvider(w http.ResponseWriter, r *http.Request, id int64) {
	provider, err := api.db.GetProvider(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if provider == nil {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(provider, true))
}

func (api *ProvidersAPI) updateProvider(w http.ResponseWriter, r *http.Request, id int64) {
	provider, err := api.db.GetProvider(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if provider == nil {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		provider.Name = *req.Name
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.IsDefault != nil {
		provider.IsDefault = *req.IsDefault
	}
	if req.AuthConfig != nil {
		provider.AuthConfig = *req.AuthConfig
	}
	if req.Config != nil {
		provider.Config = *req.Config
	}

	if err := api.db.UpdateProvider(provider); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If setting as default, update other providers
	if req.IsDefault != nil && *req.IsDefault {
		if err := api.db.SetDefaultProvider(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(provider, true))
}

func (api *ProvidersAPI) deleteProvider(w http.ResponseWriter, r *http.Request, id int64) {
	if err := api.db.DeleteProvider(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *ProvidersAPI) enableProvider(w http.ResponseWriter, r *http.Request, id int64) {
	if err := api.db.EnableProvider(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	provider, _ := api.db.GetProvider(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(provider, true))
}

func (api *ProvidersAPI) disableProvider(w http.ResponseWriter, r *http.Request, id int64) {
	if err := api.db.DisableProvider(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	provider, _ := api.db.GetProvider(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

func (api *ProvidersAPI) setDefaultProvider(w http.ResponseWriter, r *http.Request, id int64) {
	if err := api.db.SetDefaultProvider(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	provider, _ := api.db.GetProvider(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerToResponse(provider, true))
}

// GetDefaultEmbeddingProvider is a helper to get the default embedding provider
func (api *ProvidersAPI) GetDefaultEmbeddingProvider() (*db.Provider, error) {
	return api.db.GetDefaultProvider(db.ProviderTypeEmbedding)
}

// ProviderTestResult contains the result of testing a provider
type ProviderTestResult struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	ResponseTime float64 `json:"response_time_ms"`
	Details      any     `json:"details,omitempty"`
}

func (api *ProvidersAPI) testProvider(w http.ResponseWriter, r *http.Request, id int64) {
	provider, err := api.db.GetProvider(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if provider == nil {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	var result ProviderTestResult

	switch provider.Type {
	case db.ProviderTypeEmbedding:
		result = api.testEmbeddingProvider(ctx, provider)
	case db.ProviderTypeLLM:
		result = api.testLLMProvider(ctx, provider)
	default:
		result = ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Testing not implemented for provider type: %s", provider.Type),
		}
	}

	result.ResponseTime = float64(time.Since(start).Milliseconds())

	w.Header().Set("Content-Type", "application/json")
	if !result.Success {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(result)
}

func (api *ProvidersAPI) testEmbeddingProvider(ctx context.Context, provider *db.Provider) ProviderTestResult {
	switch provider.Service {
	case "vertex_ai":
		return api.testVertexAI(ctx, provider)
	case "openai":
		return api.testOpenAI(ctx, provider)
	case "ollama":
		return api.testOllama(ctx, provider)
	default:
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Unknown embedding service: %s", provider.Service),
		}
	}
}

func (api *ProvidersAPI) testVertexAI(ctx context.Context, provider *db.Provider) ProviderTestResult {
	// Vertex AI embedding testing is no longer available — embeddings are now
	// handled by the Emergent backend. This test endpoint is retained as a
	// placeholder until Emergent SDK integration provides its own health check.
	return ProviderTestResult{
		Success: false,
		Message: "Vertex AI embedding testing is deprecated; embeddings are now handled by Emergent",
	}
}

func (api *ProvidersAPI) testOpenAI(ctx context.Context, provider *db.Provider) ProviderTestResult {
	apiKey := provider.GetAuthString("api_key")
	if apiKey == "" {
		return ProviderTestResult{
			Success: false,
			Message: "Missing API key",
		}
	}

	model := provider.GetConfigString("model")
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Make a test embedding request to OpenAI
	reqBody := map[string]any{
		"model": model,
		"input": "Hello, this is a test message.",
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", strings.NewReader(string(reqBytes)))
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("API error (status %d)", resp.StatusCode),
			Details: errResp,
		}
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	var embeddingLen int
	if data, ok := result["data"].([]any); ok && len(data) > 0 {
		if first, ok := data[0].(map[string]any); ok {
			if emb, ok := first["embedding"].([]any); ok {
				embeddingLen = len(emb)
			}
		}
	}

	// Extract usage from OpenAI response
	var tokensUsed int
	if usage, ok := result["usage"].(map[string]any); ok {
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			tokensUsed = int(promptTokens)
		}
		// Record usage
		if tokensUsed > 0 {
			if err := api.RecordUsage(provider.ID, provider.Service, model, tokensUsed, 0, 0); err != nil {
				slog.Warn("Failed to record OpenAI embedding usage", "error", err, "provider_id", provider.ID)
			}
		}
	}

	return ProviderTestResult{
		Success: true,
		Message: "Successfully generated embedding",
		Details: map[string]any{
			"model":            model,
			"embedding_length": embeddingLen,
			"tokens_used":      tokensUsed,
		},
	}
}

func (api *ProvidersAPI) testOllama(ctx context.Context, provider *db.Provider) ProviderTestResult {
	baseURL := provider.GetConfigString("base_url")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := provider.GetConfigString("model")
	if model == "" {
		model = "nomic-embed-text"
	}

	// Make a test embedding request to Ollama
	reqBody := map[string]any{
		"model":  model,
		"prompt": "Hello, this is a test message.",
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/embeddings", strings.NewReader(string(reqBytes)))
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Request failed (is Ollama running at %s?): %v", baseURL, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("API error (status %d)", resp.StatusCode),
			Details: errResp,
		}
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	var embeddingLen int
	if emb, ok := result["embedding"].([]any); ok {
		embeddingLen = len(emb)
	}

	return ProviderTestResult{
		Success: true,
		Message: "Successfully generated embedding",
		Details: map[string]any{
			"model":            model,
			"base_url":         baseURL,
			"embedding_length": embeddingLen,
		},
	}
}

// =============================================================================
// LLM Provider Testing
// =============================================================================

func (api *ProvidersAPI) testLLMProvider(ctx context.Context, provider *db.Provider) ProviderTestResult {
	switch provider.Service {
	case "vertex_ai_llm":
		return api.testVertexAILLM(ctx, provider)
	default:
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Unknown LLM service: %s", provider.Service),
		}
	}
}

func (api *ProvidersAPI) testVertexAILLM(ctx context.Context, provider *db.Provider) ProviderTestResult {
	projectID := provider.GetConfigString("project_id")
	location := provider.GetConfigString("location")
	model := provider.GetConfigString("model")
	account := provider.GetAuthString("oauth_account")

	if projectID == "" {
		return ProviderTestResult{
			Success: false,
			Message: "Missing project_id configuration",
		}
	}

	if model == "" {
		model = "gemini-2.5-flash"
	}
	if location == "" {
		location = "us-central1"
	}
	if account == "" {
		account = "default"
	}

	// Get OAuth token source
	tokenSource, err := auth.GetTokenSource(ctx, account, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get OAuth token: %v", err),
		}
	}

	// Create HTTP client with OAuth
	httpClient := oauth2.NewClient(ctx, tokenSource)

	// Build the Vertex AI Gemini API URL
	apiURL := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		location, projectID, location, model,
	)

	// Simple test request
	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": "Say 'hello' and nothing else."},
				},
			},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 50,
			"temperature":     0.1,
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(reqBytes)))
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return ProviderTestResult{
			Success: false,
			Message: fmt.Sprintf("API error (status %d)", resp.StatusCode),
			Details: errResp,
		}
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// Extract response text
	var responseText string
	if candidates, ok := result["candidates"].([]any); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]any); ok {
			if content, ok := candidate["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok && len(parts) > 0 {
					if part, ok := parts[0].(map[string]any); ok {
						if text, ok := part["text"].(string); ok {
							responseText = text
						}
					}
				}
			}
		}
	}

	// Extract usage metadata
	var inputTokens, outputTokens, cachedTokens int
	if usage, ok := result["usageMetadata"].(map[string]any); ok {
		if v, ok := usage["promptTokenCount"].(float64); ok {
			inputTokens = int(v)
		}
		if v, ok := usage["candidatesTokenCount"].(float64); ok {
			outputTokens = int(v)
		}
		if v, ok := usage["cachedContentTokenCount"].(float64); ok {
			cachedTokens = int(v)
		}
	}

	// Record usage (only if we have token counts)
	var cost float64
	if inputTokens > 0 || outputTokens > 0 {
		if err := api.RecordUsage(provider.ID, provider.Service, model, inputTokens, outputTokens, cachedTokens); err != nil {
			slog.Warn("Failed to record usage", "error", err, "provider_id", provider.ID)
		} else {
			// Get the cost for display
			if api.registry != nil {
				providerIDStr := models.GetProviderIDForService(provider.Service)
				if c, err := api.registry.CalculateCost(providerIDStr, model, inputTokens, outputTokens); err == nil {
					cost = c
				}
			}
		}
	}

	return ProviderTestResult{
		Success: true,
		Message: "Successfully generated response",
		Details: map[string]any{
			"model":         model,
			"location":      location,
			"response":      strings.TrimSpace(responseText),
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"cached_tokens": cachedTokens,
			"cost":          cost,
		},
	}
}

// =============================================================================
// Model Discovery
// =============================================================================

// ModelCost represents pricing for a model (per 1M tokens)
type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

// ModelLimits represents context and output limits
type ModelLimits struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// ModelInfo represents an available model
type ModelInfo struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	DisplayName string       `json:"display_name"`
	LaunchStage string       `json:"launch_stage,omitempty"`
	Family      string       `json:"family,omitempty"`
	Cost        *ModelCost   `json:"cost,omitempty"`
	Limits      *ModelLimits `json:"limits,omitempty"`
	ToolCall    bool         `json:"tool_call,omitempty"`
	Reasoning   bool         `json:"reasoning,omitempty"`
}

// ListModelsRequest is the request format for listing models
type ListModelsRequest struct {
	Service   string `json:"service"`
	Type      string `json:"type"` // "llm" or "embedding"
	ProjectID string `json:"project_id,omitempty"`
}

// ListModelsResponse is the response format for listing models
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// handleListModels returns available models for a service
func (api *ProvidersAPI) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ListModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var models []ModelInfo
	var err error

	switch req.Service {
	case "vertex_ai_llm":
		models, err = api.listVertexAILLMModels(ctx, req.ProjectID)
	case "vertex_ai":
		// For embedding models, return static list (no discovery API available)
		models = []ModelInfo{
			{ID: "text-embedding-005", Name: "text-embedding-005", DisplayName: "Text Embedding 005"},
			{ID: "text-embedding-004", Name: "text-embedding-004", DisplayName: "Text Embedding 004"},
			{ID: "text-multilingual-embedding-002", Name: "text-multilingual-embedding-002", DisplayName: "Multilingual Embedding 002"},
		}
	default:
		http.Error(w, "Unsupported service for model discovery", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListModelsResponse{Models: models})
}

// listVertexAILLMModels fetches available Gemini models from Vertex AI
func (api *ProvidersAPI) listVertexAILLMModels(ctx context.Context, projectID string) ([]ModelInfo, error) {
	// Get OAuth token
	tokenSource, err := auth.GetTokenSource(ctx, "default", "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth token: %w", err)
	}

	httpClient := oauth2.NewClient(ctx, tokenSource)

	// Query the publisher models API
	apiURL := "https://aiplatform.googleapis.com/v1beta1/publishers/google/models?pageSize=100"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add quota project header if provided
	if projectID != "" {
		req.Header.Set("x-goog-user-project", projectID)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error (status %d): %v", resp.StatusCode, errResp)
	}

	var result struct {
		PublisherModels []struct {
			Name        string `json:"name"`
			LaunchStage string `json:"launchStage"`
		} `json:"publisherModels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var models []ModelInfo
	for _, m := range result.PublisherModels {
		// Extract model ID from name (e.g., "publishers/google/models/gemini-2.0-flash-001" -> "gemini-2.0-flash-001")
		parts := strings.Split(m.Name, "/")
		if len(parts) < 4 {
			continue
		}
		modelID := parts[len(parts)-1]

		// Only include Gemini models
		if !strings.HasPrefix(modelID, "gemini") {
			continue
		}

		// Create display name
		displayName := strings.ReplaceAll(modelID, "-", " ")
		displayName = strings.Title(displayName)

		models = append(models, ModelInfo{
			ID:          modelID,
			Name:        modelID,
			DisplayName: displayName,
			LaunchStage: m.LaunchStage,
		})
	}

	return models, nil
}

// =============================================================================
// Google OAuth Authentication
// =============================================================================

// GoogleAuthStatus represents the current Google authentication status
type GoogleAuthStatus struct {
	Authenticated  bool   `json:"authenticated"`
	Account        string `json:"account"`
	HasToken       bool   `json:"has_token"`
	HasCredentials bool   `json:"has_credentials"`
	HasADC         bool   `json:"has_adc"`
	UsingADC       bool   `json:"using_adc"`
	TokenPath      string `json:"token_path,omitempty"`
}

// GoogleDeviceCodeResponse is the response from starting the device flow
type GoogleDeviceCodeResponse struct {
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	DeviceCode      string `json:"device_code"`
}

// GoogleAuthStartRequest is the request to start Google OAuth
type GoogleAuthStartRequest struct {
	Account string   `json:"account,omitempty"`
	Scopes  []string `json:"scopes,omitempty"`
}

// GoogleAuthPollRequest is the request to poll for a token
type GoogleAuthPollRequest struct {
	Account    string `json:"account,omitempty"`
	DeviceCode string `json:"device_code"`
	Interval   int    `json:"interval"`
}

// handleGoogleAuth handles GET /google/auth (status) and DELETE /google/auth (logout)
func (api *ProvidersAPI) handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.getGoogleAuthStatus(w, r)
	case http.MethodDelete:
		api.deleteGoogleAuth(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getGoogleAuthStatus returns the current Google authentication status
func (api *ProvidersAPI) getGoogleAuthStatus(w http.ResponseWriter, r *http.Request) {
	account := r.URL.Query().Get("account")
	if account == "" {
		account = "default"
	}

	status, err := auth.GetAuthStatus(account)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleAuthStatus{
		Authenticated:  status.Authenticated,
		Account:        status.Account,
		HasToken:       status.HasToken,
		HasCredentials: status.HasCredentials,
		HasADC:         status.HasADC,
		UsingADC:       status.UsingADC,
		TokenPath:      status.TokenPath,
	})
}

// deleteGoogleAuth removes the Google OAuth token
func (api *ProvidersAPI) deleteGoogleAuth(w http.ResponseWriter, r *http.Request) {
	account := r.URL.Query().Get("account")
	if account == "" {
		account = "default"
	}

	if err := auth.DeleteToken(account); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Token deleted successfully",
	})
}

// handleGoogleAuthStart initiates the Google OAuth device flow
func (api *ProvidersAPI) handleGoogleAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleAuthStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body - use defaults
		req = GoogleAuthStartRequest{}
	}

	// Default scopes for Google Cloud Platform (covers Vertex AI, Drive, Gmail, Calendar, etc.)
	if len(req.Scopes) == 0 {
		req.Scopes = []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/gmail.modify",
			"https://www.googleapis.com/auth/calendar",
			"https://www.googleapis.com/auth/drive.readonly",
		}
	}

	dcr, err := auth.StartDeviceFlow(req.Scopes...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start device flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GoogleDeviceCodeResponse{
		UserCode:        dcr.UserCode,
		VerificationURL: dcr.VerificationURL,
		ExpiresIn:       dcr.ExpiresIn,
		Interval:        dcr.Interval,
		DeviceCode:      dcr.DeviceCode,
	})
}

// handleGoogleAuthPoll polls for a token after user authorization
func (api *ProvidersAPI) handleGoogleAuthPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleAuthPollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DeviceCode == "" {
		http.Error(w, "device_code is required", http.StatusBadRequest)
		return
	}

	account := req.Account
	if account == "" {
		account = "default"
	}

	token, err := auth.PollForToken(account, req.DeviceCode, req.Interval)
	if err != nil {
		errStr := err.Error()
		// Return specific error codes for known OAuth states
		if errStr == "authorization_pending" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted) // 202 - still waiting
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "pending",
				"message": "Waiting for user to authorize",
			})
			return
		}
		if errStr == "slow_down" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests) // 429 - slow down
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "slow_down",
				"message": "Polling too frequently, please slow down",
			})
			return
		}
		if errStr == "expired_token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGone) // 410 - expired
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "expired",
				"message": "Device code has expired, please start over",
			})
			return
		}
		if errStr == "access_denied" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden) // 403 - denied
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "denied",
				"message": "User denied the authorization request",
			})
			return
		}

		http.Error(w, fmt.Sprintf("Failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "success",
		"message": "Successfully authenticated with Google",
		"account": account,
		"expires": token.Expiry,
	})
}

// =============================================================================
// Models Registry (from models.dev)
// =============================================================================

// ModelsRegistryResponse represents the response from the models registry
type ModelsRegistryResponse struct {
	ProviderID   string      `json:"provider_id"`
	ProviderName string      `json:"provider_name"`
	Models       []ModelInfo `json:"models"`
	CachedAt     *time.Time  `json:"cached_at,omitempty"`
}

// handleModelsRegistry returns available models from models.dev
func (api *ProvidersAPI) handleModelsRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure registry is loaded
	if api.registry != nil {
		if err := api.registry.Load(); err != nil {
			// Log but continue - may have cached data
			_ = err
		}
	}

	providerID := r.URL.Query().Get("provider")
	service := r.URL.Query().Get("service")

	// Map service to provider ID if needed
	if providerID == "" && service != "" {
		providerID = models.GetProviderIDForService(service)
	}

	if providerID == "" {
		// List all provider IDs
		if api.registry == nil {
			http.Error(w, "Models registry not initialized", http.StatusServiceUnavailable)
			return
		}
		providers := api.registry.ListProviders()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"providers": providers,
		})
		return
	}

	// Get models for specific provider
	if api.registry == nil {
		http.Error(w, "Models registry not initialized", http.StatusServiceUnavailable)
		return
	}

	provider, ok := api.registry.GetProvider(providerID)
	if !ok {
		http.Error(w, fmt.Sprintf("Provider not found: %s", providerID), http.StatusNotFound)
		return
	}

	modelList := api.registry.ListModels(providerID)
	modelInfos := make([]ModelInfo, 0, len(modelList))
	for _, m := range modelList {
		info := ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			DisplayName: m.Name,
			LaunchStage: m.LaunchStage,
			Family:      m.Family,
			ToolCall:    m.ToolCall,
			Reasoning:   m.Reasoning,
		}
		if m.Cost.Input > 0 || m.Cost.Output > 0 {
			info.Cost = &ModelCost{
				Input:      m.Cost.Input,
				Output:     m.Cost.Output,
				CacheRead:  m.Cost.CacheRead,
				CacheWrite: m.Cost.CacheWrite,
			}
		}
		if m.Limit.Context > 0 || m.Limit.Output > 0 {
			info.Limits = &ModelLimits{
				Context: m.Limit.Context,
				Output:  m.Limit.Output,
			}
		}
		modelInfos = append(modelInfos, info)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModelsRegistryResponse{
		ProviderID:   providerID,
		ProviderName: provider.Name,
		Models:       modelInfos,
	})
}

// handleModelDetails returns details for a specific model
func (api *ProvidersAPI) handleModelDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /models/{provider}/{model}
	path := strings.TrimPrefix(r.URL.Path, "/models/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "Usage: /models/{provider}/{model}", http.StatusBadRequest)
		return
	}

	providerID := parts[0]
	modelID := parts[1]

	if api.registry == nil {
		http.Error(w, "Models registry not initialized", http.StatusServiceUnavailable)
		return
	}

	// Ensure registry is loaded
	if err := api.registry.Load(); err != nil {
		_ = err // Log but continue
	}

	model, ok := api.registry.GetModel(providerID, modelID)
	if !ok {
		http.Error(w, fmt.Sprintf("Model not found: %s/%s", providerID, modelID), http.StatusNotFound)
		return
	}

	info := ModelInfo{
		ID:          model.ID,
		Name:        model.Name,
		DisplayName: model.Name,
		LaunchStage: model.LaunchStage,
		Family:      model.Family,
		ToolCall:    model.ToolCall,
		Reasoning:   model.Reasoning,
	}
	if model.Cost.Input > 0 || model.Cost.Output > 0 {
		info.Cost = &ModelCost{
			Input:      model.Cost.Input,
			Output:     model.Cost.Output,
			CacheRead:  model.Cost.CacheRead,
			CacheWrite: model.Cost.CacheWrite,
		}
	}
	if model.Limit.Context > 0 || model.Limit.Output > 0 {
		info.Limits = &ModelLimits{
			Context: model.Limit.Context,
			Output:  model.Limit.Output,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// =============================================================================
// Usage Tracking
// =============================================================================

// UsageRecord represents a usage record in API responses
type UsageRecord struct {
	ID           int64     `json:"id"`
	ProviderID   int64     `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Service      string    `json:"service"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CachedTokens int       `json:"cached_tokens"`
	Cost         float64   `json:"cost"`
	CreatedAt    time.Time `json:"created_at"`
}

// UsageSummaryRecord represents aggregated usage in API responses
type UsageSummaryRecord struct {
	ProviderID    int64   `json:"provider_id"`
	ProviderName  string  `json:"provider_name"`
	Service       string  `json:"service"`
	Model         string  `json:"model"`
	TotalRequests int     `json:"total_requests"`
	TotalInput    int     `json:"total_input"`
	TotalOutput   int     `json:"total_output"`
	TotalCached   int     `json:"total_cached"`
	TotalCost     float64 `json:"total_cost"`
}

// UsageResponse represents the response for usage queries
type UsageResponse struct {
	Records   []UsageRecord `json:"records"`
	TotalCost float64       `json:"total_cost"`
	From      time.Time     `json:"from"`
	To        time.Time     `json:"to"`
}

// UsageSummaryResponse represents the response for usage summary
type UsageSummaryResponse struct {
	Summary   []UsageSummaryRecord `json:"summary"`
	TotalCost float64              `json:"total_cost"`
	From      time.Time            `json:"from"`
	To        time.Time            `json:"to"`
}

// handleUsage handles GET /usage
func (api *ProvidersAPI) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	from := parseTimeParam(r.URL.Query().Get("from"), time.Now().AddDate(0, -1, 0)) // Default: 1 month ago
	to := parseTimeParam(r.URL.Query().Get("to"), time.Now())
	limit := parseIntParam(r.URL.Query().Get("limit"), 100)
	service := r.URL.Query().Get("service")
	providerIDStr := r.URL.Query().Get("provider_id")

	var usages []*db.Usage
	var err error

	if providerIDStr != "" {
		providerID, _ := strconv.ParseInt(providerIDStr, 10, 64)
		usages, err = api.db.GetUsageByProvider(providerID, from, to, limit)
	} else if service != "" {
		usages, err = api.db.GetUsageByService(service, from, to, limit)
	} else {
		usages, err = api.db.GetAllUsage(from, to, limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	records := make([]UsageRecord, len(usages))
	var totalCost float64
	for i, u := range usages {
		records[i] = UsageRecord{
			ID:           u.ID,
			ProviderID:   u.ProviderID,
			ProviderName: u.ProviderName,
			Service:      u.Service,
			Model:        u.Model,
			InputTokens:  u.InputTokens,
			OutputTokens: u.OutputTokens,
			CachedTokens: u.CachedTokens,
			Cost:         u.Cost,
			CreatedAt:    u.CreatedAt,
		}
		totalCost += u.Cost
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UsageResponse{
		Records:   records,
		TotalCost: totalCost,
		From:      from,
		To:        to,
	})
}

// handleUsageSummary handles GET /usage/summary
func (api *ProvidersAPI) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	from := parseTimeParam(r.URL.Query().Get("from"), time.Now().AddDate(0, -1, 0))
	to := parseTimeParam(r.URL.Query().Get("to"), time.Now())

	summaries, err := api.db.GetUsageSummary(from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	records := make([]UsageSummaryRecord, len(summaries))
	var totalCost float64
	for i, s := range summaries {
		records[i] = UsageSummaryRecord{
			ProviderID:    s.ProviderID,
			ProviderName:  s.ProviderName,
			Service:       s.Service,
			Model:         s.Model,
			TotalRequests: s.TotalRequests,
			TotalInput:    s.TotalInput,
			TotalOutput:   s.TotalOutput,
			TotalCached:   s.TotalCached,
			TotalCost:     s.TotalCost,
		}
		totalCost += s.TotalCost
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UsageSummaryResponse{
		Summary:   records,
		TotalCost: totalCost,
		From:      from,
		To:        to,
	})
}

// parseTimeParam parses a time string or returns the default
func parseTimeParam(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return def
}

// parseIntParam parses an int string or returns the default
func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

// RecordUsage records a usage entry (can be called by LLM providers)
func (api *ProvidersAPI) RecordUsage(providerID int64, service, model string, inputTokens, outputTokens, cachedTokens int) error {
	// Calculate cost from registry
	var cost float64
	if api.registry != nil {
		providerIDStr := models.GetProviderIDForService(service)
		if c, err := api.registry.CalculateCost(providerIDStr, model, inputTokens, outputTokens); err == nil {
			cost = c
		}
	}

	usage := &db.Usage{
		ProviderID:   providerID,
		Service:      service,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
		Cost:         cost,
	}

	_, err := api.db.RecordUsage(usage)
	return err
}
