package store

// TODO(emergent-migration): Provider remaining phases:
//   Phase 2: Swap primary/secondary in buildProviderStore (api.go) so Emergent is primary
//   Phase 3: Drop dual-write, use EmergentProviderStore directly
//   Phase 4: Delete provider_sqlite.go, remove SQLite provider methods from db/providers.go
//   Phase 5: Write one-time migration script to seed Emergent from existing SQLite data

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentProviderStore implements ProviderStore against the Emergent
// knowledge-base graph API. Each SQLite provider row maps to a graph
// object of type "provider".
//
// Mapping:
//
//	SQLite ID        -> properties.legacy_id  (int64, kept for API compat)
//	                    + label "legacy_id:{id}" (for fast lookup)
//	Name (unique)    -> properties.name + label "name:{name}"
//	Type             -> properties.type  (embedding | llm | storage)
//	Service          -> properties.service
//	Enabled          -> properties.enabled (bool)
//	IsDefault        -> properties.is_default (bool)
//	AuthType         -> properties.auth_type
//	AuthConfig       -> properties.auth_config (nested JSON)
//	Config           -> properties.config (nested JSON)
//	CreatedAt        -> object.CreatedAt (built-in)
//	UpdatedAt        -> properties.updated_at (explicit, since Emergent
//	                    versioning may differ from our update semantics)
type EmergentProviderStore struct {
	client *sdk.Client
}

const providerType = "provider"

// NewEmergentProviderStore creates a new Emergent-backed ProviderStore.
func NewEmergentProviderStore(client *sdk.Client) *EmergentProviderStore {
	return &EmergentProviderStore{client: client}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func legacyIDLabel(id int64) string     { return fmt.Sprintf("legacy_id:%d", id) }
func providerNameLabel(n string) string { return fmt.Sprintf("name:%s", n) }

// toProperties converts a db.Provider to Emergent properties map.
func toProperties(p *db.Provider) map[string]any {
	props := map[string]any{
		"name":       p.Name,
		"type":       string(p.Type),
		"service":    p.Service,
		"enabled":    p.Enabled,
		"is_default": p.IsDefault,
		"auth_type":  string(p.AuthType),
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if p.AuthConfig != nil {
		props["auth_config"] = p.AuthConfig
	}
	if p.Config != nil {
		props["config"] = p.Config
	}
	if p.ID != 0 {
		props["legacy_id"] = p.ID
	}
	return props
}

// fromObject converts an Emergent GraphObject back to a db.Provider.
func fromObject(obj *graph.GraphObject) (*db.Provider, error) {
	p := &db.Provider{
		CreatedAt: obj.CreatedAt,
	}

	// legacy_id
	if v, ok := obj.Properties["legacy_id"]; ok {
		switch n := v.(type) {
		case float64:
			p.ID = int64(n)
		case json.Number:
			id, _ := n.Int64()
			p.ID = id
		case int64:
			p.ID = n
		}
	}

	if v, ok := obj.Properties["name"].(string); ok {
		p.Name = v
	}
	if v, ok := obj.Properties["type"].(string); ok {
		p.Type = db.ProviderType(v)
	}
	if v, ok := obj.Properties["service"].(string); ok {
		p.Service = v
	}
	if v, ok := obj.Properties["enabled"].(bool); ok {
		p.Enabled = v
	}
	if v, ok := obj.Properties["is_default"].(bool); ok {
		p.IsDefault = v
	}
	if v, ok := obj.Properties["auth_type"].(string); ok {
		p.AuthType = db.AuthType(v)
	}

	// auth_config — comes back as map[string]any from JSON
	if v, ok := obj.Properties["auth_config"]; ok && v != nil {
		p.AuthConfig = toMapStringAny(v)
	}
	// config
	if v, ok := obj.Properties["config"]; ok && v != nil {
		p.Config = toMapStringAny(v)
	}

	// updated_at
	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			p.UpdatedAt = t
		}
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}

	return p, nil
}

// toMapStringAny normalises an interface{} that should be map[string]any.
func toMapStringAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	// Fallback: round-trip through JSON (handles nested structs, etc.)
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// lookupByLegacyID finds the Emergent object ID for a given legacy SQLite ID.
func (s *EmergentProviderStore) lookupByLegacyID(ctx context.Context, id int64) (*graph.GraphObject, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  providerType,
		Label: legacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup by legacy_id %d: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return resp.Items[0], nil
}

