package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/store"
)

// MCPServersAPI handles MCP server configuration endpoints
type MCPServersAPI struct {
	db             store.MCPServerStore
	statusProvider StatusProvider
}

// NewMCPServersAPI creates a new MCPServersAPI
func NewMCPServersAPI(mcpStore store.MCPServerStore, statusProvider StatusProvider) *MCPServersAPI {
	return &MCPServersAPI{db: mcpStore, statusProvider: statusProvider}
}

// MCPServerResponse represents an MCP server in API responses
type MCPServerResponse struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Type      string            `json:"type"` // stdio, sse, http, builtin
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	OAuth     *db.OAuthConfig   `json:"oauth,omitempty"`
	NodeID    string            `json:"node_id,omitempty"`
	NodeMode  string            `json:"node_mode,omitempty"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// RegisterRoutes registers MCP server API routes on the given mux
func (api *MCPServersAPI) RegisterRoutes(mux *http.ServeMux) {
	// MCP Registry routes (server definitions)
	mux.HandleFunc("/mcp-servers-config", api.handleMCPServers)
	mux.HandleFunc("/mcp-servers-config/", api.handleMCPServerAction)

	// Placement routes (deployment config)
	mux.HandleFunc("/mcp-placements", api.handlePlacements)
	mux.HandleFunc("/mcp-placements/", api.handlePlacementAction)

	// Backward compatibility aliases
	mux.HandleFunc("/mcp-registry", api.handleMCPServers)
	mux.HandleFunc("/mcp-registry/", api.handleMCPServerAction)
}

// handleMCPServers handles GET /mcp-servers-config and POST /mcp-servers-config
func (api *MCPServersAPI) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		api.listServers(w, r)
	case http.MethodPost:
		api.createServer(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// listServers returns all MCP servers (builtin + configured)
func (api *MCPServersAPI) listServers(w http.ResponseWriter, r *http.Request) {
	var response []MCPServerResponse

	// Add builtin servers from status provider (with negative IDs to distinguish)
	if api.statusProvider != nil {
		mcpServers := api.statusProvider.GetMCPServers()
		now := time.Now().Format("2006-01-02T15:04:05Z07:00")
		builtinID := int64(-1)
		for _, s := range mcpServers {
			if s.Builtin {
				response = append(response, MCPServerResponse{
					ID:        builtinID,
					Name:      s.Name,
					Enabled:   s.Enabled,
					Type:      "builtin",
					CreatedAt: now,
					UpdatedAt: now,
				})
				builtinID--
			}
		}
	}

	// Add configured servers from database
	servers, err := api.db.ListMCPServers(context.Background())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	for _, s := range servers {
		response = append(response, MCPServerResponse{
			ID:        s.ID,
			Name:      s.Name,
			Enabled:   s.Enabled,
			Type:      s.Type,
			Command:   s.Command,
			Args:      s.Args,
			Env:       s.Env,
			URL:       s.URL,
			Headers:   s.Headers,
			OAuth:     s.OAuth,
			NodeID:    s.NodeID,
			NodeMode:  s.NodeMode,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	json.NewEncoder(w).Encode(response)
}

// createServer creates a new MCP server
func (api *MCPServersAPI) createServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string            `json:"name"`
		Enabled  *bool             `json:"enabled,omitempty"`
		Type     string            `json:"type"`
		Command  string            `json:"command,omitempty"`
		Args     []string          `json:"args,omitempty"`
		Env      map[string]string `json:"env,omitempty"`
		URL      string            `json:"url,omitempty"`
		Headers  map[string]string `json:"headers,omitempty"`
		OAuth    *db.OAuthConfig   `json:"oauth,omitempty"`
		NodeID   string            `json:"node_id,omitempty"`
		NodeMode string            `json:"node_mode,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server name is required"})
		return
	}

	if body.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server type is required"})
		return
	}

	// Validate type
	if body.Type != "stdio" && body.Type != "sse" && body.Type != "http" && body.Type != "builtin" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid server type. Must be: stdio, sse, http, or builtin"})
		return
	}

	// Validate required fields based on type
	if body.Type == "stdio" && body.Command == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Command is required for stdio servers"})
		return
	}

	if (body.Type == "sse" || body.Type == "http") && body.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL is required for sse/http servers"})
		return
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	// Set default node_mode if not provided
	nodeMode := body.NodeMode
	if nodeMode == "" {
		nodeMode = "master"
	}

	// Validate node_mode
	if nodeMode != "master" && nodeMode != "specific" && nodeMode != "any" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid node_mode. Must be: master, specific, or any"})
		return
	}

	// Validate node_id is provided when node_mode is "specific"
	if nodeMode == "specific" && body.NodeID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "node_id is required when node_mode is 'specific'"})
		return
	}

	server := &db.MCPServer{
		Name:     body.Name,
		Enabled:  enabled,
		Type:     body.Type,
		Command:  body.Command,
		Args:     body.Args,
		Env:      body.Env,
		URL:      body.URL,
		Headers:  body.Headers,
		OAuth:    body.OAuth,
		NodeID:   body.NodeID,
		NodeMode: nodeMode,
	}

	if err := api.db.CreateMCPServer(context.Background(), server); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Server with this name already exists"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(MCPServerResponse{
		ID:        server.ID,
		Name:      server.Name,
		Enabled:   server.Enabled,
		Type:      server.Type,
		Command:   server.Command,
		Args:      server.Args,
		Env:       server.Env,
		URL:       server.URL,
		Headers:   server.Headers,
		OAuth:     server.OAuth,
		NodeID:    server.NodeID,
		NodeMode:  server.NodeMode,
		CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// handleMCPServerAction routes /mcp-servers-config/{id}/... requests
