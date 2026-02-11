package notifications

import (
	"testing"
)

func TestProviderName(t *testing.T) {
	p := NewProvider()
	// Note: Provider still returns "notifications" as internal name
	// The server maps this to "discord" externally
	if p.Name() != "notifications" {
		t.Errorf("expected provider name 'notifications', got %q", p.Name())
	}
}

func TestProviderTools(t *testing.T) {
	p := &Provider{discordAvailable: true, homeAssistantAvailable: false}

	tools := p.Tools()

	// Should have 4 Discord tools when Discord is available
	expectedTools := []string{
		"discord_send_notification",
		"discord_send_embed",
		"discord_send_reaction",
		"discord_send_message_with_buttons",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestProviderToolsWithHomeAssistant(t *testing.T) {
	p := &Provider{discordAvailable: false, homeAssistantAvailable: true}

	tools := p.Tools()

	// Should have 3 Home Assistant tools
	expectedTools := []string{
		"homeassistant_send_notification",
		"homeassistant_send_actionable_notification",
		"homeassistant_send_command",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}
}

func TestProviderToolsBothServices(t *testing.T) {
	p := &Provider{discordAvailable: true, homeAssistantAvailable: true}

	tools := p.Tools()

	// Should have 7 total tools (4 Discord + 3 Home Assistant)
	if len(tools) != 7 {
		t.Errorf("expected 7 tools, got %d", len(tools))
	}
}

func TestProviderNoServicesAvailable(t *testing.T) {
	p := &Provider{discordAvailable: false, homeAssistantAvailable: false}

	tools := p.Tools()

	if len(tools) != 0 {
		t.Errorf("expected 0 tools when no services available, got %d", len(tools))
	}
}

func TestHasTool(t *testing.T) {
	p := &Provider{discordAvailable: true}

	tests := []struct {
		name     string
		expected bool
	}{
		{"discord_send_notification", true},
		{"discord_send_embed", true},
		{"discord_send_reaction", true},
		{"discord_send_message_with_buttons", true},
		{"unknown_tool", false},
		{"homeassistant_send_notification", false}, // HA not enabled
	}

	for _, tt := range tests {
		if got := p.HasTool(tt.name); got != tt.expected {
			t.Errorf("HasTool(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestCallUnknownTool(t *testing.T) {
	p := &Provider{discordAvailable: true}

	_, err := p.Call("unknown_tool", nil)
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
}

func TestGetChannelIDNumeric(t *testing.T) {
	// Test that numeric channel IDs are returned directly
	channelID, err := getChannelID("1234567890123456789")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if channelID != "1234567890123456789" {
		t.Errorf("expected '1234567890123456789', got %q", channelID)
	}
}

func TestGetString(t *testing.T) {
	args := map[string]interface{}{
		"message": "hello",
		"number":  123,
		"empty":   "",
	}

	if got := getString(args, "message"); got != "hello" {
		t.Errorf("getString('message') = %q, want 'hello'", got)
	}

	if got := getString(args, "number"); got != "" {
		t.Errorf("getString('number') = %q, want ''", got)
	}

	if got := getString(args, "missing"); got != "" {
		t.Errorf("getString('missing') = %q, want ''", got)
	}

	if got := getString(args, "empty"); got != "" {
		t.Errorf("getString('empty') = %q, want ''", got)
	}
}

func TestGetStringRequired(t *testing.T) {
	args := map[string]interface{}{
		"message": "hello",
		"empty":   "",
	}

	val, err := getStringRequired(args, "message")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("getStringRequired('message') = %q, want 'hello'", val)
	}

	_, err = getStringRequired(args, "missing")
	if err == nil {
		t.Error("expected error for missing key")
	}

	_, err = getStringRequired(args, "empty")
	if err == nil {
		t.Error("expected error for empty value")
	}
}

func TestTextContent(t *testing.T) {
	result := textContent("hello world")

	content, ok := result["content"].([]map[string]interface{})
	if !ok || len(content) != 1 {
		t.Fatal("expected content array with 1 element")
	}

	if content[0]["type"] != "text" {
		t.Errorf("expected type 'text', got %v", content[0]["type"])
	}
	if content[0]["text"] != "hello world" {
		t.Errorf("expected text 'hello world', got %v", content[0]["text"])
	}
}

func TestObjectSchema(t *testing.T) {
	schema := objectSchema(
		map[string]interface{}{
			"name": stringProperty("The name"),
		},
		[]string{"name"},
	)

	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}

	required, ok := schema["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "name" {
		t.Errorf("expected required ['name'], got %v", schema["required"])
	}
}

func TestToolInputSchemas(t *testing.T) {
	p := &Provider{discordAvailable: true}

	for _, tool := range p.Tools() {
		if tool.InputSchema == nil {
			t.Errorf("tool %s has nil InputSchema", tool.Name)
			continue
		}

		schemaType, ok := tool.InputSchema["type"]
		if !ok || schemaType != "object" {
			t.Errorf("tool %s InputSchema type should be 'object', got %v", tool.Name, schemaType)
		}

		_, ok = tool.InputSchema["properties"]
		if !ok {
			t.Errorf("tool %s InputSchema missing 'properties'", tool.Name)
		}
	}
}

// --- Prompt Tests ---

func TestPrompts(t *testing.T) {
	p := &Provider{discordAvailable: true}

	prompts := p.Prompts()

	expectedPrompts := []string{
		"discord_notify_job_result",
		"discord_daily_summary",
		"discord_ask_confirmation",
	}

	if len(prompts) != len(expectedPrompts) {
		t.Errorf("expected %d prompts, got %d", len(expectedPrompts), len(prompts))
	}

	promptNames := make(map[string]bool)
	for _, prompt := range prompts {
		promptNames[prompt.Name] = true
	}

	for _, name := range expectedPrompts {
		if !promptNames[name] {
			t.Errorf("missing expected prompt: %s", name)
		}
	}
}

func TestPromptsNoDiscord(t *testing.T) {
	p := &Provider{discordAvailable: false}

	prompts := p.Prompts()

	if len(prompts) != 0 {
		t.Errorf("expected 0 prompts when Discord not available, got %d", len(prompts))
	}
}

func TestGetPrompt(t *testing.T) {
	p := &Provider{discordAvailable: true}

	tests := []struct {
		name      string
		args      map[string]string
		expectErr bool
	}{
		{
			name:      "discord_notify_job_result",
			args:      map[string]string{"job_name": "test-job", "success": "true"},
			expectErr: false,
		},
		{
			name:      "discord_daily_summary",
			args:      map[string]string{"title": "Daily Report"},
			expectErr: false,
		},
		{
			name:      "discord_ask_confirmation",
			args:      map[string]string{"question": "Continue?"},
			expectErr: false,
		},
		{
			name:      "unknown_prompt",
			args:      nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		messages, err := p.GetPrompt(tt.name, tt.args)
		if tt.expectErr {
			if err == nil {
				t.Errorf("GetPrompt(%q) expected error, got nil", tt.name)
			}
		} else {
			if err != nil {
				t.Errorf("GetPrompt(%q) unexpected error: %v", tt.name, err)
			}
			if len(messages) == 0 {
				t.Errorf("GetPrompt(%q) returned empty messages", tt.name)
			}
			// Verify message structure
			for _, msg := range messages {
				if msg.Role != "user" && msg.Role != "assistant" {
					t.Errorf("GetPrompt(%q) message has invalid role: %q", tt.name, msg.Role)
				}
				if msg.Content.Type != "text" {
					t.Errorf("GetPrompt(%q) message has invalid content type: %q", tt.name, msg.Content.Type)
				}
				if msg.Content.Text == "" {
					t.Errorf("GetPrompt(%q) message has empty text", tt.name)
				}
			}
		}
	}
}

func TestGetArgOrDefault(t *testing.T) {
	args := map[string]string{
		"present": "value",
		"empty":   "",
	}

	if got := getArgOrDefault(args, "present", "default"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}

	if got := getArgOrDefault(args, "missing", "default"); got != "default" {
		t.Errorf("expected 'default', got %q", got)
	}

	if got := getArgOrDefault(args, "empty", "default"); got != "default" {
		t.Errorf("expected 'default' for empty value, got %q", got)
	}
}

func TestPromptJobResultContent(t *testing.T) {
	p := &Provider{discordAvailable: true}

	// Test success case
	messages, _ := p.GetPrompt("discord_notify_job_result", map[string]string{
		"job_name": "backup-db",
		"success":  "true",
		"output":   "Backup completed",
	})

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	text := messages[0].Content.Text
	if !contains(text, "backup-db") {
		t.Error("prompt should contain job name")
	}
	if !contains(text, "success") {
		t.Error("prompt should mention success color for successful job")
	}

	// Test failure case
	messages, _ = p.GetPrompt("discord_notify_job_result", map[string]string{
		"job_name": "sync-data",
		"success":  "false",
	})

	text = messages[0].Content.Text
	if !contains(text, "error") {
		t.Error("prompt should mention error color for failed job")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
