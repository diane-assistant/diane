package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/diane-assistant/diane/internal/db"
)

// ---------------------------------------------------------------------------
// In-memory mock stores for testing
// ---------------------------------------------------------------------------

type mockJobStore struct {
	mu   sync.Mutex
	jobs map[int64]*db.Job
	seq  int64
}

func newMockJobStore() *mockJobStore {
	return &mockJobStore{jobs: make(map[int64]*db.Job)}
}

func (s *mockJobStore) CreateJob(_ context.Context, name, command, schedule string) (*db.Job, error) {
	return s.CreateJobWithAction(context.Background(), name, command, schedule, "shell", nil)
}

func (s *mockJobStore) CreateJobWithAction(_ context.Context, name, command, schedule, actionType string, agentName *string) (*db.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.Name == name {
			return nil, fmt.Errorf("duplicate job name: %s", name)
		}
	}
	s.seq++
	now := time.Now()
	j := &db.Job{
		ID: s.seq, Name: name, Command: command, Schedule: schedule,
		Enabled: true, ActionType: actionType, AgentName: agentName,
		CreatedAt: now, UpdatedAt: now,
	}
	s.jobs[j.ID] = j
	return j, nil
}

func (s *mockJobStore) GetJob(_ context.Context, id int64) (*db.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: id=%d", id)
	}
	return j, nil
}

func (s *mockJobStore) GetJobByName(_ context.Context, name string) (*db.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.Name == name {
			return j, nil
		}
	}
	return nil, fmt.Errorf("job not found: name=%s", name)
}

func (s *mockJobStore) ListJobs(_ context.Context, enabledOnly bool) ([]*db.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*db.Job
	for _, j := range s.jobs {
		if enabledOnly && !j.Enabled {
			continue
		}
		result = append(result, j)
	}
	sort.Slice(result, func(i, k int) bool { return result[i].Name < result[k].Name })
	return result, nil
}

func (s *mockJobStore) UpdateJob(_ context.Context, id int64, command, schedule *string, enabled *bool) error {
	return s.UpdateJobFull(context.Background(), id, command, schedule, enabled, nil, nil)
}

func (s *mockJobStore) UpdateJobFull(_ context.Context, id int64, command, schedule *string, enabled *bool, _ *string, _ *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: id=%d", id)
	}
	if command != nil {
		j.Command = *command
	}
	if schedule != nil {
		j.Schedule = *schedule
	}
	if enabled != nil {
		j.Enabled = *enabled
	}
	j.UpdatedAt = time.Now()
	return nil
}

func (s *mockJobStore) DeleteJob(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
	return nil
}

type mockExecutionStore struct{}

func (s *mockExecutionStore) CreateJobExecution(_ context.Context, _ int64) (int64, error) {
	return 1, nil
}
func (s *mockExecutionStore) UpdateJobExecution(_ context.Context, _ int64, _ int, _, _ string, _ error) error {
	return nil
}
func (s *mockExecutionStore) GetJobExecution(_ context.Context, _ int64) (*db.JobExecution, error) {
	return nil, fmt.Errorf("not found")
}
func (s *mockExecutionStore) ListJobExecutions(_ context.Context, _ *int64, _, _ int) ([]*db.JobExecution, error) {
	return nil, nil
}
func (s *mockExecutionStore) DeleteOldExecutions(_ context.Context, _ int) (int64, error) {
	return 0, nil
}

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

// --- Integration tests with mock stores ---

func setupTestDB(t *testing.T) (*Provider, func()) {
	p := NewProviderWithStores(newMockJobStore(), &mockExecutionStore{})
	return p, func() {}
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
