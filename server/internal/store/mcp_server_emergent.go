package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentMCPServerStore implements MCPServerStore against the Emergent knowledge-base graph API.
//
// Mapping:
//
//	MCPServer:
//	  - SQLite table row -> graph object type "mcp_server"
//	  - ID (auto-increment) -> properties.legacy_id (int64) + label "legacy_id:{id}"
//	  - Name (unique)       -> properties.name + label "name:{name}"
//	  - Enabled             -> properties.enabled (bool)
//	  - Type                -> properties.type (stdio/sse/http/builtin)
//	  - Command             -> properties.command
//	  - Args                -> properties.args ([]string, JSON)
//	  - Env                 -> properties.env (map[string]string, JSON)
//	  - URL                 -> properties.url
//	  - Headers             -> properties.headers (map[string]string, JSON)
//	  - OAuth               -> properties.oauth (nested JSON)
//	  - NodeID              -> properties.node_id
//	  - NodeMode            -> properties.node_mode
//	  - CreatedAt           -> object.CreatedAt (built-in)
//	  - UpdatedAt           -> properties.updated_at (RFC3339Nano)
//
//	MCPServerPlacement:
//	  - SQLite table row -> graph relationship type "mcp_server_placement"
//	  - ID (auto-increment) -> properties.legacy_id (int64)
//	  - ServerID (FK)       -> relationship srcID (mcp_server object)
//	  - HostID              -> relationship dstID (slave_server object or synthetic "master" object)
//	  - Enabled             -> properties.enabled (bool)
//	  - CreatedAt           -> relationship.CreatedAt (built-in)
//	  - UpdatedAt           -> properties.updated_at (RFC3339Nano)
//
//	Alternative placement mapping (simpler):
//	  - Use labels on mcp_server objects: "placement:{hostID}:enabled" or "placement:{hostID}:disabled"
//	  - This avoids relationship complexity but loses some relational integrity
//	  - For now, we'll use this approach for simplicity
type EmergentMCPServerStore struct {
	client *sdk.Client
}

const (
	mcpServerType = "mcp_server"
)

