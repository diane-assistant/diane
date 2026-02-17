package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// MCPServerPlacement represents a server deployed to a specific host
type MCPServerPlacement struct {
	ID        int64     `json:"id"`
	ServerID  int64     `json:"server_id"`
	HostID    string    `json:"host_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlacementWithServer includes the full server details
type PlacementWithServer struct {
	MCPServerPlacement
	Server MCPServer `json:"server"`
}

// GetPlacementsForHost returns all server placements for a specific host
func (db *DB) GetPlacementsForHost(hostID string) ([]PlacementWithServer, error) {
	rows, err := db.conn.Query(`
		SELECT 
			p.id, p.server_id, p.host_id, p.enabled, p.created_at, p.updated_at,
			s.id, s.name, s.enabled, s.type, s.command, s.args, s.env, s.url, s.headers, s.oauth, s.node_id, s.node_mode, s.created_at, s.updated_at
		FROM mcp_server_placements p
		JOIN mcp_servers s ON p.server_id = s.id
		WHERE p.host_id = ?
		ORDER BY s.name
	`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var placements []PlacementWithServer
	for rows.Next() {
		var p PlacementWithServer
		var s MCPServer

		// Scan placement and server fields - handle JSON columns properly
		var placementEnabled int
		var argsJSON, envJSON, headersJSON, oauthJSON sql.NullString
		var nodeID, nodeMode sql.NullString

		if err := rows.Scan(
			&p.ID, &p.ServerID, &p.HostID, &placementEnabled, &p.CreatedAt, &p.UpdatedAt,
			&s.ID, &s.Name, &s.Enabled, &s.Type, &s.Command,
			&argsJSON, &envJSON, &s.URL, &headersJSON, &oauthJSON,
			&nodeID, &nodeMode,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}

		// Parse JSON fields
		if argsJSON.Valid && argsJSON.String != "" {
			if err := json.Unmarshal([]byte(argsJSON.String), &s.Args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal args: %w", err)
			}
		}

		if envJSON.Valid && envJSON.String != "" {
			if err := json.Unmarshal([]byte(envJSON.String), &s.Env); err != nil {
				return nil, fmt.Errorf("failed to unmarshal env: %w", err)
			}
		}

		if headersJSON.Valid && headersJSON.String != "" {
			if err := json.Unmarshal([]byte(headersJSON.String), &s.Headers); err != nil {
				return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
			}
		}

		if oauthJSON.Valid && oauthJSON.String != "" {
			var oauth OAuthConfig
			if err := json.Unmarshal([]byte(oauthJSON.String), &oauth); err != nil {
				return nil, fmt.Errorf("failed to unmarshal oauth: %w", err)
			}
			s.OAuth = &oauth
		}

		if nodeID.Valid {
			s.NodeID = nodeID.String
		}

		if nodeMode.Valid {
			s.NodeMode = nodeMode.String
		} else {
			s.NodeMode = "master"
		}

		p.Enabled = (placementEnabled == 1)
		p.Server = s
		placements = append(placements, p)
	}
	return placements, rows.Err()
}

// GetPlacementsForServer returns all host placements for a specific server
func (db *DB) GetPlacementsForServer(serverID int64) ([]MCPServerPlacement, error) {
	rows, err := db.conn.Query(`
		SELECT id, server_id, host_id, enabled, created_at, updated_at
		FROM mcp_server_placements
		WHERE server_id = ?
		ORDER BY host_id
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var placements []MCPServerPlacement
	for rows.Next() {
		var p MCPServerPlacement
		var enabled int
		if err := rows.Scan(&p.ID, &p.ServerID, &p.HostID, &enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Enabled = (enabled == 1)
		placements = append(placements, p)
	}
	return placements, rows.Err()
}

// GetPlacement returns a specific placement by server ID and host ID
func (db *DB) GetPlacement(serverID int64, hostID string) (*MCPServerPlacement, error) {
	var p MCPServerPlacement
	var enabled int
	err := db.conn.QueryRow(`
		SELECT id, server_id, host_id, enabled, created_at, updated_at
		FROM mcp_server_placements
		WHERE server_id = ? AND host_id = ?
	`, serverID, hostID).Scan(&p.ID, &p.ServerID, &p.HostID, &enabled, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.Enabled = (enabled == 1)
	return &p, nil
}

// CreatePlacement creates a new server placement
func (db *DB) CreatePlacement(placement *MCPServerPlacement) error {
	enabled := 0
	if placement.Enabled {
		enabled = 1
	}

	result, err := db.conn.Exec(`
		INSERT INTO mcp_server_placements (server_id, host_id, enabled)
		VALUES (?, ?, ?)
	`, placement.ServerID, placement.HostID, enabled)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	placement.ID = id
	return nil
}

// UpsertPlacement creates or updates a placement
func (db *DB) UpsertPlacement(serverID int64, hostID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err := db.conn.Exec(`
		INSERT INTO mcp_server_placements (server_id, host_id, enabled)
		VALUES (?, ?, ?)
		ON CONFLICT(server_id, host_id) DO UPDATE SET 
			enabled = excluded.enabled,
			updated_at = CURRENT_TIMESTAMP
	`, serverID, hostID, enabledInt)
	return err
}

// SetPlacementEnabled updates the enabled state of a placement
func (db *DB) SetPlacementEnabled(serverID int64, hostID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	result, err := db.conn.Exec(`
		UPDATE mcp_server_placements 
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE server_id = ? AND host_id = ?
	`, enabledInt, serverID, hostID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		// Placement doesn't exist, create it
		return db.UpsertPlacement(serverID, hostID, enabled)
	}
	return nil
}

// DeletePlacement deletes a specific placement
func (db *DB) DeletePlacement(serverID int64, hostID string) error {
	_, err := db.conn.Exec(`
		DELETE FROM mcp_server_placements
		WHERE server_id = ? AND host_id = ?
	`, serverID, hostID)
	return err
}

// DeletePlacementsForServer deletes all placements for a server
func (db *DB) DeletePlacementsForServer(serverID int64) error {
	_, err := db.conn.Exec(`
		DELETE FROM mcp_server_placements
		WHERE server_id = ?
	`, serverID)
	return err
}

// DeletePlacementsForHost deletes all placements for a host
func (db *DB) DeletePlacementsForHost(hostID string) error {
	_, err := db.conn.Exec(`
		DELETE FROM mcp_server_placements
		WHERE host_id = ?
	`, hostID)
	return err
}

// GetEnabledServersForHost returns all enabled servers for a specific host
func (db *DB) GetEnabledServersForHost(hostID string) ([]MCPServer, error) {
	rows, err := db.conn.Query(`
		SELECT s.id, s.name, s.enabled, s.type, s.command, s.args, s.env, s.url, s.headers, s.oauth, s.node_id, s.node_mode, s.created_at, s.updated_at
		FROM mcp_servers s
		JOIN mcp_server_placements p ON s.id = p.server_id
		WHERE p.host_id = ? AND p.enabled = 1 AND s.enabled = 1
		ORDER BY s.name
	`, hostID)
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

// IsServerEnabledOnHost checks if a server is enabled on a specific host
func (db *DB) IsServerEnabledOnHost(serverID int64, hostID string) (bool, error) {
	var serverEnabled int
	var placementEnabled int

	err := db.conn.QueryRow(`
		SELECT s.enabled, p.enabled
		FROM mcp_servers s
		JOIN mcp_server_placements p ON s.id = p.server_id
		WHERE s.id = ? AND p.host_id = ?
	`, serverID, hostID).Scan(&serverEnabled, &placementEnabled)

	if err == sql.ErrNoRows {
		// Server not placed on this host
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Both the server globally and the placement must be enabled
	return serverEnabled == 1 && placementEnabled == 1, nil
}

// BulkSetPlacements sets multiple placements at once for a server
func (db *DB) BulkSetPlacements(serverID int64, placements map[string]bool) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing placements
	if _, err := tx.Exec("DELETE FROM mcp_server_placements WHERE server_id = ?", serverID); err != nil {
		return err
	}

	// Insert new placements
	stmt, err := tx.Prepare(`
		INSERT INTO mcp_server_placements (server_id, host_id, enabled)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for hostID, enabled := range placements {
		enabledInt := 0
		if enabled {
			enabledInt = 1
		}
		if _, err := stmt.Exec(serverID, hostID, enabledInt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// EnsurePlacementExists ensures a placement exists with default disabled state
func (db *DB) EnsurePlacementExists(serverID int64, hostID string) error {
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO mcp_server_placements (server_id, host_id, enabled)
		VALUES (?, ?, 0)
	`, serverID, hostID)
	return err
}

// EnsurePlacementsForAllHosts ensures all servers have placements for all hosts
// This is useful when a new host is added or when ensuring consistency
func (db *DB) EnsurePlacementsForAllHosts() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get all host IDs (master + slaves)
	hostIDs := []string{"master"}
	slaveRows, err := tx.Query("SELECT id FROM slave_servers")
	if err != nil {
		return err
	}
	for slaveRows.Next() {
		var slaveID string
		if err := slaveRows.Scan(&slaveID); err != nil {
			slaveRows.Close()
			return err
		}
		hostIDs = append(hostIDs, slaveID)
	}
	slaveRows.Close()

	// Get all server IDs
	serverRows, err := tx.Query("SELECT id FROM mcp_servers")
	if err != nil {
		return err
	}
	var serverIDs []int64
	for serverRows.Next() {
		var serverID int64
		if err := serverRows.Scan(&serverID); err != nil {
			serverRows.Close()
			return err
		}
		serverIDs = append(serverIDs, serverID)
	}
	serverRows.Close()

	// Ensure placement exists for each server-host combination
	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO mcp_server_placements (server_id, host_id, enabled)
		VALUES (?, ?, 0)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, serverID := range serverIDs {
		for _, hostID := range hostIDs {
			if _, err := stmt.Exec(serverID, hostID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
