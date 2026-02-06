// Package apple provides MCP tools for Apple Reminders and Contacts
package apple

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// --- Helper Functions (embedded from SDK) ---

// RunCommand executes a command and returns stdout
func runCommand(name string, args ...string) (string, error) {
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

// CommandExists checks if a command is available in PATH
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// GetString extracts a string argument, returns empty string if not found
func getString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

// GetStringRequired extracts a required string argument
func getStringRequired(args map[string]interface{}, key string) (string, error) {
	if val, ok := args[key].(string); ok && val != "" {
		return val, nil
	}
	return "", fmt.Errorf("missing required argument: %s", key)
}

// TextContent creates an MCP text content response
func textContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

// ObjectSchema creates a standard object inputSchema
func objectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// StringProperty creates a string property for inputSchema
func stringProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// --- Tool Definition ---

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements ToolProvider for Apple services
type Provider struct {
	swiftScriptPath string
}

// NewProvider creates a new Apple tools provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "apple"
}

// CheckDependencies verifies required binaries exist
func (p *Provider) CheckDependencies() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("Apple tools are only available on macOS")
	}

	if !commandExists("remindctl") {
		return fmt.Errorf("remindctl not found. Install with: brew install keith/formulae/remindctl")
	}

	if !commandExists("swift") {
		return fmt.Errorf("swift not found. Install Xcode Command Line Tools")
	}

	return nil
}

// SetSwiftScriptPath sets the path to the contacts Swift script
func (p *Provider) SetSwiftScriptPath(path string) {
	p.swiftScriptPath = path
}

// Tools returns all Apple tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name:        "apple_list_reminders",
			Description: "List Apple Reminders from a specific list or all lists",
			InputSchema: objectSchema(
				map[string]interface{}{
					"listName": stringProperty("The name of the reminder list (optional, lists all if omitted)"),
				},
				nil, // no required fields
			),
		},
		{
			Name:        "apple_add_reminder",
			Description: "Add a new Apple Reminder. Dates must be in YYYY-MM-DD HH:MM format.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"title":    stringProperty("The title of the reminder"),
					"listName": stringProperty("The list to add the reminder to (optional)"),
					"due":      stringProperty("Due date/time in 'YYYY-MM-DD HH:MM' format (optional)"),
				},
				[]string{"title"},
			),
		},
		{
			Name:        "apple_search_contacts",
			Description: "Search Apple Contacts by name, email, or phone number. Returns ID, name and email columns in tab-separated format.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query": stringProperty("Search query (name, email, or phone). Use empty string to list all contacts."),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "apple_list_all_contacts",
			Description: "List all contacts in your Apple Contacts. Returns ID, name and email columns.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
	}
}

// Call executes a tool by name
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "apple_list_reminders":
		return p.listReminders(args)
	case "apple_add_reminder":
		return p.addReminder(args)
	case "apple_search_contacts":
		return p.searchContacts(args)
	case "apple_list_all_contacts":
		return p.listAllContacts(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "apple_list_reminders", "apple_add_reminder", "apple_search_contacts", "apple_list_all_contacts":
		return true
	default:
		return false
	}
}

// --- Reminders Tools ---

func (p *Provider) listReminders(args map[string]interface{}) (interface{}, error) {
	listName := getString(args, "listName")

	var output string
	var err error

	if listName != "" {
		output, err = runCommand("remindctl", "list", listName, "--json")
	} else {
		output, err = runCommand("remindctl", "list", "--json")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list reminders: %w", err)
	}

	if output == "" {
		output = "No reminders found."
	}

	return textContent(output), nil
}

func (p *Provider) addReminder(args map[string]interface{}) (interface{}, error) {
	title, err := getStringRequired(args, "title")
	if err != nil {
		return nil, err
	}

	listName := getString(args, "listName")
	due := getString(args, "due")

	// Build command arguments
	cmdArgs := []string{"add", title}

	if listName != "" {
		cmdArgs = append(cmdArgs, "--list", listName)
	}

	if due != "" {
		cmdArgs = append(cmdArgs, "--due", due)
	}

	output, err := runCommand("remindctl", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to add reminder: %w", err)
	}

	return textContent(output), nil
}

// --- Contacts Tools ---

func (p *Provider) getSwiftScriptPath() string {
	if p.swiftScriptPath != "" {
		return p.swiftScriptPath
	}
	// Default path relative to the diane repo
	return filepath.Join(".opencode", "tools", "lib", "search_all_contacts.swift")
}

func (p *Provider) searchContacts(args map[string]interface{}) (interface{}, error) {
	query := getString(args, "query")

	scriptPath := p.getSwiftScriptPath()
	output, err := runCommand("swift", scriptPath, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search contacts: %w", err)
	}

	if output == "" {
		output = "No contacts found."
	}

	return textContent(output), nil
}

func (p *Provider) listAllContacts(args map[string]interface{}) (interface{}, error) {
	scriptPath := p.getSwiftScriptPath()
	output, err := runCommand("swift", scriptPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}

	if output == "" {
		output = "No contacts found."
	}

	return textContent(output), nil
}