// NewEmergentMCPServerStore creates a new Emergent-backed MCPServerStore.
func NewEmergentMCPServerStore(client *sdk.Client) *EmergentMCPServerStore {
	return &EmergentMCPServerStore{client: client}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mcpLegacyIDLabel(id int64) string      { return fmt.Sprintf("legacy_id:%d", id) }
func mcpServerNameLabel(name string) string { return fmt.Sprintf("name:%s", name) }
func placementLabel(hostID string, enabled bool) string {
	if enabled {
		return fmt.Sprintf("placement:%s:enabled", hostID)
	}
	return fmt.Sprintf("placement:%s:disabled", hostID)
}
func placementHostLabel(hostID string) string { return fmt.Sprintf("placement_host:%s", hostID) }

// mcpServerToProperties converts a db.MCPServer to Emergent properties.
func mcpServerToProperties(s *db.MCPServer) map[string]any {
	props := map[string]any{
		"name":       s.Name,
		"enabled":    s.Enabled,
		"type":       s.Type,
		"command":    s.Command,
		"url":        s.URL,
		"node_id":    s.NodeID,
		"node_mode":  s.NodeMode,
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if s.ID != 0 {
		props["legacy_id"] = s.ID
	}
	if s.Args != nil {
		props["args"] = s.Args
	}
	if s.Env != nil {
		props["env"] = s.Env
	}
	if s.Headers != nil {
		props["headers"] = s.Headers
	}
	if s.OAuth != nil {
		props["oauth"] = s.OAuth
	}
	return props
}

// mcpServerFromObject converts an Emergent GraphObject to a db.MCPServer.
func mcpServerFromObject(obj *graph.GraphObject) (*db.MCPServer, error) {
	s := &db.MCPServer{
		CreatedAt: obj.CreatedAt,
	}

	// legacy_id
	if v, ok := obj.Properties["legacy_id"]; ok {
		switch n := v.(type) {
		case float64:
			s.ID = int64(n)
		case json.Number:
			id, _ := n.Int64()
			s.ID = id
		case int64:
			s.ID = n
		}
	}

	if v, ok := obj.Properties["name"].(string); ok {
		s.Name = v
	}
	if v, ok := obj.Properties["enabled"].(bool); ok {
		s.Enabled = v
	}
	if v, ok := obj.Properties["type"].(string); ok {
		s.Type = v
	}
	if v, ok := obj.Properties["command"].(string); ok {
		s.Command = v
	}
	if v, ok := obj.Properties["url"].(string); ok {
		s.URL = v
	}
	if v, ok := obj.Properties["node_id"].(string); ok {
		s.NodeID = v
	}
	if v, ok := obj.Properties["node_mode"].(string); ok {
		s.NodeMode = v
	}

	// Parse JSON fields
	if v, ok := obj.Properties["args"]; ok && v != nil {
		if args, ok := v.([]interface{}); ok {
			s.Args = make([]string, 0, len(args))
			for _, a := range args {
				if str, ok := a.(string); ok {
					s.Args = append(s.Args, str)
				}
			}
		}
	}

	if v, ok := obj.Properties["env"]; ok && v != nil {
		s.Env = toMapStringString(v)
	}

	if v, ok := obj.Properties["headers"]; ok && v != nil {
		s.Headers = toMapStringString(v)
	}

	if v, ok := obj.Properties["oauth"]; ok && v != nil {
		if oauthMap, ok := v.(map[string]interface{}); ok {
			oauth := &db.OAuthConfig{}
			if provider, ok := oauthMap["provider"].(string); ok {
				oauth.Provider = provider
			}
			if clientID, ok := oauthMap["client_id"].(string); ok {
				oauth.ClientID = clientID
			}
			if clientSecret, ok := oauthMap["client_secret"].(string); ok {
				oauth.ClientSecret = clientSecret
			}
			if deviceAuthURL, ok := oauthMap["device_auth_url"].(string); ok {
				oauth.DeviceAuthURL = deviceAuthURL
			}
			if tokenURL, ok := oauthMap["token_url"].(string); ok {
				oauth.TokenURL = tokenURL
			}
			if scopes, ok := oauthMap["scopes"].([]interface{}); ok {
				oauth.Scopes = make([]string, 0, len(scopes))
				for _, scope := range scopes {
					if s, ok := scope.(string); ok {
						oauth.Scopes = append(oauth.Scopes, s)
					}
				}
			}
			s.OAuth = oauth
		}
	}

	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.UpdatedAt = t
		}
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}

	return s, nil
}

// toMapStringString converts an interface{} to map[string]string.
func toMapStringString(v interface{}) map[string]string {
	result := make(map[string]string)
	if m, ok := v.(map[string]interface{}); ok {
		for k, val := range m {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}

// nextLegacyID allocates the next legacy ID by scanning existing objects.
func (s *EmergentMCPServerStore) nextLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list for next ID: %w", err)
	}

	var maxID int64
	for _, obj := range resp.Items {
		if v, ok := obj.Properties["legacy_id"]; ok {
			var id int64
			switch n := v.(type) {
			case float64:
				id = int64(n)
			case json.Number:
				id, _ = n.Int64()
			case int64:
				id = n
			}
			if id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1, nil
}

// ---------------------------------------------------------------------------
// MCPServer operations
// ---------------------------------------------------------------------------

func (s *EmergentMCPServerStore) ListMCPServers(ctx context.Context) ([]db.MCPServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list mcp servers: %w", err)
	}

	servers := make([]db.MCPServer, 0, len(resp.Items))
	for _, obj := range resp.Items {
		srv, err := mcpServerFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed mcp server", "object_id", obj.ID, "error", err)
			continue
		}
		servers = append(servers, *srv)
	}

	// Sort by name (match SQLite ORDER BY)
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers, nil
}

func (s *EmergentMCPServerStore) GetMCPServer(ctx context.Context, name string) (*db.MCPServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpServerNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup mcp server by name %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return mcpServerFromObject(resp.Items[0])
}

func (s *EmergentMCPServerStore) GetMCPServerByID(ctx context.Context, id int64) (*db.MCPServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup mcp server by id %d: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return mcpServerFromObject(resp.Items[0])
}

