// Package google provides MCP tools for Google services (Gmail, Drive, Sheets, Calendar)
package google

import (
	"encoding/json"
	"strings"
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

func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected bool
	}{
		{
			name:     "existing true value",
			args:     map[string]interface{}{"enabled": true},
			key:      "enabled",
			expected: true,
		},
		{
			name:     "existing false value",
			args:     map[string]interface{}{"enabled": false},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "missing key returns false",
			args:     map[string]interface{}{"name": "John"},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "non-bool value returns false",
			args:     map[string]interface{}{"enabled": "true"},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "nil map returns false",
			args:     nil,
			key:      "enabled",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBool(tt.args, tt.key)
			if result != tt.expected {
				t.Errorf("getBool(%v, %q) = %v, want %v", tt.args, tt.key, result, tt.expected)
			}
		})
	}
}

func TestTextContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple text",
			input: "Hello, World!",
		},
		{
			name:  "JSON content",
			input: `{"key": "value"}`,
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "multiline text",
			input: "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textContent(tt.input)

			// Verify structure
			content, ok := result["content"].([]map[string]interface{})
			if !ok {
				t.Fatalf("textContent() content is not []map[string]interface{}")
			}
			if len(content) != 1 {
				t.Fatalf("textContent() content length = %d, want 1", len(content))
			}

			// Verify type
			typeVal, ok := content[0]["type"].(string)
			if !ok || typeVal != "text" {
				t.Errorf("textContent() type = %v, want 'text'", content[0]["type"])
			}

			// Verify text
			textVal, ok := content[0]["text"].(string)
			if !ok || textVal != tt.input {
				t.Errorf("textContent() text = %q, want %q", textVal, tt.input)
			}
		})
	}
}

// =============================================================================
// Provider Tests
// =============================================================================

func TestProviderName(t *testing.T) {
	p := NewProvider()
	if name := p.Name(); name != "google" {
		t.Errorf("Provider.Name() = %q, want %q", name, "google")
	}
}

func TestCheckDependencies(t *testing.T) {
	p := NewProvider()
	// After migration, no external dependencies are required
	if err := p.CheckDependencies(); err != nil {
		t.Errorf("Provider.CheckDependencies() unexpected error: %v", err)
	}
}

func TestToolsNotEmpty(t *testing.T) {
	p := NewProvider()
	tools := p.Tools()
	if len(tools) == 0 {
		t.Error("Provider.Tools() returned empty list")
	}
}

func TestHasTool(t *testing.T) {
	p := NewProvider()

	// Test known tools exist
	knownTools := []string{
		// Gmail
		"gmail_search",
		"gmail_read",
		"gmail_batch_get_messages",
		"gmail_list_labels",
		// Drive
		"drive_search",
		"drive_list",
		// Sheets
		"sheets_get",
		"sheets_update",
		"sheets_append",
		"sheets_clear",
		"sheets_get_metadata",
		// Calendar
		"calendar_list",
		"calendar_list_events",
		"calendar_get_event",
		"calendar_create_event",
		"calendar_update_event",
		"calendar_delete_event",
		"calendar_check_freebusy",
	}

	for _, tool := range knownTools {
		t.Run(tool, func(t *testing.T) {
			if !p.HasTool(tool) {
				t.Errorf("Provider.HasTool(%q) = false, want true", tool)
			}
		})
	}

	// Test unknown tools don't exist
	unknownTools := []string{
		"unknown_tool",
		"google_unknown",
		"",
	}

	for _, tool := range unknownTools {
		t.Run("unknown_"+tool, func(t *testing.T) {
			if p.HasTool(tool) {
				t.Errorf("Provider.HasTool(%q) = true, want false", tool)
			}
		})
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
			// Verify tool has a name
			if tool.Name == "" {
				t.Error("Tool has empty name")
			}

			// Verify tool has a description
			if tool.Description == "" {
				t.Errorf("Tool %q has empty description", tool.Name)
			}

			// Verify input schema exists
			if tool.InputSchema == nil {
				t.Errorf("Tool %q has nil InputSchema", tool.Name)
				return
			}

			// Verify schema type is object
			schemaType, ok := tool.InputSchema["type"].(string)
			if !ok || schemaType != "object" {
				t.Errorf("Tool %q InputSchema type = %v, want 'object'", tool.Name, tool.InputSchema["type"])
			}

			// Verify properties exist (may be empty for some tools)
			_, ok = tool.InputSchema["properties"].(map[string]interface{})
			if !ok {
				t.Errorf("Tool %q InputSchema missing properties", tool.Name)
			}
		})
	}
}

