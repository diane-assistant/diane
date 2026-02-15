package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool is the interface that all SpecMCP tools must implement.
type Tool interface {
	// Name returns the tool name (e.g. "spec_new", "spec_get_context").
	Name() string

	// Description returns a human-readable description of what the tool does.
	Description() string

	// InputSchema returns the JSON Schema for the tool's parameters.
	InputSchema() json.RawMessage

	// Execute runs the tool with the given parameters and returns the result.
	Execute(ctx context.Context, params json.RawMessage) (*ToolsCallResult, error)
}

// Prompt is the interface for MCP prompts.
type Prompt interface {
	// Definition returns the prompt metadata (name, description, arguments).
	Definition() PromptDefinition

	// Get returns the prompt messages, optionally customized by arguments.
	Get(arguments map[string]string) (*PromptsGetResult, error)
}

// Resource is the interface for MCP resources.
type Resource interface {
	// Definition returns the resource metadata (URI, name, description, mimeType).
	Definition() ResourceDefinition

	// Read returns the resource content.
	Read() (*ResourcesReadResult, error)
}

// Registry holds all registered tools, prompts, and resources.
type Registry struct {
	mu            sync.RWMutex
	tools         map[string]Tool
	toolOrder     []string
	prompts       map[string]Prompt
	promptOrder   []string
	resources     map[string]Resource // keyed by URI
	resourceOrder []string
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:     make(map[string]Tool),
		prompts:   make(map[string]Prompt),
		resources: make(map[string]Resource),
	}
}

// --- Tools ---

// Register adds a tool to the registry.
// Panics if a tool with the same name is already registered.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tool %q already registered", name))
	}
	r.tools[name] = t
	r.toolOrder = append(r.toolOrder, name)
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tool definitions in registration order.
func (r *Registry) List() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(r.toolOrder))
	for _, name := range r.toolOrder {
		t := r.tools[name]
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// --- Prompts ---

// RegisterPrompt adds a prompt to the registry.
// Panics if a prompt with the same name is already registered.
func (r *Registry) RegisterPrompt(p Prompt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Definition().Name
	if _, exists := r.prompts[name]; exists {
		panic(fmt.Sprintf("prompt %q already registered", name))
	}
	r.prompts[name] = p
	r.promptOrder = append(r.promptOrder, name)
}

// GetPrompt returns a prompt by name, or nil if not found.
func (r *Registry) GetPrompt(name string) Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prompts[name]
}

// ListPrompts returns all registered prompt definitions in registration order.
func (r *Registry) ListPrompts() []PromptDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]PromptDefinition, 0, len(r.promptOrder))
	for _, name := range r.promptOrder {
		defs = append(defs, r.prompts[name].Definition())
	}
	return defs
}

// HasPrompts returns true if any prompts are registered.
func (r *Registry) HasPrompts() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.prompts) > 0
}

// --- Resources ---

// RegisterResource adds a resource to the registry.
// Panics if a resource with the same URI is already registered.
func (r *Registry) RegisterResource(res Resource) {
	r.mu.Lock()
	defer r.mu.Unlock()

	uri := res.Definition().URI
	if _, exists := r.resources[uri]; exists {
		panic(fmt.Sprintf("resource %q already registered", uri))
	}
	r.resources[uri] = res
	r.resourceOrder = append(r.resourceOrder, uri)
}

// GetResource returns a resource by URI, or nil if not found.
func (r *Registry) GetResource(uri string) Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resources[uri]
}

// ListResources returns all registered resource definitions in registration order.
func (r *Registry) ListResources() []ResourceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ResourceDefinition, 0, len(r.resourceOrder))
	for _, uri := range r.resourceOrder {
		defs = append(defs, r.resources[uri].Definition())
	}
	return defs
}

// HasResources returns true if any resources are registered.
func (r *Registry) HasResources() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.resources) > 0
}
