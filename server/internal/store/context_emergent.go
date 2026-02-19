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

// EmergentContextStore implements ContextStore against the Emergent knowledge-base graph API.
//
// Mapping:
//
//	Context:
//	  - SQLite table row -> graph object type "context"
//	  - ID (auto-increment) -> properties.legacy_id (int64) + label "legacy_id:{id}"
//	  - Name (unique)       -> properties.name + label "name:{name}"
//	  - Description         -> properties.description
//	  - IsDefault           -> properties.is_default (bool)
//	  - CreatedAt           -> object.CreatedAt (built-in)
//	  - UpdatedAt           -> properties.updated_at (RFC3339Nano)
//
//	ContextServer (many-to-many):
//	  - Stored as labels on context objects:
//	    - "server:{serverName}:enabled" or "server:{serverName}:disabled"
//	    - "server_ref:{serverName}" (for easier querying)
//	  - This avoids complex relationship management
//
//	ContextServerTool (tool overrides):
//	  - Stored in context object properties as nested map:
//	    properties.tool_overrides = {
//	      "{serverName}": {
//	        "{toolName}": true/false,
//	        ...
//	      },
//	      ...
//	    }
type EmergentContextStore struct {
	client *sdk.Client
}

const (
	contextType = "context"
)

// NewEmergentContextStore creates a new Emergent-backed ContextStore.
func NewEmergentContextStore(client *sdk.Client) *EmergentContextStore {
	return &EmergentContextStore{client: client}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contextLegacyIDLabel(id int64) string { return fmt.Sprintf("legacy_id:%d", id) }
func contextNameLabel(name string) string  { return fmt.Sprintf("name:%s", name) }
func serverLabel(serverName string, enabled bool) string {
	if enabled {
		return fmt.Sprintf("server:%s:enabled", serverName)
	}
	return fmt.Sprintf("server:%s:disabled", serverName)
}
func serverRefLabel(serverName string) string { return fmt.Sprintf("server_ref:%s", serverName) }

// contextToProperties converts a db.Context to Emergent properties.
func contextToProperties(c *db.Context) map[string]any {
	props := map[string]any{
		"name":        c.Name,
		"description": c.Description,
		"is_default":  c.IsDefault,
		"updated_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if c.ID != 0 {
		props["legacy_id"] = c.ID
	}
	return props
}

// contextFromObject converts an Emergent GraphObject to a db.Context.
func contextFromObject(obj *graph.GraphObject) (*db.Context, error) {
	c := &db.Context{
		CreatedAt: obj.CreatedAt,
	}

	// legacy_id
	if v, ok := obj.Properties["legacy_id"]; ok {
		switch n := v.(type) {
		case float64:
			c.ID = int64(n)
		case json.Number:
			id, _ := n.Int64()
			c.ID = id
		case int64:
			c.ID = n
		}
	}

	if v, ok := obj.Properties["name"].(string); ok {
		c.Name = v
	}
	if v, ok := obj.Properties["description"].(string); ok {
		c.Description = v
	}
	if v, ok := obj.Properties["is_default"].(bool); ok {
		c.IsDefault = v
	}

	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			c.UpdatedAt = t
		}
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	return c, nil
}

// nextLegacyID allocates the next legacy ID by scanning existing objects.
func (s *EmergentContextStore) nextLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
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
// Context operations
// ---------------------------------------------------------------------------

func (s *EmergentContextStore) ListContexts(ctx context.Context) ([]db.Context, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list contexts: %w", err)
	}

	contexts := make([]db.Context, 0, len(resp.Items))
	for _, obj := range resp.Items {
		c, err := contextFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed context", "object_id", obj.ID, "error", err)
			continue
		}
		contexts = append(contexts, *c)
	}

	// Sort: default first, then by name (match SQLite ORDER BY)
	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].IsDefault != contexts[j].IsDefault {
			return contexts[i].IsDefault // true comes first
		}
		return contexts[i].Name < contexts[j].Name
	})

	return contexts, nil
}

func (s *EmergentContextStore) GetContext(ctx context.Context, name string) (*db.Context, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup context by name %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return contextFromObject(resp.Items[0])
}

func (s *EmergentContextStore) GetDefaultContext(ctx context.Context) (*db.Context, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: contextType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "is_default", Op: "eq", Value: true},
		},
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent get default context: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return contextFromObject(resp.Items[0])
}