func (api *MCPServersAPI) handleMCPServerAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/mcp-servers-config/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server ID required"})
		return
	}

	serverID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid server ID"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.getServer(w, serverID)
	case http.MethodPut:
		api.updateServer(w, r, serverID)
	case http.MethodDelete:
		api.deleteServer(w, serverID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// getServer returns a single server by ID
func (api *MCPServersAPI) getServer(w http.ResponseWriter, id int64) {
	server, err := api.db.GetMCPServerByID(context.Background(), id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if server == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server not found"})
		return
	}

	json.NewEncoder(w).Encode(MCPServerResponse{
		ID:        server.ID,
		Name:      server.Name,
		Enabled:   server.Enabled,
		Type:      server.Type,
		Command:   server.Command,
		Args:      server.Args,
		Env:       server.Env,
		URL:       server.URL,
		Headers:   server.Headers,
		OAuth:     server.OAuth,
		NodeID:    server.NodeID,
		NodeMode:  server.NodeMode,
		CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// updateServer updates an existing server
func (api *MCPServersAPI) updateServer(w http.ResponseWriter, r *http.Request, id int64) {
	// First, fetch the existing server
	server, err := api.db.GetMCPServerByID(context.Background(), id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if server == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server not found"})
		return
	}

	var body struct {
		Name     *string            `json:"name,omitempty"`
		Enabled  *bool              `json:"enabled,omitempty"`
		Type     *string            `json:"type,omitempty"`
		Command  *string            `json:"command,omitempty"`
		Args     *[]string          `json:"args,omitempty"`
		Env      *map[string]string `json:"env,omitempty"`
		URL      *string            `json:"url,omitempty"`
		Headers  *map[string]string `json:"headers,omitempty"`
		OAuth    *db.OAuthConfig    `json:"oauth,omitempty"`
		NodeID   *string            `json:"node_id,omitempty"`
		NodeMode *string            `json:"node_mode,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Update fields if provided
	if body.Name != nil {
		server.Name = *body.Name
	}
	if body.Enabled != nil {
		server.Enabled = *body.Enabled
	}
	if body.Type != nil {
		// Validate type
		if *body.Type != "stdio" && *body.Type != "sse" && *body.Type != "http" && *body.Type != "builtin" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid server type"})
			return
		}
		server.Type = *body.Type
	}
	if body.Command != nil {
		server.Command = *body.Command
	}
	if body.Args != nil {
		server.Args = *body.Args
	}
	if body.Env != nil {
		server.Env = *body.Env
	}
	if body.URL != nil {
		server.URL = *body.URL
	}
	if body.Headers != nil {
		server.Headers = *body.Headers
	}
	if body.OAuth != nil {
		server.OAuth = body.OAuth
	}
	if body.NodeID != nil {
		server.NodeID = *body.NodeID
	}
	if body.NodeMode != nil {
		// Validate node_mode
		if *body.NodeMode != "master" && *body.NodeMode != "specific" && *body.NodeMode != "any" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid node_mode. Must be: master, specific, or any"})
			return
		}
		server.NodeMode = *body.NodeMode
	}

	// Validate node_id is provided when node_mode is "specific"
	if server.NodeMode == "specific" && server.NodeID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "node_id is required when node_mode is 'specific'"})
		return
	}

	if err := api.db.UpdateMCPServer(context.Background(), server); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Server with this name already exists"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		return
	}

	json.NewEncoder(w).Encode(MCPServerResponse{
		ID:        server.ID,
		Name:      server.Name,
		Enabled:   server.Enabled,
		Type:      server.Type,
		Command:   server.Command,
		Args:      server.Args,
		Env:       server.Env,
		URL:       server.URL,
		Headers:   server.Headers,
		OAuth:     server.OAuth,
		NodeID:    server.NodeID,
		NodeMode:  server.NodeMode,
		CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// deleteServer deletes a server
func (api *MCPServersAPI) deleteServer(w http.ResponseWriter, id int64) {
	// First check if server exists
	server, err := api.db.GetMCPServerByID(context.Background(), id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if server == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server not found"})
		return
	}

	if err := api.db.DeleteMCPServer(context.Background(), server.Name); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": strconv.FormatInt(id, 10)})
}

// PlacementResponse represents a placement in API responses
type PlacementResponse struct {
	ID        int64             `json:"id"`
	ServerID  int64             `json:"server_id"`
	HostID    string            `json:"host_id"`
	Enabled   bool              `json:"enabled"`
	Server    MCPServerResponse `json:"server,omitempty"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// handlePlacements handles GET /mcp-placements?host_id=X
func (api *MCPServersAPI) handlePlacements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	hostID := r.URL.Query().Get("host_id")
	if hostID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "host_id query parameter is required"})
		return
	}

	placements, err := api.db.GetPlacementsForHost(context.Background(), hostID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	response := make([]PlacementResponse, len(placements))
	for i, p := range placements {
		response[i] = PlacementResponse{
			ID:       p.ID,
			ServerID: p.ServerID,
			HostID:   p.HostID,
			Enabled:  p.Enabled,
			Server: MCPServerResponse{
				ID:        p.Server.ID,
				Name:      p.Server.Name,
				Enabled:   p.Server.Enabled,
				Type:      p.Server.Type,
				Command:   p.Server.Command,
				Args:      p.Server.Args,
				Env:       p.Server.Env,
				URL:       p.Server.URL,
				Headers:   p.Server.Headers,
				OAuth:     p.Server.OAuth,
				NodeID:    p.Server.NodeID,
				NodeMode:  p.Server.NodeMode,
				CreatedAt: p.Server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt: p.Server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			},
			CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	json.NewEncoder(w).Encode(response)
}

// handlePlacementAction handles PUT /mcp-placements/{serverID}/{hostID}
func (api *MCPServersAPI) handlePlacementAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/mcp-placements/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Both server_id and host_id required in path"})
		return
	}

	serverID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid server ID"})
		return
	}

	hostID := parts[1]

	switch r.Method {
	case http.MethodPut:
		api.updatePlacement(w, r, serverID, hostID)
	case http.MethodDelete:
		api.deletePlacement(w, serverID, hostID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
	}
}

// updatePlacement updates a placement's enabled state
func (api *MCPServersAPI) updatePlacement(w http.ResponseWriter, r *http.Request, serverID int64, hostID string) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Verify server exists
	server, err := api.db.GetMCPServerByID(context.Background(), serverID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if server == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server not found"})
		return
	}

	if err := api.db.SetPlacementEnabled(context.Background(), serverID, hostID, body.Enabled); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Return the updated placement
	placement, err := api.db.GetPlacement(context.Background(), serverID, hostID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(PlacementResponse{
		ID:        placement.ID,
		ServerID:  placement.ServerID,
		HostID:    placement.HostID,
		Enabled:   placement.Enabled,
		CreatedAt: placement.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: placement.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// deletePlacement deletes a placement
func (api *MCPServersAPI) deletePlacement(w http.ResponseWriter, serverID int64, hostID string) {
	if err := api.db.DeletePlacement(context.Background(), serverID, hostID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":    "deleted",
		"server_id": strconv.FormatInt(serverID, 10),
		"host_id":   hostID,
	})
}
