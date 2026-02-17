package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// OAuthConfig represents OAuth configuration for an MCP server
type OAuthConfig struct {
	Provider      string   `json:"provider,omitempty"`
	ClientID      string   `json:"client_id,omitempty"`
	ClientSecret  string   `json:"client_secret,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	DeviceAuthURL string   `json:"device_auth_url,omitempty"`
	TokenURL      string   `json:"token_url,omitempty"`
}

// MCPServer represents an MCP server in the database
type MCPServer struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Type      string            `json:"type"` // stdio, sse, http, builtin
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	OAuth     *OAuthConfig      `json:"oauth,omitempty"`
	NodeID    string            `json:"node_id,omitempty"`   // Target slave hostname
	NodeMode  string            `json:"node_mode,omitempty"` // "master", "specific", "any"
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// ListMCPServers returns all MCP servers
func (db *DB) ListMCPServers() ([]MCPServer, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, enabled, type, command, args, env, url, headers, oauth, node_id, node_mode, created_at, updated_at
		FROM mcp_servers
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []MCPServer
	for rows.Next() {
		s, err := scanMCPServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

// GetMCPServer returns a single MCP server by name
func (db *DB) GetMCPServer(name string) (*MCPServer, error) {
	row := db.conn.QueryRow(`
		SELECT id, name, enabled, type, command, args, env, url, headers, oauth, node_id, node_mode, created_at, updated_at
		FROM mcp_servers
		WHERE name = ?
	`, name)

	s, err := scanMCPServerRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetMCPServerByID returns a single MCP server by ID
func (db *DB) GetMCPServerByID(id int64) (*MCPServer, error) {
	row := db.conn.QueryRow(`
		SELECT id, name, enabled, type, command, args, env, url, headers, oauth, node_id, node_mode, created_at, updated_at
		FROM mcp_servers
		WHERE id = ?
	`, id)

	s, err := scanMCPServerRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateMCPServer creates a new MCP server
func (db *DB) CreateMCPServer(server *MCPServer) error {
	argsJSON, err := json.Marshal(server.Args)
	if err != nil {
		return fmt.Errorf("failed to marshal args: %w", err)
	}

	envJSON, err := json.Marshal(server.Env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	headersJSON, err := json.Marshal(server.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	var oauthJSON []byte
	if server.OAuth != nil {
		oauthJSON, err = json.Marshal(server.OAuth)
		if err != nil {
			return fmt.Errorf("failed to marshal oauth: %w", err)
		}
	}

	result, err := db.conn.Exec(`
		INSERT INTO mcp_servers (name, enabled, type, command, args, env, url, headers, oauth, node_id, node_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, server.Name, server.Enabled, server.Type, server.Command,
		string(argsJSON), string(envJSON), server.URL, string(headersJSON), string(oauthJSON),
		server.NodeID, server.NodeMode)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	server.ID = id
	return nil
}

