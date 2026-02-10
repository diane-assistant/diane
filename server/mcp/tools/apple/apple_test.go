// Package apple provides MCP tools for Apple Reminders and Contacts
package apple

import (
	"runtime"
	"testing"
)

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "existing string value",
			args:     map[string]interface{}{"name": "John"},
			key:      "name",
			expected: "John",
		},
		{
			name:     "missing key",
			args:     map[string]interface{}{"name": "John"},
			key:      "age",
			expected: "",
		},
		{
			name:     "non-string value",
			args:     map[string]interface{}{"age": 30},
			key:      "age",
			expected: "",
		},
		{
			name:     "nil map",
			args:     nil,
			key:      "name",
			expected: "",
		},
		{
			name:     "empty map",
			args:     map[string]interface{}{},
			key:      "name",
			expected: "",
		},
		{
			name:     "empty string value",
			args:     map[string]interface{}{"name": ""},
			key:      "name",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.args, tt.key)
			if result != tt.expected {
				t.Errorf("getString(%v, %q) = %q, want %q", tt.args, tt.key, result, tt.expected)
			}
		})
	}
}

func TestGetStringRequired(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]interface{}
		key         string
		expected    string
		expectError bool
	}{
		{
			name:        "existing string value",
			args:        map[string]interface{}{"id": "abc123"},
			key:         "id",
			expected:    "abc123",
			expectError: false,
		},
		{
			name:        "missing key",
			args:        map[string]interface{}{"name": "John"},
			key:         "id",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string value",
			args:        map[string]interface{}{"id": ""},
			key:         "id",
			expected:    "",
			expectError: true,
		},
		{
			name:        "non-string value",
			args:        map[string]interface{}{"id": 123},
			key:         "id",
			expected:    "",
			expectError: true,
		},
		{
			name:        "nil map",
			args:        nil,
			key:         "id",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getStringRequired(tt.args, tt.key)
			if tt.expectError {
				if err == nil {
					t.Errorf("getStringRequired(%v, %q) expected error, got nil", tt.args, tt.key)
				}
			} else {
				if err != nil {
					t.Errorf("getStringRequired(%v, %q) unexpected error: %v", tt.args, tt.key, err)
				}
				if result != tt.expected {
					t.Errorf("getStringRequired(%v, %q) = %q, want %q", tt.args, tt.key, result, tt.expected)
				}
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]interface{}
		key        string
		defaultVal int
		expected   int
	}{
		{
			name:       "existing float64 value",
			args:       map[string]interface{}{"limit": float64(100)},
			key:        "limit",
			defaultVal: 50,
			expected:   100,
		},
		{
			name:       "missing key returns default",
			args:       map[string]interface{}{"name": "John"},
			key:        "limit",
			defaultVal: 50,
			expected:   50,
		},
		{
			name:       "non-numeric value returns default",
			args:       map[string]interface{}{"limit": "100"},
			key:        "limit",
			defaultVal: 50,
			expected:   50,
		},
		{
			name:       "integer value (should fail, JSON uses float64)",
			args:       map[string]interface{}{"limit": 100},
			key:        "limit",
			defaultVal: 50,
			expected:   50, // int type won't match float64 assertion
		},
		{
			name:       "zero value",
			args:       map[string]interface{}{"limit": float64(0)},
			key:        "limit",
			defaultVal: 50,
			expected:   0,
		},
		{
			name:       "negative value",
			args:       map[string]interface{}{"limit": float64(-10)},
			key:        "limit",
			defaultVal: 50,
			expected:   -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt(tt.args, tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("getInt(%v, %q, %d) = %d, want %d", tt.args, tt.key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestTextContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "empty text",
			input:    "",
			expected: "",
		},
		{
			name:     "json text",
			input:    `{"contacts": [], "count": 0}`,
			expected: `{"contacts": [], "count": 0}`,
		},
		{
			name:     "multiline text",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textContent(tt.input)

			// Verify structure
			content, ok := result["content"].([]map[string]interface{})
			if !ok {
				t.Fatalf("textContent(%q) returned invalid structure", tt.input)
			}

			if len(content) != 1 {
				t.Fatalf("textContent(%q) returned %d content items, want 1", tt.input, len(content))
			}

			if content[0]["type"] != "text" {
				t.Errorf("textContent(%q) type = %v, want 'text'", tt.input, content[0]["type"])
			}

			if content[0]["text"] != tt.expected {
				t.Errorf("textContent(%q) text = %v, want %q", tt.input, content[0]["text"], tt.expected)
			}
		})
	}
}

func TestObjectSchema(t *testing.T) {
	t.Run("with required fields", func(t *testing.T) {
		props := map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "number"},
		}
		required := []string{"name"}

		schema := objectSchema(props, required)

		if schema["type"] != "object" {
			t.Errorf("schema type = %v, want 'object'", schema["type"])
		}

		schemaProps, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Error("schema properties should be a map")
		}
		if len(schemaProps) != len(props) {
			t.Error("schema properties don't match input")
		}

		reqSlice, ok := schema["required"].([]string)
		if !ok {
			t.Fatal("schema required is not []string")
		}
		if len(reqSlice) != 1 || reqSlice[0] != "name" {
			t.Errorf("schema required = %v, want [name]", reqSlice)
		}
	})

	t.Run("without required fields", func(t *testing.T) {
		props := map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		}

		schema := objectSchema(props, nil)

		if schema["type"] != "object" {
			t.Errorf("schema type = %v, want 'object'", schema["type"])
		}

		if _, hasRequired := schema["required"]; hasRequired {
			t.Error("schema should not have 'required' key when nil is passed")
		}
	})

	t.Run("with empty required slice", func(t *testing.T) {
		props := map[string]interface{}{}

		schema := objectSchema(props, []string{})

		if _, hasRequired := schema["required"]; hasRequired {
			t.Error("schema should not have 'required' key when empty slice is passed")
		}
	})
}

