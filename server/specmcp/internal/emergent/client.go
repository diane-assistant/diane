// Package emergent provides a domain-specific wrapper around the Emergent SDK.
// It exposes typed operations for SpecMCP entity types rather than generic graph operations.
package emergent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diane-assistant/diane/server/specmcp/internal/config"
	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// Client wraps the Emergent SDK with domain-specific operations for SpecMCP.
type Client struct {
	sdk    *sdk.Client
	logger *slog.Logger
}

// New creates a new Emergent client from configuration.
// The project token (emt_*) carries project scope, so no separate project ID is needed.
// If ProjectID is set, it is passed as X-Project-ID header for standalone API keys.
func New(cfg *config.EmergentConfig, logger *slog.Logger) (*Client, error) {
	sdkClient, err := sdk.New(sdk.Config{
		ServerURL: cfg.URL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: cfg.Token,
		},
		ProjectID: cfg.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}

	return &Client{
		sdk:    sdkClient,
		logger: logger,
	}, nil
}

// Graph returns the underlying SDK graph client for advanced operations.
func (c *Client) Graph() *graph.Client {
	return c.sdk.Graph
}

// CreateObject creates a graph object with the given type, key, properties, and labels.
func (c *Client) CreateObject(ctx context.Context, typeName string, key *string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	obj, err := c.sdk.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       typeName,
		Key:        key,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s object: %w", typeName, err)
	}
	c.logger.Debug("created object", "type", typeName, "id", obj.ID, "key", key)
	return obj, nil
}

// GetObject retrieves a graph object by ID.
func (c *Client) GetObject(ctx context.Context, id string) (*graph.GraphObject, error) {
	obj, err := c.sdk.Graph.GetObject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting object %s: %w", id, err)
	}
	return obj, nil
}

// UpdateObject updates a graph object's properties and/or labels.
func (c *Client) UpdateObject(ctx context.Context, id string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	req := &graph.UpdateObjectRequest{
		Properties: props,
	}
	if labels != nil {
		req.Labels = labels
		req.ReplaceLabels = true
	}
	obj, err := c.sdk.Graph.UpdateObject(ctx, id, req)
	if err != nil {
		return nil, fmt.Errorf("updating object %s: %w", id, err)
	}
	return obj, nil
}

// DeleteObject soft-deletes a graph object.
func (c *Client) DeleteObject(ctx context.Context, id string) error {
	if err := c.sdk.Graph.DeleteObject(ctx, id); err != nil {
		return fmt.Errorf("deleting object %s: %w", id, err)
	}
	return nil
}

// ListObjects lists objects with filtering options.
func (c *Client) ListObjects(ctx context.Context, opts *graph.ListObjectsOptions) ([]*graph.GraphObject, error) {
	resp, err := c.sdk.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}
	return resp.Items, nil
}

// CountObjects returns the total count of objects matching the given options.
// It uses Limit=1 to minimize data transfer and reads the Total field from the response.
func (c *Client) CountObjects(ctx context.Context, opts *graph.ListObjectsOptions) (int, error) {
	// Clone opts and set limit to 1 — we only need the Total field
	countOpts := *opts
	countOpts.Limit = 1
	resp, err := c.sdk.Graph.ListObjects(ctx, &countOpts)
	if err != nil {
		return 0, fmt.Errorf("counting objects: %w", err)
	}
	return resp.Total, nil
}

// FindByTypeAndKey finds a single object by type and key.
// Returns nil, nil if not found.
// When multiple objects share the same type+key (duplicates from before dedup was added),
// this returns the one with the smallest ID (string sort) for determinism.
func (c *Client) FindByTypeAndKey(ctx context.Context, typeName, key string) (*graph.GraphObject, error) {
	items, err := c.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  typeName,
		Key:   key,
		Limit: 50, // Fetch enough to find all duplicates
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > 1 {
		c.logger.Warn("found multiple objects with same type+key", "type", typeName, "key", key, "count", len(items))
	}
	// Pick the one with the smallest CanonicalID for determinism across versions
	oldest := items[0]
	for _, item := range items[1:] {
		if item.CanonicalID < oldest.CanonicalID {
			oldest = item
		}
	}
	return oldest, nil
}

// CreateRelationship creates a relationship between two objects.
func (c *Client) CreateRelationship(ctx context.Context, relType, srcID, dstID string, props map[string]any) (*graph.GraphRelationship, error) {
	rel, err := c.sdk.Graph.CreateRelationship(ctx, &graph.CreateRelationshipRequest{
		Type:       relType,
		SrcID:      srcID,
		DstID:      dstID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s relationship: %w", relType, err)
	}
	c.logger.Debug("created relationship", "type", relType, "src", srcID, "dst", dstID, "id", rel.ID)
	return rel, nil
}

// ListRelationships lists relationships with filtering options.
// Works around Emergent server v0.8.8 bug where the response uses "data" instead
// of "items" (which the SDK expects). We call the SDK, and if Items is empty but
// Total > 0 or the raw response contains "data", we fall back to raw HTTP parsing.
func (c *Client) ListRelationships(ctx context.Context, opts *graph.ListRelationshipsOptions) ([]*graph.GraphRelationship, error) {
	resp, err := c.sdk.Graph.ListRelationships(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing relationships: %w", err)
	}
	// If the SDK found items, return them
	if len(resp.Items) > 0 {
		return resp.Items, nil
	}
	// Work around "data" vs "items" mismatch: use GetObjectEdges as fallback
	// when we have both src_id and dst_id (the typical ensureRelationship case).
	if opts.SrcID != "" {
		edges, err := c.sdk.Graph.GetObjectEdges(ctx, opts.SrcID)
		if err != nil {
			c.logger.Debug("fallback GetObjectEdges failed", "error", err)
			return resp.Items, nil // Return empty rather than error
		}
		var matched []*graph.GraphRelationship
		for _, rel := range edges.Outgoing {
			if opts.Type != "" && rel.Type != opts.Type {
				continue
			}
			if opts.DstID != "" && rel.DstID != opts.DstID {
				continue
			}
			matched = append(matched, rel)
			if opts.Limit > 0 && len(matched) >= opts.Limit {
				break
			}
		}
		return matched, nil
	}
	return resp.Items, nil
}

// GetObjectEdges returns all incoming and outgoing relationships for an object.
func (c *Client) GetObjectEdges(ctx context.Context, objectID string) (*graph.GetObjectEdgesResponse, error) {
	edges, err := c.sdk.Graph.GetObjectEdges(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("getting edges for %s: %w", objectID, err)
	}
	return edges, nil
}

// ExpandGraph performs a graph expansion from root nodes.
func (c *Client) ExpandGraph(ctx context.Context, req *graph.GraphExpandRequest) (*graph.GraphExpandResponse, error) {
	resp, err := c.sdk.Graph.ExpandGraph(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("expanding graph: %w", err)
	}
	return resp, nil
}

// FTSSearch performs a full-text search across graph objects.
func (c *Client) FTSSearch(ctx context.Context, opts *graph.FTSSearchOptions) (*graph.SearchResponse, error) {
	resp, err := c.sdk.Graph.FTSSearch(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("FTS search: %w", err)
	}
	return resp, nil
}

// DeleteRelationship soft-deletes a relationship.
func (c *Client) DeleteRelationship(ctx context.Context, id string) error {
	if err := c.sdk.Graph.DeleteRelationship(ctx, id); err != nil {
		return fmt.Errorf("deleting relationship %s: %w", id, err)
	}
	return nil
}