// lookupByName finds the Emergent object for a given provider name.
func (s *EmergentProviderStore) lookupByName(ctx context.Context, name string) (*graph.GraphObject, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  providerType,
		Label: providerNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup by name %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return resp.Items[0], nil
}

// nextLegacyID allocates the next legacy ID by scanning existing objects.
// This is a simple max(legacy_id)+1 approach — acceptable because provider
// creation is infrequent and low-volume.
func (s *EmergentProviderStore) nextLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  providerType,
		Limit: 500, // providers are low-cardinality
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
			}
			if id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1, nil
}

// ---------------------------------------------------------------------------
// ProviderStore implementation
// ---------------------------------------------------------------------------

func (s *EmergentProviderStore) CreateProvider(p *db.Provider) (int64, error) {
	ctx := context.Background()

	// Allocate a legacy ID if one is not set.
	if p.ID == 0 {
		id, err := s.nextLegacyID(ctx)
		if err != nil {
			return 0, err
		}
		p.ID = id
	}

	// Check for auto-default: if first provider of this type.
	if !p.IsDefault {
		resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
			Type: providerType,
			PropertyFilters: []graph.PropertyFilter{
				{Path: "type", Op: "eq", Value: string(p.Type)},
				{Path: "enabled", Op: "eq", Value: true},
			},
			Limit: 1,
		})
		if err == nil && resp.Total == 0 {
			p.IsDefault = true
		}
	}

	props := toProperties(p)
	labels := []string{
		legacyIDLabel(p.ID),
		providerNameLabel(p.Name),
	}

	status := "active"
	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       providerType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent create provider: %w", err)
	}

	slog.Info("Provider created in Emergent",
		"legacy_id", p.ID, "emergent_id", obj.ID, "name", p.Name)
	return p.ID, nil
}

func (s *EmergentProviderStore) GetProvider(id int64) (*db.Provider, error) {
	ctx := context.Background()
	obj, err := s.lookupByLegacyID(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	return fromObject(obj)
}

func (s *EmergentProviderStore) GetProviderByName(name string) (*db.Provider, error) {
	ctx := context.Background()
	obj, err := s.lookupByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	return fromObject(obj)
}

func (s *EmergentProviderStore) ListProviders() ([]*db.Provider, error) {
	ctx := context.Background()
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  providerType,
		Limit: 500,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list providers: %w", err)
	}

	providers := make([]*db.Provider, 0, len(resp.Items))
	for _, obj := range resp.Items {
		p, err := fromObject(obj)
		if err != nil {
			slog.Warn("Skipping malformed provider object", "id", obj.ID, "error", err)
			continue
		}
		providers = append(providers, p)
	}

	// Sort: defaults first, then by name (match SQLite ORDER BY is_default DESC, name)
	sortProviders(providers)
	return providers, nil
}

func (s *EmergentProviderStore) ListProvidersByType(ptype db.ProviderType) ([]*db.Provider, error) {
	ctx := context.Background()
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: providerType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "type", Op: "eq", Value: string(ptype)},
		},
		Limit: 500,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list providers by type: %w", err)
	}

	providers := make([]*db.Provider, 0, len(resp.Items))
	for _, obj := range resp.Items {
		p, err := fromObject(obj)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}
	sortProviders(providers)
	return providers, nil
}

func (s *EmergentProviderStore) ListEnabledProvidersByType(ptype db.ProviderType) ([]*db.Provider, error) {
	ctx := context.Background()
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: providerType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "type", Op: "eq", Value: string(ptype)},
			{Path: "enabled", Op: "eq", Value: true},
		},
		Limit: 500,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list enabled providers by type: %w", err)
	}

	providers := make([]*db.Provider, 0, len(resp.Items))
	for _, obj := range resp.Items {
		p, err := fromObject(obj)
		if err != nil {
			continue
		}
		providers = append(providers, p)
	}
	sortProviders(providers)
	return providers, nil
}