func (s *EmergentContextStore) CreateContext(ctx context.Context, c *db.Context) error {
	// Allocate legacy ID if not set
	if c.ID == 0 {
		id, err := s.nextLegacyID(ctx)
		if err != nil {
			return err
		}
		c.ID = id
	}

	props := contextToProperties(c)
	labels := []string{
		contextLegacyIDLabel(c.ID),
		contextNameLabel(c.Name),
	}

	status := "active"
	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       contextType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent create context: %w", err)
	}

	slog.Info("emergent: created context", "name", c.Name, "legacy_id", c.ID, "object_id", obj.ID)
	return nil
}

func (s *EmergentContextStore) UpdateContext(ctx context.Context, c *db.Context) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(c.Name),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context for update: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("context not found: %s", c.Name)
	}

	obj := resp.Items[0]
	props := map[string]any{
		"description": c.Description,
		"updated_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: props,
	})
	if err != nil {
		return fmt.Errorf("emergent update context: %w", err)
	}

	slog.Info("emergent: updated context", "name", c.Name)
	return nil
}

func (s *EmergentContextStore) DeleteContext(ctx context.Context, name string) error {
	// Check if it's the default context
	c, err := s.GetContext(ctx, name)
	if err != nil {
		return err
	}
	if c != nil && c.IsDefault {
		return db.ErrCannotDeleteDefault
	}

	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context for delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, resp.Items[0].ID)
	if err != nil {
		return fmt.Errorf("emergent delete context: %w", err)
	}

	slog.Info("emergent: deleted context", "name", name, "object_id", resp.Items[0].ID)
	return nil
}

func (s *EmergentContextStore) SetDefaultContext(ctx context.Context, name string) error {
	// Get all contexts
	allContexts, err := s.ListContexts(ctx)
	if err != nil {
		return err
	}

	// Update all contexts to unset default, then set the new default
	for _, c := range allContexts {
		resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
			Type:  contextType,
			Label: contextNameLabel(c.Name),
			Limit: 1,
		})
		if err != nil {
			continue
		}
		if len(resp.Items) == 0 {
			continue
		}

		isDefault := (c.Name == name)
		_, err = s.client.Graph.UpdateObject(ctx, resp.Items[0].ID, &graph.UpdateObjectRequest{
			Properties: map[string]any{
				"is_default": isDefault,
				"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
			},
		})
		if err != nil {
			slog.Warn("failed to update context default status", "name", c.Name, "error", err)
		}
	}

	slog.Info("emergent: set default context", "name", name)
	return nil
}

// ---------------------------------------------------------------------------
// ContextServer operations
// ---------------------------------------------------------------------------

func (s *EmergentContextStore) GetServersForContext(ctx context.Context, contextName string) ([]db.ContextServer, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		return []db.ContextServer{}, nil
	}

	obj := resp.Items[0]
	c, _ := contextFromObject(obj)

	servers := make([]db.ContextServer, 0)

	// Parse server labels
	for _, label := range obj.Labels {
		if len(label) > 7 && label[:7] == "server:" {
			// Extract serverName and enabled status
			parts := label[7:] // Remove "server:" prefix
			var serverName string
			var enabled bool

			if len(parts) > 8 && parts[len(parts)-8:] == ":enabled" {
				serverName = parts[:len(parts)-8]
				enabled = true
			} else if len(parts) > 9 && parts[len(parts)-9:] == ":disabled" {
				serverName = parts[:len(parts)-9]
				enabled = false
			} else {
				continue // Invalid label format
			}

			// TODO: Look up server ID by name - for now use legacy_id=0
			servers = append(servers, db.ContextServer{
				ID:         c.ID, // Use context ID
				ContextID:  c.ID,
				ServerID:   0, // Would need to lookup via MCPServerStore
				ServerName: serverName,
				Enabled:    enabled,
			})
		}
	}

	// Sort by server name
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].ServerName < servers[j].ServerName
	})

	return servers, nil
}

func (s *EmergentContextStore) AddServerToContext(ctx context.Context, contextName, serverName string, enabled bool) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("context not found: %s", contextName)
	}

	obj := resp.Items[0]

	// Build new label set
	newLabels := []string{
		contextLegacyIDLabel(int64(obj.Properties["legacy_id"].(float64))),
		contextNameLabel(contextName),
		serverLabel(serverName, enabled),
		serverRefLabel(serverName),
	}

	// Remove old server labels for this server from existing labels
	for _, label := range obj.Labels {
		isOldServer := false
		if label == serverLabel(serverName, true) || label == serverLabel(serverName, false) {
			isOldServer = true
		}
		if label == serverRefLabel(serverName) {
			isOldServer = true
		}
		if !isOldServer {
			// Keep labels that aren't server-related for this server
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
		return fmt.Errorf("emergent add server to context: %w", err)
	}

	slog.Info("emergent: added server to context", "context", contextName, "server", serverName, "enabled", enabled)
	return nil
}