func TestStringProperty(t *testing.T) {
	desc := "The contact's email address"
	prop := stringProperty(desc)

	if prop["type"] != "string" {
		t.Errorf("stringProperty type = %v, want 'string'", prop["type"])
	}

	if prop["description"] != desc {
		t.Errorf("stringProperty description = %v, want %q", prop["description"], desc)
	}
}

func TestNumberProperty(t *testing.T) {
	desc := "Maximum number of results"
	prop := numberProperty(desc)

	if prop["type"] != "number" {
		t.Errorf("numberProperty type = %v, want 'number'", prop["type"])
	}

	if prop["description"] != desc {
		t.Errorf("numberProperty description = %v, want %q", prop["description"], desc)
	}
}

// =============================================================================
// Provider Tests
// =============================================================================

func TestProviderName(t *testing.T) {
	p := NewProvider()
	if p.Name() != "apple" {
		t.Errorf("Provider.Name() = %q, want 'apple'", p.Name())
	}
}

func TestProviderTools(t *testing.T) {
	p := NewProvider()
	tools := p.Tools()

	// Verify we have all expected tools
	expectedTools := []string{
		"apple_list_reminders",
		"apple_add_reminder",
		"apple_search_contacts",
		"apple_get_contact",
		"apple_create_contact",
		"apple_update_contact",
		"apple_delete_contact",
		"apple_list_contact_groups",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Provider.Tools() returned %d tools, want %d", len(tools), len(expectedTools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Provider.Tools() missing tool %q", expected)
		}
	}
}