func (s *EmergentProviderStore) GetDefaultProvider(ptype db.ProviderType) (*db.Provider, error) {
	ctx := context.Background()

	// Try explicit default first
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: providerType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "type", Op: "eq", Value: string(ptype)},
			{Path: "enabled", Op: "eq", Value: true},
			{Path: "is_default", Op: "eq", Value: true},
		},
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent get default provider: %w", err)
	}
	if len(resp.Items) > 0 {
		return fromObject(resp.Items[0])
	}

	// Fallback: first enabled provider of this type (oldest)
	resp, err = s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: providerType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "type", Op: "eq", Value: string(ptype)},
			{Path: "enabled", Op: "eq", Value: true},
		},
		Order: "asc",
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent get default provider fallback: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}
	return fromObject(resp.Items[0])
}

func (s *EmergentProviderStore) UpdateProvider(p *db.Provider) error {
	ctx := context.Background()
	obj, err := s.lookupByLegacyID(ctx, p.ID)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("provider not found: %d", p.ID)
	}

	// Determine if name changed — need to update labels
	oldName, _ := obj.Properties["name"].(string)
	labels := obj.Labels
	replaceLabels := false
	if oldName != p.Name {
		// Replace the name label
		newLabels := make([]string, 0, len(labels))
		for _, l := range labels {
			if l != providerNameLabel(oldName) {
				newLabels = append(newLabels, l)
			}
		}
		newLabels = append(newLabels, providerNameLabel(p.Name))
		labels = newLabels
		replaceLabels = true
	}

	props := toProperties(p)

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties:    props,
		Labels:        labels,
		ReplaceLabels: replaceLabels,
	})
	if err != nil {
		return fmt.Errorf("emergent update provider %d: %w", p.ID, err)
	}
	return nil
}

func (s *EmergentProviderStore) DeleteProvider(id int64) error {
	ctx := context.Background()
	obj, err := s.lookupByLegacyID(ctx, id)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil // already deleted or never existed
	}

	if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
		return fmt.Errorf("emergent delete provider %d: %w", id, err)
	}
	return nil
}

func (s *EmergentProviderStore) SetDefaultProvider(id int64) error {
	ctx := context.Background()

	// Find the target provider to get its type
	target, err := s.lookupByLegacyID(ctx, id)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("provider not found: %d", id)
	}

	ptype, _ := target.Properties["type"].(string)

	// Unset is_default on all providers of this type
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type: providerType,
		PropertyFilters: []graph.PropertyFilter{
			{Path: "type", Op: "eq", Value: ptype},
			{Path: "is_default", Op: "eq", Value: true},
		},
		Limit: 500,
	})
	if err != nil {
		return fmt.Errorf("emergent list defaults for type %s: %w", ptype, err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, obj := range resp.Items {
		if obj.ID == target.ID {
			continue
		}
		_, err := s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
			Properties: map[string]any{
				"is_default": false,
				"updated_at": now,
			},
		})
		if err != nil {
			slog.Warn("Failed to unset default on provider",
				"emergent_id", obj.ID, "error", err)
		}
	}

	// Set the target as default
	_, err = s.client.Graph.UpdateObject(ctx, target.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			"is_default": true,
			"updated_at": now,
		},
	})
	if err != nil {
		return fmt.Errorf("emergent set default provider %d: %w", id, err)
	}

	return nil
}

func (s *EmergentProviderStore) EnableProvider(id int64) error {
	return s.setBoolProperty(id, "enabled", true)
}

func (s *EmergentProviderStore) DisableProvider(id int64) error {
	return s.setBoolProperty(id, "enabled", false)
}

func (s *EmergentProviderStore) setBoolProperty(id int64, key string, val bool) error {
	ctx := context.Background()
	obj, err := s.lookupByLegacyID(ctx, id)
	if err != nil {
		return err
	}
	if obj == nil {
		return fmt.Errorf("provider not found: %d", id)
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties: map[string]any{
			key:          val,
			"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return fmt.Errorf("emergent set %s=%v on provider %d: %w", key, val, id, err)
	}
	return nil
}

// sortProviders sorts by is_default DESC, then name ASC (matching SQLite behaviour).
func sortProviders(providers []*db.Provider) {
	for i := 1; i < len(providers); i++ {
		for j := i; j > 0; j-- {
			a, b := providers[j-1], providers[j]
			// defaults first
			if !a.IsDefault && b.IsDefault {
				providers[j-1], providers[j] = b, a
			} else if a.IsDefault == b.IsDefault && a.Name > b.Name {
				providers[j-1], providers[j] = b, a
			}
		}
	}
}

// Ensure compile-time interface satisfaction.
var _ ProviderStore = (*EmergentProviderStore)(nil)
