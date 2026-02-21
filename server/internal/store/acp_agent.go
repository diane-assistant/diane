package store

import "context"

// ACPAgentConfig represents an ACP agent in the store
type ACPAgentConfig struct {
	Name            string            `json:"name"`
	URL             string            `json:"url,omitempty"`
	Type            string            `json:"type,omitempty"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	WorkDir         string            `json:"workdir,omitempty"`
	Port            int               `json:"port,omitempty"`
	SubAgent        string            `json:"sub_agent,omitempty"`
	Enabled         bool              `json:"enabled"`
	Description     string            `json:"description,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	WorkspaceConfig *WorkspaceConfig  `json:"workspace_config,omitempty"`
}

// ACPAgentStore defines the interface for ACP agent storage operations.
type ACPAgentStore interface {
	ListAgents(ctx context.Context) ([]ACPAgentConfig, error)
	GetAgent(ctx context.Context, name string) (*ACPAgentConfig, error)
	SaveAgent(ctx context.Context, agent ACPAgentConfig) error
	DeleteAgent(ctx context.Context, name string) error
	EnableAgent(ctx context.Context, name string, enabled bool) error
}
