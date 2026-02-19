package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"

	"github.com/diane-assistant/diane/internal/db"
)

// EmergentAgentStore implements AgentStore against the Emergent graph API.
//
// Mapping:
//
//	Agent:
//	  - Graph object type: "agent"
//	  - ID        -> properties.legacy_id + label "legacy_id:{id}"
//	  - Name      -> properties.name + label "name:{name}"
//	  - Type      -> properties.type
//	  - URL       -> properties.url
//	  - Enabled   -> properties.enabled (bool)
//	  - CreatedAt -> properties.created_at (RFC3339Nano)
//	  - UpdatedAt -> properties.updated_at (RFC3339Nano)
//
//	AgentLog:
//	  - Graph object type: "agent_log"
//	  - ID          -> properties.legacy_id + label "legacy_id:{id}"
//	  - AgentName   -> properties.agent_name + label "agent_name:{name}"
//	  - Direction   -> properties.direction
//	  - MessageType -> properties.message_type
//	  - Content     -> properties.content (nullable)
//	  - Error       -> properties.error (nullable)
//	  - DurationMs  -> properties.duration_ms (nullable)
//	  - CreatedAt   -> properties.created_at (RFC3339Nano)
type EmergentAgentStore struct {
	client *sdk.Client
}

const (
	agentType    = "agent"
	agentLogType = "agent_log"
)

// NewEmergentAgentStore creates a new Emergent-backed AgentStore.
func NewEmergentAgentStore(client *sdk.Client) *EmergentAgentStore {
	return &EmergentAgentStore{client: client}
}

// ---------------------------------------------------------------------------
// Label helpers
// ---------------------------------------------------------------------------

func agentLegacyIDLabel(id int64) string        { return fmt.Sprintf("legacy_id:%d", id) }
func agentNameLabel(name string) string         { return fmt.Sprintf("name:%s", name) }
func agentLogAgentNameLabel(name string) string { return fmt.Sprintf("agent_name:%s", name) }

// ---------------------------------------------------------------------------
// Conversion helpers — Agent
// ---------------------------------------------------------------------------

func agentToProperties(a *db.Agent) map[string]any {
	now := time.Now().UTC()
	props := map[string]any{
		"name":       a.Name,
		"type":       a.Type,
		"url":        a.URL,
		"enabled":    a.Enabled,
		"updated_at": now.Format(time.RFC3339Nano),
	}
	if a.ID != 0 {
		props["legacy_id"] = a.ID
	}
	if !a.CreatedAt.IsZero() {
		props["created_at"] = a.CreatedAt.Format(time.RFC3339Nano)
	} else {
		props["created_at"] = now.Format(time.RFC3339Nano)
	}
	return props
}

func agentFromObject(obj *graph.GraphObject) (*db.Agent, error) {
	a := &db.Agent{}

	if v, ok := obj.Properties["legacy_id"]; ok {
		a.ID = toInt64(v)
	}
	if v, ok := obj.Properties["name"].(string); ok {
		a.Name = v
	}
	if v, ok := obj.Properties["type"].(string); ok {
		a.Type = v
	}
	if v, ok := obj.Properties["url"].(string); ok {
		a.URL = v
	}
	if v, ok := obj.Properties["enabled"].(bool); ok {
		a.Enabled = v
	}
	if v, ok := obj.Properties["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			a.CreatedAt = t
		}
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = obj.CreatedAt
	}
	if v, ok := obj.Properties["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			a.UpdatedAt = t
		}
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = a.CreatedAt
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// Conversion helpers — AgentLog
// ---------------------------------------------------------------------------

func agentLogToProperties(l *db.AgentLog) map[string]any {
	now := time.Now().UTC()
	props := map[string]any{
		"legacy_id":    l.ID,
		"agent_name":   l.AgentName,
		"direction":    l.Direction,
		"message_type": l.MessageType,
		"created_at":   now.Format(time.RFC3339Nano),
	}
	if l.Content != nil {
		props["content"] = *l.Content
	}
	if l.Error != nil {
		props["error"] = *l.Error
	}
	if l.DurationMs != nil {
		props["duration_ms"] = *l.DurationMs
	}
	return props
}

