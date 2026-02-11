package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/diane-assistant/diane/internal/db"
)

// ToolProvider provides access to running MCP server tools
type ToolProvider interface {
	GetAllTools() []ToolInfo
}

// ContextsAPI handles context-related API endpoints
type ContextsAPI struct {
	db           *db.DB
	toolProvider ToolProvider
}

// NewContextsAPI creates a new ContextsAPI
func NewContextsAPI(database *db.DB) *ContextsAPI {
	return &ContextsAPI{db: database}
}

// SetToolProvider sets the tool provider for syncing tools from running servers
func (api *ContextsAPI) SetToolProvider(provider ToolProvider) {
	api.toolProvider = provider
}

// ContextResponse represents a context in API responses
type ContextResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// ContextDetailResponse includes servers and tools
type ContextDetailResponse struct {
	Context ContextResponse         `json:"context"`
	Servers []ContextServerResponse `json:"servers"`
	Summary ContextSummary          `json:"summary"`
}

// ContextSummary provides aggregate counts
type ContextSummary struct {
	ServersEnabled int `json:"servers_enabled"`
	ServersTotal   int `json:"servers_total"`
	ToolsActive    int `json:"tools_active"`
	ToolsTotal     int `json:"tools_total"`
}

// RegisterRoutes registers context API routes on the given mux
func (api *ContextsAPI) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/contexts", api.handleContexts)
	mux.HandleFunc("/contexts/", api.handleContextAction)
}

// handleContexts handles GET /contexts and POST /contexts
func (api *ContextsAPI) handleContexts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		api.listContexts(w, r)
	case http.MethodPost:
		api.createContext(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// listContexts returns all contexts
func (api *ContextsAPI) listContexts(w http.ResponseWriter, r *http.Request) {
	contexts, err := api.db.ListContexts()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	response := make([]ContextResponse, len(contexts))
	for i, c := range contexts {
		response[i] = ContextResponse{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			IsDefault:   c.IsDefault,
		}
	}
	json.NewEncoder(w).Encode(response)
}

// createContext creates a new context
func (api *ContextsAPI) createContext(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Context name is required"})
		return
	}

	ctx := &db.Context{
		Name:        body.Name,
		Description: body.Description,
	}
	if err := api.db.CreateContext(ctx); err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ContextResponse{
		ID:          ctx.ID,
		Name:        ctx.Name,
		Description: ctx.Description,
		IsDefault:   ctx.IsDefault,
	})
}

// handleContextAction routes /contexts/{name}/... requests
func (api *ContextsAPI) handleContextAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/contexts/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Context name required"})
		return
	}

	contextName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		api.handleContextCRUD(w, r, contextName)
	case "default":
		api.handleSetDefault(w, r, contextName)
	case "connect":
		api.handleConnect(w, r, contextName)
	case "sync":
		api.handleSyncTools(w, r, contextName)
	case "available-servers":
		api.handleAvailableServers(w, r, contextName)
	case "servers":
		api.routeServersAction(w, r, contextName, parts[2:])
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action: " + action})
	}
}

