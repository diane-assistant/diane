package api

import (
	"encoding/json"
	"net/http"

	"github.com/diane-assistant/diane/internal/db"
)

// ContextServerResponse represents a server in a context
type ContextServerResponse struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Enabled     bool                 `json:"enabled"`
	ToolsActive int                  `json:"tools_active"`
	ToolsTotal  int                  `json:"tools_total"`
	Tools       []ToolStatusResponse `json:"tools,omitempty"`
}

// ToolStatusResponse represents a tool's enabled status
type ToolStatusResponse struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// routeServersAction routes /contexts/{name}/servers/... requests
func (api *ContextsAPI) routeServersAction(w http.ResponseWriter, r *http.Request, contextName string, pathParts []string) {
	if len(pathParts) == 0 {
		api.handleContextServers(w, r, contextName)
		return
	}

	serverName := pathParts[0]

	if len(pathParts) == 1 {
		api.handleServerInContext(w, r, contextName, serverName)
		return
	}

	if pathParts[1] == "tools" {
		if len(pathParts) == 2 {
			api.handleServerTools(w, r, contextName, serverName)
		} else {
			toolName := pathParts[2]
			api.handleToolAction(w, r, contextName, serverName, toolName)
		}
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": "Unknown path"})
}

// handleContextServers handles GET/POST /contexts/{name}/servers
func (api *ContextsAPI) handleContextServers(w http.ResponseWriter, r *http.Request, contextName string) {
	switch r.Method {
	case http.MethodGet:
		api.listContextServers(w, contextName)
	case http.MethodPost:
		api.addServerToContext(w, r, contextName)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// listContextServers returns all servers in a context
func (api *ContextsAPI) listContextServers(w http.ResponseWriter, contextName string) {
	servers, err := api.db.GetServersForContext(contextName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	response := make([]ContextServerResponse, len(servers))
	for i, s := range servers {
		server, _ := api.db.GetMCPServerByID(s.ServerID)
		serverType := ""
		if server != nil {
			serverType = server.Type
		}
		response[i] = ContextServerResponse{
			ID:      s.ID,
			Name:    s.ServerName,
			Type:    serverType,
			Enabled: s.Enabled,
		}
	}
	json.NewEncoder(w).Encode(response)
}

// addServerToContext adds a server to a context
func (api *ContextsAPI) addServerToContext(w http.ResponseWriter, r *http.Request, contextName string) {
	var body struct {
		ServerName string `json:"server_name"`
		Enabled    *bool  `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.ServerName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "server_name is required"})
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	// First, ensure the server exists in the database
	// If it doesn't exist, try to sync it from the running proxy
	server, err := api.db.GetMCPServer(body.ServerName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if server == nil {
		// Server doesn't exist in DB - it might be a builtin server
		// Create a minimal entry for it
		server = &db.MCPServer{
			Name:    body.ServerName,
			Enabled: true,
			Type:    "builtin",
		}
		if err := api.db.CreateMCPServer(server); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create server: " + err.Error()})
			return
		}
	}

	// Add server to context
	if err := api.db.AddServerToContext(contextName, body.ServerName, enabled); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// If we have a tool provider, sync the tools for this server
	if api.toolProvider != nil {
		allTools := api.toolProvider.GetAllTools()
		var serverTools []string
		for _, tool := range allTools {
			if tool.Server == body.ServerName {
				serverTools = append(serverTools, tool.Name)
			}
		}

		if len(serverTools) > 0 {
			toolUpdates := make(map[string]bool)
			for _, toolName := range serverTools {
				toolUpdates[toolName] = true // default enabled
			}
			// Ignore errors for tool sync - server is already added
			_ = api.db.BulkSetToolsEnabled(contextName, body.ServerName, toolUpdates)
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "added",
		"server":  body.ServerName,
		"enabled": enabled,
	})
}

// handleServerInContext handles PUT/DELETE /contexts/{name}/servers/{server}
func (api *ContextsAPI) handleServerInContext(w http.ResponseWriter, r *http.Request, contextName, serverName string) {
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := api.db.SetServerEnabledInContext(contextName, serverName, body.Enabled); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"server":  serverName,
			"enabled": body.Enabled,
		})

	case http.MethodDelete:
		if err := api.db.RemoveServerFromContext(contextName, serverName); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "removed", "server": serverName})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// handleServerTools handles GET/PUT /contexts/{name}/servers/{server}/tools
func (api *ContextsAPI) handleServerTools(w http.ResponseWriter, r *http.Request, contextName, serverName string) {
	switch r.Method {
	case http.MethodGet:
		tools, err := api.db.GetToolsForContextServer(contextName, serverName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		response := make([]ToolStatusResponse, 0, len(tools))
		for name, enabled := range tools {
			response = append(response, ToolStatusResponse{
				Name:    name,
				Enabled: enabled,
			})
		}
		json.NewEncoder(w).Encode(response)

	case http.MethodPut:
		var body map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if err := api.db.BulkSetToolsEnabled(contextName, serverName, body); err != nil {
			if err == db.ErrServerNotInContext {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// handleToolAction handles PUT /contexts/{name}/servers/{server}/tools/{tool}
func (api *ContextsAPI) handleToolAction(w http.ResponseWriter, r *http.Request, contextName, serverName, toolName string) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if err := api.db.SetToolEnabled(contextName, serverName, toolName, body.Enabled); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"tool":    toolName,
		"enabled": body.Enabled,
	})
}