func agentLogFromObject(obj *graph.GraphObject) (*db.AgentLog, error) {
	l := &db.AgentLog{}

	if v, ok := obj.Properties["legacy_id"]; ok {
		l.ID = toInt64(v)
	}
	if v, ok := obj.Properties["agent_name"].(string); ok {
		l.AgentName = v
	}
	if v, ok := obj.Properties["direction"].(string); ok {
		l.Direction = v
	}
	if v, ok := obj.Properties["message_type"].(string); ok {
		l.MessageType = v
	}
	if v, ok := obj.Properties["content"].(string); ok {
		l.Content = &v
	}
	if v, ok := obj.Properties["error"].(string); ok {
		l.Error = &v
	}
	if v, ok := obj.Properties["duration_ms"]; ok && v != nil {
		ms := int(toInt64(v))
		l.DurationMs = &ms
	}
	if v, ok := obj.Properties["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			l.CreatedAt = t
		}
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = obj.CreatedAt
	}
	return l, nil
}

// nextAgentLegacyID allocates the next legacy ID for agents.
func (s *EmergentAgentStore) nextAgentLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list for next agent ID: %w", err)
	}

	var maxID int64
	for _, obj := range resp.Items {
		if id := toInt64(obj.Properties["legacy_id"]); id > maxID {
			maxID = id
		}
	}
	return maxID + 1, nil
}

// nextAgentLogLegacyID allocates the next legacy ID for agent logs.
func (s *EmergentAgentStore) nextAgentLogLegacyID(ctx context.Context) (int64, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentLogType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list for next agent log ID: %w", err)
	}

	var maxID int64
	for _, obj := range resp.Items {
		if id := toInt64(obj.Properties["legacy_id"]); id > maxID {
			maxID = id
		}
	}
	return maxID + 1, nil
}

// ---------------------------------------------------------------------------
// Agent CRUD
// ---------------------------------------------------------------------------

func (s *EmergentAgentStore) CreateAgent(ctx context.Context, name, url, agentType_ string) (*db.Agent, error) {
	id, err := s.nextAgentLegacyID(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	a := &db.Agent{
		ID:        id,
		Name:      name,
		Type:      agentType_,
		URL:       url,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	props := agentToProperties(a)
	labels := []string{
		agentLegacyIDLabel(id),
		agentNameLabel(name),
	}
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       agentType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent create agent: %w", err)
	}

	slog.Info("emergent: created agent", "name", name, "legacy_id", id, "object_id", obj.ID)
	return a, nil
}

func (s *EmergentAgentStore) GetAgent(ctx context.Context, id int64) (*db.Agent, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Label: agentLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup agent by id %d: %w", id, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("agent not found: id=%d", id)
	}
	return agentFromObject(resp.Items[0])
}

func (s *EmergentAgentStore) GetAgentByName(ctx context.Context, name string) (*db.Agent, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Label: agentNameLabel(name),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup agent by name %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("agent not found: name=%s", name)
	}
	return agentFromObject(resp.Items[0])
}

func (s *EmergentAgentStore) ListAgents(ctx context.Context) ([]*db.Agent, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list agents: %w", err)
	}

	agents := make([]*db.Agent, 0, len(resp.Items))
	for _, obj := range resp.Items {
		a, err := agentFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed agent", "object_id", obj.ID, "error", err)
			continue
		}
		agents = append(agents, a)
	}

	sort.Slice(agents, func(i, k int) bool { return agents[i].Name < agents[k].Name })
	return agents, nil
}

func (s *EmergentAgentStore) DeleteAgent(ctx context.Context, id int64) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Label: agentLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup agent for delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, resp.Items[0].ID)
	if err != nil {
		return fmt.Errorf("emergent delete agent: %w", err)
	}

	slog.Info("emergent: deleted agent", "legacy_id", id, "object_id", resp.Items[0].ID)
	return nil
}

