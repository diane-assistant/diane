package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

// BuiltinServerDefinition defines metadata for a builtin MCP server
type BuiltinServerDefinition struct {
	Name string
	Type string
}

// EnsureBuiltinServers ensures all builtin MCP servers exist in the database
// This is idempotent - it will only create servers that don't exist
// All builtin servers are created as type="builtin" and enabled=false (secure by default)
func (db *DB) EnsureBuiltinServers() error {
	builtins := []BuiltinServerDefinition{
		{Name: "apple", Type: "builtin"},
		{Name: "google", Type: "builtin"},
		{Name: "infrastructure", Type: "builtin"},
		{Name: "discord", Type: "builtin"},
		{Name: "finance", Type: "builtin"},
		{Name: "places", Type: "builtin"},
		{Name: "weather", Type: "builtin"},
		{Name: "github-bot", Type: "builtin"},
		{Name: "downloads", Type: "builtin"},
		{Name: "file_registry", Type: "builtin"},
	}

	for _, builtin := range builtins {
		// Check if server already exists
		existing, err := db.GetMCPServer(builtin.Name)
		if err != nil {
			return fmt.Errorf("failed to check for builtin server %s: %w", builtin.Name, err)
		}

		if existing != nil {
			// Server already exists, skip
			continue
		}

		// Create the builtin server (disabled by default)
		server := &MCPServer{
			Name:    builtin.Name,
			Type:    builtin.Type,
			Enabled: false, // Secure by default
		}

		if err := db.CreateMCPServer(server); err != nil {
			return fmt.Errorf("failed to create builtin server %s: %w", builtin.Name, err)
		}

		// Create a placement for master node (disabled by default)
		if err := db.UpsertPlacement(server.ID, "master", false); err != nil {
			return fmt.Errorf("failed to create master placement for builtin server %s: %w", builtin.Name, err)
		}
	}

	return nil
}