func (s *EmergentContextStore) RemoveServerFromContext(ctx context.Context, contextName, serverName string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("context not found: %s", contextName)
	}

	obj := resp.Items[0]

	// Build new label set without server labels for this server
	newLabels := make([]string, 0, len(obj.Labels))
	for _, label := range obj.Labels {
		if label != serverLabel(serverName, true) &&
			label != serverLabel(serverName, false) &&
			label != serverRefLabel(serverName) {
			newLabels = append(newLabels, label)
		}
	}

	// Also remove tool overrides for this server
	toolOverrides := make(map[string]map[string]bool)
	if v, ok := obj.Properties["tool_overrides"]; ok && v != nil {
		if overridesMap, ok := v.(map[string]interface{}); ok {
			for srvName, tools := range overridesMap {
				if srvName != serverName {
					if toolsMap, ok := tools.(map[string]interface{}); ok {
						serverTools := make(map[string]bool)
						for toolName, enabled := range toolsMap {
							if enabledBool, ok := enabled.(bool); ok {
								serverTools[toolName] = enabledBool
							}
						}
						toolOverrides[srvName] = serverTools
					}
				}
			}
		}
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Labels:        newLabels,
		ReplaceLabels: true,
		Properties: map[string]any{
			"tool_overrides": toolOverrides,
			"updated_at":     time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent remove server from context: %w", err)
	}

	slog.Info("emergent: removed server from context", "context", contextName, "server", serverName)
	return nil
}

func (s *EmergentContextStore) SetServerEnabledInContext(ctx context.Context, contextName, serverName string, enabled bool) error {
	// Check if server exists in context
	servers, err := s.GetServersForContext(ctx, contextName)
	if err != nil {
		return err
	}

	found := false
	for _, srv := range servers {
		if srv.ServerName == serverName {
			found = true
			break
		}
	}

	if !found {
		// Add server if it doesn't exist
		return s.AddServerToContext(ctx, contextName, serverName, enabled)
	}

	// Update existing server
	return s.AddServerToContext(ctx, contextName, serverName, enabled)
}

// ---------------------------------------------------------------------------
// ContextServerTool operations
// ---------------------------------------------------------------------------

func (s *EmergentContextStore) GetToolsForContextServer(ctx context.Context, contextName, serverName string) (map[string]bool, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		return map[string]bool{}, nil
	}

	obj := resp.Items[0]

	// Extract tool overrides for this server
	tools := make(map[string]bool)
	if v, ok := obj.Properties["tool_overrides"]; ok && v != nil {
		if overridesMap, ok := v.(map[string]interface{}); ok {
			if serverTools, ok := overridesMap[serverName].(map[string]interface{}); ok {
				for toolName, enabled := range serverTools {
					if enabledBool, ok := enabled.(bool); ok {
						tools[toolName] = enabledBool
					}
				}
			}
		}
	}

	return tools, nil
}

func (s *EmergentContextStore) SetToolEnabled(ctx context.Context, contextName, serverName, toolName string, enabled bool) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		// Add server first
		if err := s.AddServerToContext(ctx, contextName, serverName, true); err != nil {
			return err
		}
		// Retry lookup
		resp, err = s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
			Type:  contextType,
			Label: contextNameLabel(contextName),
			Limit: 1,
		})
		if err != nil || len(resp.Items) == 0 {
			return fmt.Errorf("context not found after creation: %s", contextName)
		}
	}

	obj := resp.Items[0]

	// Get existing tool overrides
	toolOverrides := make(map[string]map[string]bool)
	if v, ok := obj.Properties["tool_overrides"]; ok && v != nil {
		if overridesMap, ok := v.(map[string]interface{}); ok {
			for srvName, tools := range overridesMap {
				if toolsMap, ok := tools.(map[string]interface{}); ok {
					serverTools := make(map[string]bool)
					for tn, e := range toolsMap {
						if eb, ok := e.(bool); ok {
							serverTools[tn] = eb
						}
					}
					toolOverrides[srvName] = serverTools
				}
			}
		}
	}

	// Update tool for this server
	if toolOverrides[serverName] == nil {
		toolOverrides[serverName] = make(map[string]bool)
	}
	toolOverrides[serverName][toolName] = enabled

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"tool_overrides": toolOverrides,
			"updated_at":     time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent set tool enabled: %w", err)
	}

	slog.Info("emergent: set tool enabled", "context", contextName, "server", serverName, "tool", toolName, "enabled", enabled)
	return nil
}

