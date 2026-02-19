package store

import (
	"context"
)

// ContextFilterAdapter wraps a ContextStore to implement mcpproxy.ContextFilter interface.
// The mcpproxy.ContextFilter interface methods do not take context.Context,
// so this adapter bridges the gap by using context.Background().
type ContextFilterAdapter struct {
	store ContextStore
}

// NewContextFilterAdapter creates a new adapter for context filtering using a ContextStore.
func NewContextFilterAdapter(store ContextStore) *ContextFilterAdapter {
	return &ContextFilterAdapter{store: store}
}

// IsToolEnabledInContext checks if a tool is enabled for a server in a context.
func (a *ContextFilterAdapter) IsToolEnabledInContext(contextName, serverName, toolName string) (bool, error) {
	return a.store.IsToolEnabledInContext(context.Background(), contextName, serverName, toolName)
}

// GetEnabledServersForContext returns server names that are enabled in a context.
func (a *ContextFilterAdapter) GetEnabledServersForContext(contextName string) ([]string, error) {
	servers, err := a.store.GetEnabledServersForContext(context.Background(), contextName)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(servers))
	for i, s := range servers {
		names[i] = s.Name
	}
	return names, nil
}

// GetDefaultContext returns the default context name.
func (a *ContextFilterAdapter) GetDefaultContext() (string, error) {
	ctx, err := a.store.GetDefaultContext(context.Background())
	if err != nil {
		return "", err
	}
	if ctx == nil {
		return "", nil
	}
	return ctx.Name, nil
}