func (s *EmergentMCPServerStore) CreateMCPServer(ctx context.Context, server *db.MCPServer) error {
	// Allocate legacy ID if not set
	if server.ID == 0 {
		id, err := s.nextLegacyID(ctx)
		if err != nil {
			return err
		}
		server.ID = id
	}

	props := mcpServerToProperties(server)
	labels := []string{
		mcpLegacyIDLabel(server.ID),
		mcpServerNameLabel(server.Name),
	}

	status := "active"
	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       mcpServerType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent create mcp server: %w", err)
	}

	slog.Info("emergent: created mcp server", "name", server.Name, "legacy_id", server.ID, "object_id", obj.ID)
	return nil
}

func (s *EmergentMCPServerStore) UpdateMCPServer(ctx context.Context, server *db.MCPServer) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(server.ID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup mcp server for update: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("mcp server not found: id=%d", server.ID)
	}

	obj := resp.Items[0]
	props := mcpServerToProperties(server)

	// Update labels if name changed
	labels := []string{
		mcpLegacyIDLabel(server.ID),
		mcpServerNameLabel(server.Name),
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties:    props,
		Labels:        labels,
		ReplaceLabels: true,
	})
	if err != nil {
		return fmt.Errorf("emergent update mcp server: %w", err)
	}

	slog.Info("emergent: updated mcp server", "name", server.Name, "legacy_id", server.ID)
	return nil
}

func (s *EmergentMCPServerStore) DeleteMCPServer(ctx context.Context, name string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpServerNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup mcp server for delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, resp.Items[0].ID)
	if err != nil {
		return fmt.Errorf("emergent delete mcp server: %w", err)
	}

	slog.Info("emergent: deleted mcp server", "name", name, "object_id", resp.Items[0].ID)
	return nil
}

func (s *EmergentMCPServerStore) CountMCPServers(ctx context.Context) (int, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent count mcp servers: %w", err)
	}
	return len(resp.Items), nil
}