func TestProviderHasTool(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		name     string
		expected bool
	}{
		{"apple_list_reminders", true},
		{"apple_add_reminder", true},
		{"apple_search_contacts", true},
		{"apple_get_contact", true},
		{"apple_create_contact", true},
		{"apple_update_contact", true},
		{"apple_delete_contact", true},
		{"apple_list_contact_groups", true},
		{"apple_nonexistent_tool", false},
		{"google_search_emails", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.HasTool(tt.name)
			if result != tt.expected {
				t.Errorf("Provider.HasTool(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestProviderCheckDependencies(t *testing.T) {
	p := NewProvider()
	err := p.CheckDependencies()

	if runtime.GOOS != "darwin" {
		// On non-macOS, should return an error
		if err == nil {
			t.Error("CheckDependencies() should return error on non-darwin platforms")
		}
	} else {
		// On macOS, should succeed (assuming swift is installed)
		// Note: This might fail if swift is not installed, which is acceptable
		if err != nil {
			t.Logf("CheckDependencies() returned error (might be expected if swift not installed): %v", err)
		}
	}
}

// =============================================================================
// Tool Schema Validation Tests
// =============================================================================

func TestToolSchemas(t *testing.T) {
	p := NewProvider()
	tools := p.Tools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			// Every tool should have a name
			if tool.Name == "" {
				t.Error("Tool has empty name")
			}

			// Every tool should have a description
			if tool.Description == "" {
				t.Errorf("Tool %q has empty description", tool.Name)
			}

			// Every tool should have an input schema
			if tool.InputSchema == nil {
				t.Errorf("Tool %q has nil input schema", tool.Name)
			}

			// Input schema should be type "object"
			if tool.InputSchema["type"] != "object" {
				t.Errorf("Tool %q input schema type = %v, want 'object'", tool.Name, tool.InputSchema["type"])
			}

			// Input schema should have properties (even if empty)
			if _, hasProps := tool.InputSchema["properties"]; !hasProps {
				t.Errorf("Tool %q input schema missing 'properties'", tool.Name)
			}
		})
	}
}

func TestSearchContactsSchema(t *testing.T) {
	p := NewProvider()
	var tool *Tool
	for _, tt := range p.Tools() {
		if tt.Name == "apple_search_contacts" {
			tool = &tt
			break
		}
	}

	if tool == nil {
		t.Fatal("apple_search_contacts tool not found")
	}

	// Check required fields
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("apple_search_contacts should have required fields")
	}

	hasQuery := false
	for _, r := range required {
		if r == "query" {
			hasQuery = true
		}
	}
	if !hasQuery {
		t.Error("apple_search_contacts should require 'query' field")
	}

	// Check properties exist
	props, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("apple_search_contacts properties should be a map")
	}

	expectedProps := []string{"query", "field", "limit"}
	for _, prop := range expectedProps {
		if _, exists := props[prop]; !exists {
			t.Errorf("apple_search_contacts missing property %q", prop)
		}
	}
}

func TestCreateContactSchema(t *testing.T) {
	p := NewProvider()
	var tool *Tool
	for _, tt := range p.Tools() {
		if tt.Name == "apple_create_contact" {
			tool = &tt
			break
		}
	}

	if tool == nil {
		t.Fatal("apple_create_contact tool not found")
	}

	// Check required fields
	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		t.Fatal("apple_create_contact should have required fields")
	}

	hasGivenName := false
	for _, r := range required {
		if r == "givenName" {
			hasGivenName = true
		}
	}
	if !hasGivenName {
		t.Error("apple_create_contact should require 'givenName' field")
	}

	// Check all expected properties exist
	props, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("apple_create_contact properties should be a map")
	}

	expectedProps := []string{
		"givenName", "familyName", "organization", "jobTitle", "department",
		"phones", "emails", "addresses", "birthday", "note", "urls",
	}
	for _, prop := range expectedProps {
		if _, exists := props[prop]; !exists {
			t.Errorf("apple_create_contact missing property %q", prop)
		}
	}
}

// =============================================================================
// Call Routing Tests
// =============================================================================

func TestCallUnknownTool(t *testing.T) {
	p := NewProvider()

	_, err := p.Call("unknown_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Call() should return error for unknown tool")
	}
}