// UpdateMCPServer updates an existing MCP server
func (db *DB) UpdateMCPServer(server *MCPServer) error {
	argsJSON, err := json.Marshal(server.Args)
	if err != nil {
		return fmt.Errorf("failed to marshal args: %w", err)
	}

	envJSON, err := json.Marshal(server.Env)
	if err != nil {
		return fmt.Errorf("failed to marshal env: %w", err)
	}

	headersJSON, err := json.Marshal(server.Headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	var oauthJSON []byte
	if server.OAuth != nil {
		oauthJSON, err = json.Marshal(server.OAuth)
		if err != nil {
			return fmt.Errorf("failed to marshal oauth: %w", err)
		}
	}

	_, err = db.conn.Exec(`
		UPDATE mcp_servers 
		SET name = ?, enabled = ?, type = ?, command = ?, args = ?, env = ?, url = ?, headers = ?, oauth = ?, node_id = ?, node_mode = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, server.Name, server.Enabled, server.Type, server.Command,
		string(argsJSON), string(envJSON), server.URL, string(headersJSON), string(oauthJSON),
		server.NodeID, server.NodeMode, server.ID)
	return err
}

// DeleteMCPServer deletes an MCP server by name
func (db *DB) DeleteMCPServer(name string) error {
	_, err := db.conn.Exec("DELETE FROM mcp_servers WHERE name = ?", name)
	return err
}

// CountMCPServers returns the number of MCP servers
func (db *DB) CountMCPServers() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM mcp_servers").Scan(&count)
	return count, err
}

// MigrateFromJSON performs one-time migration from mcp-servers.json to database.
// It checks if migration is needed (no servers in DB + JSON file exists) and:
// 1. Imports all servers from the JSON file
// 2. Creates a "default" context if none exists
// 3. Adds all imported servers to the default context with all tools enabled
// Returns the number of servers imported and any error.
// After migration the JSON file is no longer used; the database is the sole source of truth.
func (db *DB) MigrateFromJSON(jsonPath string) (int, error) {
	// Check if we already have servers in the database
	count, err := db.CountMCPServers()
	if err != nil {
		return 0, fmt.Errorf("failed to count existing servers: %w", err)
	}

	// If we already have servers, no migration needed
	if count > 0 {
		return 0, nil
	}

	// Check if JSON file exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return 0, nil // No JSON file, nothing to migrate
	}

	// Read and parse the JSON file
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read JSON file: %w", err)
	}

	var config struct {
		Servers []struct {
			Name    string            `json:"name"`
			Enabled bool              `json:"enabled"`
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			URL     string            `json:"url,omitempty"`
			Headers map[string]string `json:"headers,omitempty"`
			OAuth   *OAuthConfig      `json:"oauth,omitempty"`
		} `json:"servers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Import servers
	imported := 0
	for _, s := range config.Servers {
		serverType := s.Type
		if serverType == "" {
			serverType = "stdio"
		}

		server := &MCPServer{
			Name:    s.Name,
			Enabled: s.Enabled,
			Type:    serverType,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			URL:     s.URL,
			Headers: s.Headers,
			OAuth:   s.OAuth,
		}

		if err := db.CreateMCPServer(server); err != nil {
			return imported, fmt.Errorf("failed to create server %s: %w", s.Name, err)
		}
		imported++
	}

	if imported == 0 {
		return 0, nil
	}

	// Create or get the default context
	defaultCtx, err := db.GetDefaultContext()
	if err != nil {
		return imported, fmt.Errorf("failed to get default context: %w", err)
	}

	var contextName string
	if defaultCtx == nil {
		// Create a default context
		ctx := &Context{
			Name:        "default",
			Description: "Default context with all MCP servers",
			IsDefault:   true,
		}
		if err := db.CreateContext(ctx); err != nil {
			return imported, fmt.Errorf("failed to create default context: %w", err)
		}
		contextName = ctx.Name
	} else {
		contextName = defaultCtx.Name
	}

	// Add all imported servers to the default context
	servers, err := db.ListMCPServers()
	if err != nil {
		return imported, fmt.Errorf("failed to list servers: %w", err)
	}

	for _, server := range servers {
		if err := db.AddServerToContext(contextName, server.Name, true); err != nil {
			continue
		}
	}

	return imported, nil
}

// scanMCPServer scans a row into an MCPServer
func scanMCPServer(rows *sql.Rows) (MCPServer, error) {
	var s MCPServer
	var argsJSON, envJSON, headersJSON, oauthJSON sql.NullString
	var nodeID, nodeMode sql.NullString

	err := rows.Scan(
		&s.ID, &s.Name, &s.Enabled, &s.Type, &s.Command,
		&argsJSON, &envJSON, &s.URL, &headersJSON, &oauthJSON,
		&nodeID, &nodeMode,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return s, err
	}

	if argsJSON.Valid && argsJSON.String != "" {
		if err := json.Unmarshal([]byte(argsJSON.String), &s.Args); err != nil {
			return s, fmt.Errorf("failed to unmarshal args: %w", err)
		}
	}

	if envJSON.Valid && envJSON.String != "" {
		if err := json.Unmarshal([]byte(envJSON.String), &s.Env); err != nil {
			return s, fmt.Errorf("failed to unmarshal env: %w", err)
		}
	}

	if headersJSON.Valid && headersJSON.String != "" {
		if err := json.Unmarshal([]byte(headersJSON.String), &s.Headers); err != nil {
			return s, fmt.Errorf("failed to unmarshal headers: %w", err)
		}
	}

	if oauthJSON.Valid && oauthJSON.String != "" {
		var oauth OAuthConfig
		if err := json.Unmarshal([]byte(oauthJSON.String), &oauth); err != nil {
			return s, fmt.Errorf("failed to unmarshal oauth: %w", err)
		}
		s.OAuth = &oauth
	}

	if nodeID.Valid {
		s.NodeID = nodeID.String
	}

	if nodeMode.Valid {
		s.NodeMode = nodeMode.String
	} else {
		s.NodeMode = "master" // default value
	}

	return s, nil
}

// scanMCPServerRow scans a single row into an MCPServer
func scanMCPServerRow(row *sql.Row) (MCPServer, error) {
	var s MCPServer
	var argsJSON, envJSON, headersJSON, oauthJSON sql.NullString
	var nodeID, nodeMode sql.NullString

	err := row.Scan(
		&s.ID, &s.Name, &s.Enabled, &s.Type, &s.Command,
		&argsJSON, &envJSON, &s.URL, &headersJSON, &oauthJSON,
		&nodeID, &nodeMode,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return s, err
	}

	if argsJSON.Valid && argsJSON.String != "" {
		if err := json.Unmarshal([]byte(argsJSON.String), &s.Args); err != nil {
			return s, fmt.Errorf("failed to unmarshal args: %w", err)
		}
	}

	if envJSON.Valid && envJSON.String != "" {
		if err := json.Unmarshal([]byte(envJSON.String), &s.Env); err != nil {
			return s, fmt.Errorf("failed to unmarshal env: %w", err)
		}
	}

	if headersJSON.Valid && headersJSON.String != "" {
		if err := json.Unmarshal([]byte(headersJSON.String), &s.Headers); err != nil {
			return s, fmt.Errorf("failed to unmarshal headers: %w", err)
		}
	}

	if oauthJSON.Valid && oauthJSON.String != "" {
		var oauth OAuthConfig
		if err := json.Unmarshal([]byte(oauthJSON.String), &oauth); err != nil {
			return s, fmt.Errorf("failed to unmarshal oauth: %w", err)
		}
		s.OAuth = &oauth
	}

	if nodeID.Valid {
		s.NodeID = nodeID.String
	}

	if nodeMode.Valid {
		s.NodeMode = nodeMode.String
	} else {
		s.NodeMode = "master" // default value
	}

	return s, nil
}
