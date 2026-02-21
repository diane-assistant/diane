package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

const acpAgentType = "acp_agent"

// EmergentACPAgentStore implements ACPAgentStore using the Emergent graph API.
type EmergentACPAgentStore struct {
	client *sdk.Client
}

// NewEmergentACPAgentStore creates a new EmergentACPAgentStore.
func NewEmergentACPAgentStore(client *sdk.Client) *EmergentACPAgentStore {
	return &EmergentACPAgentStore{
		client: client,
	}
}

// ListAgents returns all ACP agents.
func (s *EmergentACPAgentStore) ListAgents(ctx context.Context) ([]ACPAgentConfig, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpAgentType,
		Limit: 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}

	agents := make([]ACPAgentConfig, 0, len(resp.Items))
	for _, obj := range resp.Items {
		agent, err := s.unmarshalAgent(obj)
		if err != nil {
			slog.Warn("Failed to unmarshal ACP agent from graph", "id", obj.ID, "error", err)
			continue
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// GetAgent retrieves an ACP agent by its name.
func (s *EmergentACPAgentStore) GetAgent(ctx context.Context, name string) (*ACPAgentConfig, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpAgentType,
		Label: "name:" + name,
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %q: %w", name, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil // not found
	}

	agent, err := s.unmarshalAgent(resp.Items[0])
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent %q: %w", name, err)
	}
	return &agent, nil
}

// SaveAgent creates or updates an ACP agent.
func (s *EmergentACPAgentStore) SaveAgent(ctx context.Context, agent ACPAgentConfig) error {
	// Check if exists
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpAgentType,
		Label: "name:" + agent.Name,
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("failed to check existing agent %q: %w", agent.Name, err)
	}

	labels := []string{"name:" + agent.Name}
	if agent.Enabled {
		labels = append(labels, "status:enabled")
	} else {
		labels = append(labels, "status:disabled")
	}

	props := s.marshalAgentProps(agent)

	if len(resp.Items) > 0 {
		// Update
		obj := resp.Items[0]
		_, err = s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
			Properties: props,
			Labels:     labels,
			ReplaceLabels: true,
		})
		if err != nil {
			return fmt.Errorf("failed to update agent %q: %w", agent.Name, err)
		}
	} else {
		// Create
		status := "active"
		if !agent.Enabled {
			status = "inactive"
		}
		_, err = s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
			Type:       acpAgentType,
			Status:     &status,
			Properties: props,
			Labels:     labels,
		})
		if err != nil {
			return fmt.Errorf("failed to create agent %q: %w", agent.Name, err)
		}
	}

	return nil
}

// DeleteAgent removes an ACP agent.
func (s *EmergentACPAgentStore) DeleteAgent(ctx context.Context, name string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpAgentType,
		Label: "name:" + name,
	})
	if err != nil {
		return fmt.Errorf("failed to find agent %q to delete: %w", name, err)
	}

	for _, obj := range resp.Items {
		err := s.client.Graph.DeleteObject(ctx, obj.ID)
		if err != nil {
			return fmt.Errorf("failed to delete agent %q (id: %s): %w", name, obj.ID, err)
		}
	}

	return nil
}

// EnableAgent enables or disables an ACP agent.
func (s *EmergentACPAgentStore) EnableAgent(ctx context.Context, name string, enabled bool) error {
	agent, err := s.GetAgent(ctx, name)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent %q not found", name)
	}

	agent.Enabled = enabled
	return s.SaveAgent(ctx, *agent)
}

// unmarshalAgent converts a graph object to an ACPAgentConfig.
func (s *EmergentACPAgentStore) unmarshalAgent(obj *graph.GraphObject) (ACPAgentConfig, error) {
	props := obj.Properties
	agent := ACPAgentConfig{
		Name:        getString(props, "name"),
		URL:         getString(props, "url"),
		Type:        getString(props, "type"),
		Command:     getString(props, "command"),
		WorkDir:     getString(props, "workdir"),
		SubAgent:    getString(props, "sub_agent"),
		Enabled:     getBool(props, "enabled"),
		Description: getString(props, "description"),
	}

	if port, ok := props["port"].(float64); ok {
		agent.Port = int(port)
	} else if portStr, ok := props["port"].(string); ok {
		fmt.Sscanf(portStr, "%d", &agent.Port)
	}

	if argsData, ok := props["args"].(string); ok && argsData != "" {
		_ = json.Unmarshal([]byte(argsData), &agent.Args)
	}

	if envData, ok := props["env"].(string); ok && envData != "" {
		_ = json.Unmarshal([]byte(envData), &agent.Env)
	}

	if tagsData, ok := props["tags"].(string); ok && tagsData != "" {
		_ = json.Unmarshal([]byte(tagsData), &agent.Tags)
	}

	if wcData, ok := props["workspace_config"].(string); ok && wcData != "" {
		var wc WorkspaceConfig
		if err := json.Unmarshal([]byte(wcData), &wc); err == nil {
			agent.WorkspaceConfig = &wc
		}
	}

	return agent, nil
}

// marshalAgentProps converts an ACPAgentConfig to graph object properties.
func (s *EmergentACPAgentStore) marshalAgentProps(agent ACPAgentConfig) map[string]any {
	props := map[string]any{
		"name":        agent.Name,
		"url":         agent.URL,
		"type":        agent.Type,
		"command":     agent.Command,
		"workdir":     agent.WorkDir,
		"port":        agent.Port,
		"sub_agent":   agent.SubAgent,
		"enabled":     agent.Enabled,
		"description": agent.Description,
		"updated_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}

	if len(agent.Args) > 0 {
		if b, err := json.Marshal(agent.Args); err == nil {
			props["args"] = string(b)
		}
	} else {
		props["args"] = "[]"
	}

	if len(agent.Env) > 0 {
		if b, err := json.Marshal(agent.Env); err == nil {
			props["env"] = string(b)
		}
	} else {
		props["env"] = "{}"
	}

	if len(agent.Tags) > 0 {
		if b, err := json.Marshal(agent.Tags); err == nil {
			props["tags"] = string(b)
		}
	} else {
		props["tags"] = "[]"
	}

	if agent.WorkspaceConfig != nil {
		if b, err := json.Marshal(agent.WorkspaceConfig); err == nil {
			props["workspace_config"] = string(b)
		}
	}

	return props
}

func getString(props map[string]any, key string) string {
	if val, ok := props[key].(string); ok {
		return val
	}
	return ""
}

func getBool(props map[string]any, key string) bool {
	if val, ok := props[key].(bool); ok {
		return val
	}
	return false
}