func TestCallMissingRequiredArgs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping on non-darwin platform")
	}

	p := NewProvider()

	// Test apple_get_contact without required 'id' argument
	_, err := p.Call("apple_get_contact", map[string]interface{}{})
	if err == nil {
		t.Error("apple_get_contact should fail without 'id' argument")
	}

	// Test apple_create_contact without required 'givenName' argument
	_, err = p.Call("apple_create_contact", map[string]interface{}{})
	if err == nil {
		t.Error("apple_create_contact should fail without 'givenName' argument")
	}

	// Test apple_delete_contact without required 'id' argument
	_, err = p.Call("apple_delete_contact", map[string]interface{}{})
	if err == nil {
		t.Error("apple_delete_contact should fail without 'id' argument")
	}

	// Test apple_update_contact without required 'id' argument
	_, err = p.Call("apple_update_contact", map[string]interface{}{})
	if err == nil {
		t.Error("apple_update_contact should fail without 'id' argument")
	}

	// Test apple_add_reminder without required 'title' argument
	_, err = p.Call("apple_add_reminder", map[string]interface{}{})
	if err == nil {
		t.Error("apple_add_reminder should fail without 'title' argument")
	}
}

func TestUpdateContactNoFieldsToUpdate(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping on non-darwin platform")
	}

	p := NewProvider()

	// Test apple_update_contact with only 'id' (no fields to update)
	_, err := p.Call("apple_update_contact", map[string]interface{}{
		"id": "test-id-123",
	})
	if err == nil {
		t.Error("apple_update_contact should fail when no fields to update are provided")
	}
}

// =============================================================================
// Command Exists Tests
// =============================================================================

func TestCommandExists(t *testing.T) {
	// Test with a command that should always exist
	if !commandExists("ls") {
		t.Error("commandExists('ls') should return true")
	}

	// Test with a command that shouldn't exist
	if commandExists("this_command_definitely_does_not_exist_12345") {
		t.Error("commandExists('this_command_definitely_does_not_exist_12345') should return false")
	}
}

// =============================================================================
// Integration Tests (require macOS + Contacts permission)
// =============================================================================

// These tests are skipped by default and require:
// 1. Running on macOS (darwin)
// 2. Contacts access permission granted
// 3. Running with -short flag disabled

func TestIntegrationSearchContacts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping on non-darwin platform")
	}

	p := NewProvider()

	// Search for all contacts (empty query should return contacts)
	result, err := p.Call("apple_search_contacts", map[string]interface{}{
		"query": "",
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("apple_search_contacts failed: %v", err)
	}

	// Result should be text content with JSON
	content, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	contentArray, ok := content["content"].([]map[string]interface{})
	if !ok || len(contentArray) == 0 {
		t.Fatal("Result should have content array")
	}

	text, ok := contentArray[0]["text"].(string)
	if !ok {
		t.Fatal("Content should have text")
	}

	t.Logf("Search result: %s", text)
}

func TestIntegrationListContactGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping on non-darwin platform")
	}

	p := NewProvider()

	result, err := p.Call("apple_list_contact_groups", map[string]interface{}{})
	if err != nil {
		t.Fatalf("apple_list_contact_groups failed: %v", err)
	}

	content, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	t.Logf("Groups result: %+v", content)
}

// TestIntegrationContactCRUD tests create, read, update, delete operations
// This test creates a real contact, so run with caution
func TestIntegrationContactCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping on non-darwin platform")
	}

	// Skip by default - uncomment to run
	t.Skip("Skipping CRUD test - uncomment to run (creates/deletes real contacts)")

	p := NewProvider()

	// 1. Create a test contact
	createResult, err := p.Call("apple_create_contact", map[string]interface{}{
		"givenName":    "TestContact",
		"familyName":   "ForAutomatedTesting",
		"organization": "Test Organization",
		"phones":       `[{"label": "mobile", "value": "+1234567890"}]`,
		"emails":       `[{"label": "work", "value": "test@example.com"}]`,
	})
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	t.Logf("Create result: %+v", createResult)

	// Extract contact ID from result (would need JSON parsing in real implementation)
	// For now, just verify no error occurred

	// 2. Search for the contact
	searchResult, err := p.Call("apple_search_contacts", map[string]interface{}{
		"query": "TestContact ForAutomatedTesting",
		"field": "name",
	})
	if err != nil {
		t.Fatalf("Failed to search contacts: %v", err)
	}

	t.Logf("Search result: %+v", searchResult)

	// Note: To complete this test, you would need to:
	// - Parse the JSON to get the contact ID
	// - Use apple_get_contact to get full details
	// - Use apple_update_contact to update fields
	// - Use apple_delete_contact to clean up
}
