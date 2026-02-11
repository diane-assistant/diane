package jobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/diane-assistant/diane/internal/db"
)

func TestProviderName(t *testing.T) {
	p := NewProvider()
	if p.Name() != "jobs" {
		t.Errorf("expected provider name 'jobs', got %q", p.Name())
	}
}

func TestProviderTools(t *testing.T) {
	p := NewProvider()

	tools := p.Tools()

	expectedTools := []string{
		"job_list",
		"job_add",
		"job_enable",
		"job_disable",
		"job_delete",
		"job_pause",
		"job_resume",
		"job_logs",
		"server_status",
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

func TestHasTool(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		name     string
		expected bool
	}{
		{"job_list", true},
		{"job_add", true},
		{"job_enable", true},
		{"job_disable", true},
		{"job_delete", true},
		{"job_pause", true},
		{"job_resume", true},
		{"job_logs", true},
		{"server_status", true},
		{"unknown_tool", false},
	}

	for _, tt := range tests {
		if got := p.HasTool(tt.name); got != tt.expected {
			t.Errorf("HasTool(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestCallUnknownTool(t *testing.T) {
	p := NewProvider()

	_, err := p.Call("unknown_tool", nil)
	if err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
}

func TestToolInputSchemas(t *testing.T) {
	p := NewProvider()

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

// --- Integration tests with temp database ---

func setupTestDB(t *testing.T) (*Provider, func()) {
	tmpDir, err := os.MkdirTemp("", "jobs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	p := NewProviderWithPath(dbPath)

	// Initialize the database by opening it once
	database, err := db.New(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create test database: %v", err)
	}
	database.Close()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return p, cleanup
}

func TestJobList(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	result, err := p.Call("job_list", nil)
	if err != nil {
		t.Fatalf("job_list failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestJobAddAndList(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a job
	_, err := p.Call("job_add", map[string]interface{}{
		"name":     "test-job",
		"schedule": "* * * * *",
		"command":  "echo hello",
	})
	if err != nil {
		t.Fatalf("job_add failed: %v", err)
	}

	// List jobs
	result, err := p.Call("job_list", nil)
	if err != nil {
		t.Fatalf("job_list failed: %v", err)
	}

	// Result should contain our job
	content, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	contentArr, ok := content["content"].([]map[string]interface{})
	if !ok || len(contentArr) == 0 {
		t.Fatal("expected content array")
	}

	text, ok := contentArr[0]["text"].(string)
	if !ok {
		t.Fatal("expected text in content")
	}

	if !contains(text, "test-job") {
		t.Error("job list should contain 'test-job'")
	}
}

func TestJobEnableDisable(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a job
	_, err := p.Call("job_add", map[string]interface{}{
		"name":     "toggle-job",
		"schedule": "0 * * * *",
		"command":  "echo test",
	})
	if err != nil {
		t.Fatalf("job_add failed: %v", err)
	}

	// Disable the job
	result, err := p.Call("job_disable", map[string]interface{}{
		"job": "toggle-job",
	})
	if err != nil {
		t.Fatalf("job_disable failed: %v", err)
	}
	if !containsTextResult(result, "disabled") {
		t.Error("expected 'disabled' in result")
	}

	// Enable the job
	result, err = p.Call("job_enable", map[string]interface{}{
		"job": "toggle-job",
	})
	if err != nil {
		t.Fatalf("job_enable failed: %v", err)
	}
	if !containsTextResult(result, "enabled") {
		t.Error("expected 'enabled' in result")
	}
}

func TestJobDelete(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a job
	_, err := p.Call("job_add", map[string]interface{}{
		"name":     "delete-me",
		"schedule": "0 0 * * *",
		"command":  "echo bye",
	})
	if err != nil {
		t.Fatalf("job_add failed: %v", err)
	}

	// Delete the job
	result, err := p.Call("job_delete", map[string]interface{}{
		"job": "delete-me",
	})
	if err != nil {
		t.Fatalf("job_delete failed: %v", err)
	}
	if !containsTextResult(result, "deleted") {
		t.Error("expected 'deleted' in result")
	}
}

func TestJobAddMissingArgs(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	// Missing name
	_, err := p.Call("job_add", map[string]interface{}{
		"schedule": "* * * * *",
		"command":  "echo test",
	})
	if err == nil {
		t.Error("expected error for missing 'name'")
	}

	// Missing schedule
	_, err = p.Call("job_add", map[string]interface{}{
		"name":    "test",
		"command": "echo test",
	})
	if err == nil {
		t.Error("expected error for missing 'schedule'")
	}

	// Missing command
	_, err = p.Call("job_add", map[string]interface{}{
		"name":     "test",
		"schedule": "* * * * *",
	})
	if err == nil {
		t.Error("expected error for missing 'command'")
	}
}

func TestJobLogs(t *testing.T) {
	p, cleanup := setupTestDB(t)
	defer cleanup()

	// Just verify it doesn't error with empty logs
	result, err := p.Call("job_logs", map[string]interface{}{
		"limit": float64(10),
	})
	if err != nil {
		t.Fatalf("job_logs failed: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestServerStatus(t *testing.T) {
	p := NewProvider()

	result, err := p.Call("server_status", nil)
	if err != nil {
		t.Fatalf("server_status failed: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// --- Prompt Tests ---

func TestPrompts(t *testing.T) {
	p := NewProvider()

	prompts := p.Prompts()

	expectedPrompts := []string{
		"jobs_create_scheduled_task",
		"jobs_review_schedules",
		"jobs_troubleshoot_failures",
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

func TestGetPrompt(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		name      string
		args      map[string]string
		expectErr bool
	}{
		{
			name:      "jobs_create_scheduled_task",
			args:      map[string]string{"task_description": "backup", "frequency": "daily", "command": "backup.sh"},
			expectErr: false,
		},
		{
			name:      "jobs_review_schedules",
			args:      nil,
			expectErr: false,
		},
		{
			name:      "jobs_troubleshoot_failures",
			args:      map[string]string{"job_name": "test-job"},
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
		}
	}
}

func TestPromptCreateScheduledTaskContent(t *testing.T) {
	p := NewProvider()

	messages, _ := p.GetPrompt("jobs_create_scheduled_task", map[string]string{
		"task_description": "database backup",
		"frequency":        "every day at midnight",
		"command":          "/usr/bin/backup-db.sh",
	})

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	text := messages[0].Content.Text
	if !contains(text, "database backup") {
		t.Error("prompt should contain task description")
	}
	if !contains(text, "every day at midnight") {
		t.Error("prompt should contain frequency")
	}
	if !contains(text, "/usr/bin/backup-db.sh") {
		t.Error("prompt should contain command")
	}
	if !contains(text, "job_add") {
		t.Error("prompt should reference job_add tool")
	}
}

// Helper functions

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsTextResult(result interface{}, substr string) bool {
	content, ok := result.(map[string]interface{})
	if !ok {
		return false
	}

	contentArr, ok := content["content"].([]map[string]interface{})
	if !ok || len(contentArr) == 0 {
		return false
	}

	text, ok := contentArr[0]["text"].(string)
	if !ok {
		return false
	}

	return contains(text, substr)
}