func (s *EmergentMCPServerStore) EnsureBuiltinServers(ctx context.Context) error {
	builtins := []db.BuiltinServerDefinition{
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
		existing, err := s.GetMCPServer(ctx, builtin.Name)
		if err != nil {
			return fmt.Errorf("failed to check for builtin server %s: %w", builtin.Name, err)
		}

		if existing != nil {
			// Server already exists, skip
			continue
		}

		// Create the builtin server (disabled by default)
		server := &db.MCPServer{
			Name:    builtin.Name,
			Type:    builtin.Type,
			Enabled: false, // Secure by default
		}

		if err := s.CreateMCPServer(ctx, server); err != nil {
			return fmt.Errorf("failed to create builtin server %s: %w", builtin.Name, err)
		}

		// Create a placement for master node (disabled by default)
		if err := s.UpsertPlacement(ctx, server.ID, "master", false); err != nil {
			return fmt.Errorf("failed to create master placement for builtin server %s: %w", builtin.Name, err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// MCPServerPlacement operations
// ---------------------------------------------------------------------------

// Placement info is stored as labels on the mcp_server objects:
// - "placement:{hostID}:enabled" or "placement:{hostID}:disabled"
// - "placement_host:{hostID}" (for easier querying)

func (s *EmergentMCPServerStore) GetPlacementsForHost(ctx context.Context, hostID string) ([]db.PlacementWithServer, error) {
	// Find all servers that have placement labels for this host
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: placementHostLabel(hostID),
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent get placements for host: %w", err)
	}

	placements := make([]db.PlacementWithServer, 0, len(resp.Items))
	for _, obj := range resp.Items {
		srv, err := mcpServerFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed server in placements", "object_id", obj.ID, "error", err)
			continue
		}

		// Determine if placement is enabled by checking labels
		enabled := false
		for _, label := range obj.Labels {
			if label == placementLabel(hostID, true) {
				enabled = true
				break
			}
		}

		placements = append(placements, db.PlacementWithServer{
			MCPServerPlacement: db.MCPServerPlacement{
				ID:        srv.ID, // Use server ID as placement ID for simplicity
				ServerID:  srv.ID,
				HostID:    hostID,
				Enabled:   enabled,
				CreatedAt: obj.CreatedAt,
				UpdatedAt: srv.UpdatedAt,
			},
			Server: *srv,
		})
	}

	// Sort by server name
	sort.Slice(placements, func(i, j int) bool {
		return placements[i].Server.Name < placements[j].Server.Name
	})

	return placements, nil
}

func (s *EmergentMCPServerStore) GetPlacementsForServer(ctx context.Context, serverID int64) ([]db.MCPServerPlacement, error) {
	srv, err := s.GetMCPServerByID(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if srv == nil {
		return []db.MCPServerPlacement{}, nil
	}

	// Get the object to read its labels
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(serverID),
		Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return []db.MCPServerPlacement{}, nil
	}

	obj := resp.Items[0]
	placements := make([]db.MCPServerPlacement, 0)

	// Parse placement labels
	for _, label := range obj.Labels {
		if len(label) > 10 && label[:10] == "placement:" {
			// Extract hostID and enabled status
			parts := label[10:] // Remove "placement:" prefix
			var hostID string
			var enabled bool

			if len(parts) > 8 && parts[len(parts)-8:] == ":enabled" {
				hostID = parts[:len(parts)-8]
				enabled = true
			} else if len(parts) > 9 && parts[len(parts)-9:] == ":disabled" {
				hostID = parts[:len(parts)-9]
				enabled = false
			} else {
				continue // Invalid label format
			}

			placements = append(placements, db.MCPServerPlacement{
				ID:        serverID, // Use server ID
				ServerID:  serverID,
				HostID:    hostID,
				Enabled:   enabled,
				CreatedAt: obj.CreatedAt,
				UpdatedAt: srv.UpdatedAt,
			})
		}
	}

	// Sort by host_id
	sort.Slice(placements, func(i, j int) bool {
		return placements[i].HostID < placements[j].HostID
	})

	return placements, nil
}

func (s *EmergentMCPServerStore) GetPlacement(ctx context.Context, serverID int64, hostID string) (*db.MCPServerPlacement, error) {
	placements, err := s.GetPlacementsForServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	for _, p := range placements {
		if p.HostID == hostID {
			return &p, nil
		}
	}

	return nil, nil
}

func (s *EmergentMCPServerStore) CreatePlacement(ctx context.Context, placement *db.MCPServerPlacement) error {
	return s.UpsertPlacement(ctx, placement.ServerID, placement.HostID, placement.Enabled)
}

func (s *EmergentMCPServerStore) UpsertPlacement(ctx context.Context, serverID int64, hostID string, enabled bool) error {
	// Get the server object
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(serverID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup server for placement: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("server not found: id=%d", serverID)
	}

	obj := resp.Items[0]

	// Build new label set
	newLabels := []string{
		mcpLegacyIDLabel(serverID),
		mcpServerNameLabel(obj.Properties["name"].(string)),
		placementLabel(hostID, enabled),
		placementHostLabel(hostID),
	}

	// Remove old placement labels for this host from existing labels
	for _, label := range obj.Labels {
		isOldPlacement := false
		if label == placementLabel(hostID, true) || label == placementLabel(hostID, false) {
			isOldPlacement = true
		}
		if label == placementHostLabel(hostID) {
			isOldPlacement = true
		}
		if !isOldPlacement {
			// Keep labels that aren't placement-related for this host
			isDuplicate := false
			for _, nl := range newLabels {
				if nl == label {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				newLabels = append(newLabels, label)
			}
		}
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Labels:        newLabels,
		ReplaceLabels: true,
		Properties: map[string]any{
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent upsert placement: %w", err)
	}

	slog.Info("emergent: upserted placement", "server_id", serverID, "host_id", hostID, "enabled", enabled)
	return nil
}

func (s *EmergentMCPServerStore) SetPlacementEnabled(ctx context.Context, serverID int64, hostID string, enabled bool) error {
	return s.UpsertPlacement(ctx, serverID, hostID, enabled)
}

func (s *EmergentMCPServerStore) DeletePlacement(ctx context.Context, serverID int64, hostID string) error {
	// Get the server object
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(serverID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup server for placement delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Server doesn't exist, nothing to delete
	}

	obj := resp.Items[0]

	// Build new label set without placement labels for this host
	newLabels := make([]string, 0, len(obj.Labels))
	for _, label := range obj.Labels {
		if label != placementLabel(hostID, true) &&
			label != placementLabel(hostID, false) &&
			label != placementHostLabel(hostID) {
			newLabels = append(newLabels, label)
		}
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Labels:        newLabels,
		ReplaceLabels: true,
		Properties: map[string]any{
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent delete placement: %w", err)
	}

	slog.Info("emergent: deleted placement", "server_id", serverID, "host_id", hostID)
	return nil
}

func (s *EmergentMCPServerStore) DeletePlacementsForServer(ctx context.Context, serverID int64) error {
	// Get the server object
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: mcpLegacyIDLabel(serverID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup server for placements delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Server doesn't exist
	}

	obj := resp.Items[0]

	// Build new label set without any placement labels
	newLabels := make([]string, 0, len(obj.Labels))
	for _, label := range obj.Labels {
		if len(label) < 10 || label[:10] != "placement:" {
			if len(label) < 16 || label[:16] != "placement_host:" {
				newLabels = append(newLabels, label)
			}
		}
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Labels:        newLabels,
		ReplaceLabels: true,
		Properties: map[string]any{
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent delete placements for server: %w", err)
	}

	slog.Info("emergent: deleted all placements for server", "server_id", serverID)
	return nil
}

func (s *EmergentMCPServerStore) DeletePlacementsForHost(ctx context.Context, hostID string) error {
	// Find all servers with placements for this host
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  mcpServerType,
		Label: placementHostLabel(hostID),
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("emergent find servers for host placements delete: %w", err)
	}

	for _, obj := range resp.Items {
		// Remove placement labels for this host
		newLabels := make([]string, 0, len(obj.Labels))
		for _, label := range obj.Labels {
			if label != placementLabel(hostID, true) &&
				label != placementLabel(hostID, false) &&
				label != placementHostLabel(hostID) {
				newLabels = append(newLabels, label)
			}
		}

		_, err := s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
			Labels:        newLabels,
			ReplaceLabels: true,
			Properties: map[string]any{
				"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
			},
		})
		if err != nil {
			slog.Warn("failed to delete placement for host", "object_id", obj.ID, "host_id", hostID, "error", err)
			continue
		}
	}

	slog.Info("emergent: deleted all placements for host", "host_id", hostID)
	return nil
}

func (s *EmergentMCPServerStore) GetEnabledServersForHost(ctx context.Context, hostID string) ([]db.MCPServer, error) {
	placements, err := s.GetPlacementsForHost(ctx, hostID)
	if err != nil {
		return nil, err
	}

	servers := make([]db.MCPServer, 0)
	for _, p := range placements {
		if p.Enabled && p.Server.Enabled {
			servers = append(servers, p.Server)
		}
	}

	return servers, nil
}

func (s *EmergentMCPServerStore) IsServerEnabledOnHost(ctx context.Context, serverID int64, hostID string) (bool, error) {
	placement, err := s.GetPlacement(ctx, serverID, hostID)
	if err != nil {
		return false, err
	}
	if placement == nil {
		return false, nil
	}

	// Check both placement and server enabled status
	srv, err := s.GetMCPServerByID(ctx, serverID)
	if err != nil {
		return false, err
	}
	if srv == nil {
		return false, nil
	}

	return placement.Enabled && srv.Enabled, nil
}

func (s *EmergentMCPServerStore) BulkSetPlacements(ctx context.Context, serverID int64, placements map[string]bool) error {
	// Delete all existing placements
	if err := s.DeletePlacementsForServer(ctx, serverID); err != nil {
		return err
	}

	// Create new placements
	for hostID, enabled := range placements {
		if err := s.UpsertPlacement(ctx, serverID, hostID, enabled); err != nil {
			return err
		}
	}

	return nil
}

func (s *EmergentMCPServerStore) EnsurePlacementExists(ctx context.Context, serverID int64, hostID string) error {
	placement, err := s.GetPlacement(ctx, serverID, hostID)
	if err != nil {
		return err
	}
	if placement != nil {
		return nil // Already exists
	}

	// Create disabled placement
	return s.UpsertPlacement(ctx, serverID, hostID, false)
}

func (s *EmergentMCPServerStore) EnsurePlacementsForAllHosts(ctx context.Context) error {
	// This method would need access to slave store to get all hosts
	// For now, we'll leave it as a no-op since placement creation is lazy
	// TODO: This needs to be refactored to accept a list of hostIDs
	slog.Warn("emergent: EnsurePlacementsForAllHosts not fully implemented (lazy placement creation)")
	return nil
}