// handleContextCRUD handles GET/PUT/DELETE /contexts/{name}
func (api *ContextsAPI) handleContextCRUD(w http.ResponseWriter, r *http.Request, contextName string) {
	switch r.Method {
	case http.MethodGet:
		api.getContextDetail(w, contextName)
	case http.MethodPut:
		api.updateContext(w, r, contextName)
	case http.MethodDelete:
		api.deleteContext(w, contextName)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// getContextDetail returns full context details
func (api *ContextsAPI) getContextDetail(w http.ResponseWriter, contextName string) {
	detail, err := api.db.GetContextDetail(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if detail == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Context not found"})
		return
	}

	response := api.buildContextDetailResponse(detail)
	json.NewEncoder(w).Encode(response)
}

// updateContext updates context metadata
func (api *ContextsAPI) updateContext(w http.ResponseWriter, r *http.Request, contextName string) {
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	ctx := &db.Context{
		Name:        contextName,
		Description: body.Description,
	}
	if err := api.db.UpdateContext(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// deleteContext deletes a context
func (api *ContextsAPI) deleteContext(w http.ResponseWriter, contextName string) {
	if err := api.db.DeleteContext(contextName); err != nil {
		if err == db.ErrCannotDeleteDefault {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// handleSetDefault handles POST /contexts/{name}/default
func (api *ContextsAPI) handleSetDefault(w http.ResponseWriter, r *http.Request, contextName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	if err := api.db.SetDefaultContext(contextName); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "default": contextName})
}

// ConnectInfo provides connection instructions for a context
type ConnectInfo struct {
	Context     string            `json:"context"`
	SSE         ConnectionDetails `json:"sse"`
	Streamable  ConnectionDetails `json:"streamable"`
	Description string            `json:"description,omitempty"`
}

// ConnectionDetails provides specific connection info
type ConnectionDetails struct {
	URL     string `json:"url"`
	Example string `json:"example,omitempty"`
}

// handleConnect handles GET /contexts/{name}/connect
func (api *ContextsAPI) handleConnect(w http.ResponseWriter, r *http.Request, contextName string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	// Check if context exists
	ctx, err := api.db.GetContext(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if ctx == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Context not found"})
		return
	}

	// Base URL for Diane MCP server
	baseURL := "http://localhost:8765"

	info := ConnectInfo{
		Context: contextName,
		SSE: ConnectionDetails{
			URL:     baseURL + "/sse?context=" + contextName,
			Example: `curl -N "` + baseURL + `/sse?context=` + contextName + `"`,
		},
		Streamable: ConnectionDetails{
			URL:     baseURL + "/mcp?context=" + contextName,
			Example: `{"mcpServers": {"diane-` + contextName + `": {"url": "` + baseURL + `/mcp?context=` + contextName + `"}}}`,
		},
		Description: ctx.Description,
	}

	json.NewEncoder(w).Encode(info)
}

// handleSyncTools handles POST /contexts/{name}/sync
// Syncs tools from running MCP servers to the context
func (api *ContextsAPI) handleSyncTools(w http.ResponseWriter, r *http.Request, contextName string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	if api.toolProvider == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Tool provider not available"})
		return
	}

	// Check if context exists
	ctx, err := api.db.GetContext(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if ctx == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Context not found"})
		return
	}

	// Get all tools from running servers
	allTools := api.toolProvider.GetAllTools()

	// Group tools by server
	serverTools := make(map[string][]string)
	for _, tool := range allTools {
		serverTools[tool.Server] = append(serverTools[tool.Server], tool.Name)
	}

	// Get servers in this context
	contextServers, err := api.db.GetServersForContext(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Build a set of server names in the context
	contextServerNames := make(map[string]bool)
	for _, cs := range contextServers {
		contextServerNames[cs.ServerName] = true
	}

	// Sync tools for each server in the context
	syncedCount := 0
	for serverName, tools := range serverTools {
		if !contextServerNames[serverName] {
			continue
		}

		// Get existing tool settings
		existingTools, err := api.db.GetToolsForContextServer(contextName, serverName)
		if err != nil {
			continue
		}

		// Add any missing tools (default to enabled)
		toolUpdates := make(map[string]bool)
		for _, toolName := range tools {
			if _, exists := existingTools[toolName]; !exists {
				toolUpdates[toolName] = true // default enabled
				syncedCount++
			} else {
				// Keep existing setting
				toolUpdates[toolName] = existingTools[toolName]
			}
		}

		if len(toolUpdates) > 0 {
			if err := api.db.BulkSetToolsEnabled(contextName, serverName, toolUpdates); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "synced",
		"tools_synced": syncedCount,
	})
}

// AvailableServer represents a server that can be added to a context
type AvailableServer struct {
	Name      string `json:"name"`
	ToolCount int    `json:"tool_count"`
	InContext bool   `json:"in_context"`
	Builtin   bool   `json:"builtin,omitempty"`
}

// handleAvailableServers handles GET /contexts/{name}/available-servers
// Returns all running MCP servers, indicating which are already in the context
func (api *ContextsAPI) handleAvailableServers(w http.ResponseWriter, r *http.Request, contextName string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	if api.toolProvider == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Tool provider not available"})
		return
	}

	// Get all tools from running servers
	allTools := api.toolProvider.GetAllTools()

	// Group tools by server and count
	serverToolCount := make(map[string]int)
	serverBuiltin := make(map[string]bool)
	for _, tool := range allTools {
		serverToolCount[tool.Server]++
		if tool.Builtin {
			serverBuiltin[tool.Server] = true
		}
	}

	// Get servers already in this context
	contextServers, err := api.db.GetServersForContext(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	contextServerSet := make(map[string]bool)
	for _, cs := range contextServers {
		contextServerSet[cs.ServerName] = true
	}

	// Build response
	var available []AvailableServer
	for serverName, toolCount := range serverToolCount {
		available = append(available, AvailableServer{
			Name:      serverName,
			ToolCount: toolCount,
			InContext: contextServerSet[serverName],
			Builtin:   serverBuiltin[serverName],
		})
	}

	json.NewEncoder(w).Encode(available)
}

// buildContextDetailResponse builds response from ContextDetail
func (api *ContextsAPI) buildContextDetailResponse(detail *db.ContextDetail) ContextDetailResponse {
	response := ContextDetailResponse{
		Context: ContextResponse{
			ID:          detail.ID,
			Name:        detail.Name,
			Description: detail.Description,
			IsDefault:   detail.IsDefault,
		},
		Servers: make([]ContextServerResponse, len(detail.Servers)),
		Summary: ContextSummary{
			ServersTotal: len(detail.Servers),
		},
	}

	for i, s := range detail.Servers {
		enabledCount := 0
		for _, enabled := range s.Tools {
			if enabled {
				enabledCount++
			}
		}

		totalTools := len(s.Tools)

		serverResp := ContextServerResponse{
			ID:          s.ID,
			Name:        s.ServerName,
			Type:        s.Server.Type,
			Enabled:     s.Enabled,
			ToolsActive: enabledCount,
			ToolsTotal:  totalTools,
		}

		if len(s.Tools) > 0 {
			serverResp.Tools = make([]ToolStatusResponse, 0, len(s.Tools))
			for name, enabled := range s.Tools {
				serverResp.Tools = append(serverResp.Tools, ToolStatusResponse{
					Name:    name,
					Enabled: enabled,
				})
			}
		}

		response.Servers[i] = serverResp

		if s.Enabled {
			response.Summary.ServersEnabled++
		}
		response.Summary.ToolsTotal += totalTools
		response.Summary.ToolsActive += enabledCount
	}

	return response
}