func (s *EmergentContextStore) BulkSetToolsEnabled(ctx context.Context, contextName, serverName string, tools map[string]bool) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  contextType,
		Label: contextNameLabel(contextName),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup context: %w", err)
	}
	if len(resp.Items) == 0 {
		return db.ErrServerNotInContext
	}

	obj := resp.Items[0]

	// Get existing tool overrides
	toolOverrides := make(map[string]map[string]bool)
	if v, ok := obj.Properties["tool_overrides"]; ok && v != nil {
		if overridesMap, ok := v.(map[string]interface{}); ok {
			for srvName, toolsVal := range overridesMap {
				if toolsMap, ok := toolsVal.(map[string]interface{}); ok {
					serverTools := make(map[string]bool)
					for tn, e := range toolsMap {
						if eb, ok := e.(bool); ok {
							serverTools[tn] = eb
						}
					}
					toolOverrides[srvName] = serverTools
				}
			}
		}
	}

	// Replace tools for this server
	toolOverrides[serverName] = tools

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"tool_overrides": toolOverrides,
			"updated_at":     time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent bulk set tools: %w", err)
	}

	slog.Info("emergent: bulk set tools enabled", "context", contextName, "server", serverName, "count", len(tools))
	return nil
}

// ---------------------------------------------------------------------------
// Context queries (used by mcpproxy)
// ---------------------------------------------------------------------------

func (s *EmergentContextStore) GetContextDetail(ctx context.Context, contextName string) (*db.ContextDetail, error) {
	c, err := s.GetContext(ctx, contextName)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, nil
	}

	detail := &db.ContextDetail{
		Context: *c,
		Servers: []db.ContextServerDetail{},
	}

	// Get all servers in this context
	contextServers, err := s.GetServersForContext(ctx, contextName)
	if err != nil {
		return nil, err
	}

	for _, cs := range contextServers {
		// TODO: Look up full server details via MCPServerStore
		// For now, create a minimal server object
		server := db.MCPServer{
			ID:      cs.ServerID,
			Name:    cs.ServerName,
			Enabled: cs.Enabled,
		}

		tools, err := s.GetToolsForContextServer(ctx, contextName, cs.ServerName)
		if err != nil {
			return nil, err
		}

		detail.Servers = append(detail.Servers, db.ContextServerDetail{
			ContextServer: cs,
			Server:        server,
			Tools:         tools,
		})
	}

	return detail, nil
}

func (s *EmergentContextStore) IsToolEnabledInContext(ctx context.Context, contextName, serverName, toolName string) (bool, error) {
	servers, err := s.GetServersForContext(ctx, contextName)
	if err != nil {
		return false, err
	}

	// Check if server is in context and enabled
	serverEnabled := false
	for _, srv := range servers {
		if srv.ServerName == serverName {
			serverEnabled = srv.Enabled
			break
		}
	}

	if !serverEnabled {
		return false, nil
	}

	// Check tool override
	tools, err := s.GetToolsForContextServer(ctx, contextName, serverName)
	if err != nil {
		return false, err
	}

	// If no tool override, default to enabled
	if enabled, ok := tools[toolName]; ok {
		return enabled, nil
	}

	return true, nil // Default to enabled
}

func (s *EmergentContextStore) GetEnabledServersForContext(ctx context.Context, contextName string) ([]db.MCPServer, error) {
	servers, err := s.GetServersForContext(ctx, contextName)
	if err != nil {
		return nil, err
	}

	enabled := make([]db.MCPServer, 0)
	for _, srv := range servers {
		if srv.Enabled {
			// TODO: Look up full server via MCPServerStore and check s.Enabled
			// For now, create a minimal server object
			enabled = append(enabled, db.MCPServer{
				ID:      srv.ServerID,
				Name:    srv.ServerName,
				Enabled: true,
			})
		}
	}

	return enabled, nil
}