// =============================================================================
// Call Routing Tests
// =============================================================================

func TestCallUnknownTool(t *testing.T) {
	p := NewProvider()
	_, err := p.Call("unknown_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Provider.Call() expected error for unknown tool, got nil")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("Provider.Call() error = %q, want to contain 'unknown tool'", err.Error())
	}
}

func TestCallMissingRequiredArgs(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		tool     string
		args     map[string]interface{}
		errorMsg string
	}{
		{
			tool:     "gmail_search",
			args:     map[string]interface{}{},
			errorMsg: "query",
		},
		{
			tool:     "gmail_read",
			args:     map[string]interface{}{},
			errorMsg: "id",
		},
		{
			tool:     "sheets_get",
			args:     map[string]interface{}{},
			errorMsg: "sheetId",
		},
		{
			tool:     "sheets_get",
			args:     map[string]interface{}{"sheetId": "abc"},
			errorMsg: "range",
		},
		{
			tool:     "calendar_create_event",
			args:     map[string]interface{}{},
			errorMsg: "calendar_id",
		},
		{
			tool:     "calendar_get_event",
			args:     map[string]interface{}{"calendar_id": "primary"},
			errorMsg: "event_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			_, err := p.Call(tt.tool, tt.args)
			if err == nil {
				t.Errorf("Provider.Call(%q, %v) expected error, got nil", tt.tool, tt.args)
				return
			}
			if !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Provider.Call(%q) error = %q, want to contain %q", tt.tool, err.Error(), tt.errorMsg)
			}
		})
	}
}

// =============================================================================
// parseReminderDuration Tests
// =============================================================================

func TestParseReminderDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"30m", 30},
		{"1h", 60},
		{"2h", 120},
		{"1d", 1440},
		{"7d", 10080},
		{"1w", 10080},
		{"0m", 0},
		{"", 0},
		{"invalid", 0},
		{"15", 15}, // defaults to minutes
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseReminderDuration(tt.input)
			if result != tt.expected {
				t.Errorf("parseReminderDuration(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// JSON Schema Helper Tests
// =============================================================================

func TestObjectSchema(t *testing.T) {
	t.Run("with required fields", func(t *testing.T) {
		props := map[string]interface{}{
			"name": stringProperty("The name"),
			"age":  numberProperty("The age"),
		}
		required := []string{"name"}

		schema := objectSchema(props, required)

		if schema["type"] != "object" {
			t.Errorf("objectSchema type = %v, want 'object'", schema["type"])
		}

		reqList, ok := schema["required"].([]string)
		if !ok {
			t.Fatal("objectSchema required is not []string")
		}
		if len(reqList) != 1 || reqList[0] != "name" {
			t.Errorf("objectSchema required = %v, want ['name']", reqList)
		}
	})

	t.Run("without required fields", func(t *testing.T) {
		props := map[string]interface{}{
			"name": stringProperty("The name"),
		}

		schema := objectSchema(props, nil)

		if schema["type"] != "object" {
			t.Errorf("objectSchema type = %v, want 'object'", schema["type"])
		}

		if _, ok := schema["required"]; ok {
			t.Error("objectSchema should not have 'required' when nil is passed")
		}
	})
}

func TestStringProperty(t *testing.T) {
	prop := stringProperty("Test description")

	if prop["type"] != "string" {
		t.Errorf("stringProperty type = %v, want 'string'", prop["type"])
	}
	if prop["description"] != "Test description" {
		t.Errorf("stringProperty description = %v, want 'Test description'", prop["description"])
	}
}

func TestNumberProperty(t *testing.T) {
	prop := numberProperty("Test description")

	if prop["type"] != "number" {
		t.Errorf("numberProperty type = %v, want 'number'", prop["type"])
	}
	if prop["description"] != "Test description" {
		t.Errorf("numberProperty description = %v, want 'Test description'", prop["description"])
	}
}

func TestBoolProperty(t *testing.T) {
	prop := boolProperty("Test description")

	if prop["type"] != "boolean" {
		t.Errorf("boolProperty type = %v, want 'boolean'", prop["type"])
	}
	if prop["description"] != "Test description" {
		t.Errorf("boolProperty description = %v, want 'Test description'", prop["description"])
	}
}

// =============================================================================
// Tool Count Tests
// =============================================================================

func TestToolCount(t *testing.T) {
	p := NewProvider()
	tools := p.Tools()

	// We expect specific tools for each service
	expectedMinimum := 20 // Gmail + Drive + Sheets + Calendar

	if len(tools) < expectedMinimum {
		t.Errorf("Provider.Tools() returned %d tools, expected at least %d", len(tools), expectedMinimum)
	}

	// Count tools by category
	gmailCount := 0
	driveCount := 0
	sheetsCount := 0
	calendarCount := 0

	for _, tool := range tools {
		switch {
		case strings.HasPrefix(tool.Name, "gmail_"):
			gmailCount++
		case strings.HasPrefix(tool.Name, "drive_"):
			driveCount++
		case strings.HasPrefix(tool.Name, "sheets_"):
			sheetsCount++
		case strings.HasPrefix(tool.Name, "calendar_"):
			calendarCount++
		}
	}

	t.Logf("Tool counts - Gmail: %d, Drive: %d, Sheets: %d, Calendar: %d", gmailCount, driveCount, sheetsCount, calendarCount)

	if gmailCount < 10 {
		t.Errorf("Expected at least 10 Gmail tools, got %d", gmailCount)
	}
	if driveCount < 2 {
		t.Errorf("Expected at least 2 Drive tools, got %d", driveCount)
	}
	if sheetsCount < 5 {
		t.Errorf("Expected at least 5 Sheets tools, got %d", sheetsCount)
	}
	if calendarCount < 7 {
		t.Errorf("Expected at least 7 Calendar tools, got %d", calendarCount)
	}
}

// =============================================================================
// Integration Tests (require actual Google credentials)
// =============================================================================

func TestIntegrationSearchEmails(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Integration test for Gmail search - requires actual Google credentials")
	// To run: go test -v -run TestIntegrationSearchEmails
}

func TestIntegrationListCalendars(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Integration test for Calendar list - requires actual Google credentials")
	// To run: go test -v -run TestIntegrationListCalendars
}

func TestIntegrationSearchDrive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Integration test for Drive search - requires actual Google credentials")
	// To run: go test -v -run TestIntegrationSearchDrive
}

func TestIntegrationGetSheetMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Integration test for Sheets metadata - requires actual Google credentials")
	// To run: go test -v -run TestIntegrationGetSheetMetadata
}

// =============================================================================
// JSON Marshaling Tests
// =============================================================================

func TestToolsJSONMarshal(t *testing.T) {
	p := NewProvider()
	tools := p.Tools()

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			data, err := json.Marshal(tool)
			if err != nil {
				t.Errorf("json.Marshal(tool) error: %v", err)
				return
			}
			if len(data) == 0 {
				t.Error("json.Marshal(tool) returned empty data")
			}

			// Verify it can be unmarshaled back
			var unmarshaled Tool
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("json.Unmarshal() error: %v", err)
			}
			if unmarshaled.Name != tool.Name {
				t.Errorf("Unmarshaled name = %q, want %q", unmarshaled.Name, tool.Name)
			}
		})
	}
}
