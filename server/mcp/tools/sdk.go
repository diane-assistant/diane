// Package tools provides shared utilities for MCP tool implementations
package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolProvider is implemented by each tool module (apple, google, etc.)
type ToolProvider interface {
	// Name returns the provider name (e.g., "apple", "google")
	Name() string

	// Tools returns all tools provided by this module
	Tools() []Tool

	// Call executes a tool by name with the given arguments
	Call(name string, args map[string]interface{}) (interface{}, error)

	// CheckDependencies verifies required binaries/configs exist
	CheckDependencies() error
}

// --- MCP Prompts ---

// Prompt represents an MCP prompt template
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument defines an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a message in a prompt response
type PromptMessage struct {
	Role    string        `json:"role"` // "user" or "assistant"
	Content PromptContent `json:"content"`
}

// PromptContent represents the content of a prompt message
type PromptContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// PromptProvider is optionally implemented by providers that offer prompts
type PromptProvider interface {
	// Prompts returns all prompts provided by this module
	Prompts() []Prompt

	// GetPrompt returns a prompt with arguments substituted
	GetPrompt(name string, args map[string]string) ([]PromptMessage, error)
}

// --- MCP Resources ---

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 encoded
}

// ResourceProvider is optionally implemented by providers that offer resources
type ResourceProvider interface {
	// Resources returns all resources provided by this module
	Resources() []Resource

	// ReadResource returns the content of a resource
	ReadResource(uri string) (*ResourceContent, error)
}

// --- Response Helpers ---

// TextContent creates an MCP text content response
func TextContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

// JSONContent creates an MCP text content response from JSON data
func JSONContent(data interface{}) (map[string]interface{}, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return TextContent(string(jsonBytes)), nil
}

// ErrorResponse creates an MCP error response
func ErrorResponse(code int, message string) error {
	return &ToolError{Code: code, Message: message}
}

// ToolError represents an MCP tool error
type ToolError struct {
	Code    int
	Message string
}

func (e *ToolError) Error() string {
	return e.Message
}

// --- Argument Helpers ---

// GetString extracts a string argument, returns empty string if not found
func GetString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

// GetStringRequired extracts a required string argument
func GetStringRequired(args map[string]interface{}, key string) (string, error) {
	if val, ok := args[key].(string); ok && val != "" {
		return val, nil
	}
	return "", fmt.Errorf("missing required argument: %s", key)
}

// GetInt extracts an integer argument, returns default if not found
func GetInt(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

// GetBool extracts a boolean argument, returns default if not found
func GetBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

// --- Command Execution Helpers ---

// RunCommand executes a command and returns stdout
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("%s: %s", err, stderrStr)
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunCommandJSON executes a command and parses JSON output
func RunCommandJSON(result interface{}, name string, args ...string) error {
	output, err := RunCommand(name, args...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(output), result); err != nil {
		return fmt.Errorf("failed to parse JSON output: %w", err)
	}

	return nil
}

// CommandExists checks if a command is available in PATH
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// --- Schema Helpers ---

// StringProperty creates a string property for inputSchema
func StringProperty(description string, required bool) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// IntProperty creates an integer property for inputSchema
func IntProperty(description string, defaultVal int) map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": description,
		"default":     defaultVal,
	}
}

// BoolProperty creates a boolean property for inputSchema
func BoolProperty(description string, defaultVal bool) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
		"default":     defaultVal,
	}
}

// ObjectSchema creates a standard object inputSchema
func ObjectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