func (s *EmergentAgentStore) ToggleAgent(ctx context.Context, id int64, enabled bool) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentType,
		Label: agentLegacyIDLabel(id),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup agent for toggle: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("agent not found: id=%d", id)
	}

	obj := resp.Items[0]

	// Parse current agent, update enabled, rebuild labels
	a, err := agentFromObject(obj)
	if err != nil {
		return err
	}
	a.Enabled = enabled
	a.UpdatedAt = time.Now().UTC()

	props := agentToProperties(a)
	labels := []string{
		agentLegacyIDLabel(id),
		agentNameLabel(a.Name),
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
		Properties:    props,
		Labels:        labels,
		ReplaceLabels: true,
	})
	if err != nil {
		return fmt.Errorf("emergent toggle agent: %w", err)
	}

	slog.Info("emergent: toggled agent", "name", a.Name, "legacy_id", id, "enabled", enabled)
	return nil
}

// ---------------------------------------------------------------------------
// Agent log operations
// ---------------------------------------------------------------------------

func (s *EmergentAgentStore) CreateAgentLog(ctx context.Context, agentName, direction, messageType string, content, errMsg *string, durationMs *int) (*db.AgentLog, error) {
	id, err := s.nextAgentLogLegacyID(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	l := &db.AgentLog{
		ID:          id,
		AgentName:   agentName,
		Direction:   direction,
		MessageType: messageType,
		Content:     content,
		Error:       errMsg,
		DurationMs:  durationMs,
		CreatedAt:   now,
	}

	props := agentLogToProperties(l)
	props["legacy_id"] = id
	labels := []string{
		fmt.Sprintf("legacy_id:%d", id),
		agentLogAgentNameLabel(agentName),
	}
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       agentLogType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent create agent log: %w", err)
	}

	slog.Debug("emergent: created agent log", "agent", agentName, "legacy_id", id, "object_id", obj.ID)
	return l, nil
}

func (s *EmergentAgentStore) ListAgentLogs(ctx context.Context, agentName *string, limit, offset int) ([]*db.AgentLog, error) {
	opts := &graph.ListObjectsOptions{
		Type:  agentLogType,
		Limit: 1000,
	}
	if agentName != nil && *agentName != "" {
		opts.Label = agentLogAgentNameLabel(*agentName)
	}

	resp, err := s.client.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("emergent list agent logs: %w", err)
	}

	logs := make([]*db.AgentLog, 0, len(resp.Items))
	for _, obj := range resp.Items {
		l, err := agentLogFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed agent log", "object_id", obj.ID, "error", err)
			continue
		}
		logs = append(logs, l)
	}

	// Sort by created_at descending (newest first)
	sort.Slice(logs, func(i, k int) bool { return logs[i].CreatedAt.After(logs[k].CreatedAt) })

	// Apply offset and limit
	if offset >= len(logs) {
		return []*db.AgentLog{}, nil
	}
	logs = logs[offset:]
	if limit > 0 && limit < len(logs) {
		logs = logs[:limit]
	}

	return logs, nil
}

func (s *EmergentAgentStore) DeleteOldAgentLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)

	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  agentLogType,
		Limit: 1000,
	})
	if err != nil {
		return 0, fmt.Errorf("emergent list agent logs for cleanup: %w", err)
	}

	var deleted int64
	for _, obj := range resp.Items {
		var createdAt time.Time
		if v, ok := obj.Properties["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				createdAt = t
			}
		}
		if createdAt.IsZero() {
			createdAt = obj.CreatedAt
		}

		if createdAt.Before(cutoff) {
			if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
				slog.Warn("failed to delete old agent log", "object_id", obj.ID, "error", err)
				continue
			}
			deleted++
		}
	}

	slog.Info("emergent: deleted old agent logs", "count", deleted, "older_than", olderThan)
	return deleted, nil
}
