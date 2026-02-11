// Package jobs provides MCP tools for managing cron/scheduled jobs
package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/mcp/tools"
)

// Provider implements ToolProvider for job management
type Provider struct {
	dbPath string
}

// NewProvider creates a new jobs tools provider
func NewProvider() *Provider {
	home, _ := os.UserHomeDir()
	return &Provider{
		dbPath: filepath.Join(home, ".diane", "cron.db"),
	}
}

// NewProviderWithPath creates a provider with a specific database path (for testing)
func NewProviderWithPath(dbPath string) *Provider {
	return &Provider{dbPath: dbPath}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "jobs"
}

// CheckDependencies verifies the database is accessible
func (p *Provider) CheckDependencies() error {
	database, err := db.New(p.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open jobs database: %w", err)
	}
	database.Close()
	return nil
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Tools returns all job management tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name:        "job_list",
			Description: "List all cron jobs with their schedules and enabled status",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"enabled_only": tools.BoolProperty("Filter to show only enabled jobs", false),
				},
				nil,
			),
		},
		{
			Name:        "job_add",
			Description: "Add a new cron job with schedule and command",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"name":     tools.StringProperty("Unique name for the job", true),
					"schedule": tools.StringProperty("Cron schedule expression (e.g., '* * * * *' for every minute)", true),
					"command":  tools.StringProperty("Shell command to execute", true),
				},
				[]string{"name", "schedule", "command"},
			),
		},
		{
			Name:        "job_enable",
			Description: "Enable a cron job by name or ID",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"job": tools.StringProperty("Job name or ID", true),
				},
				[]string{"job"},
			),
		},
		{
			Name:        "job_disable",
			Description: "Disable a cron job by name or ID",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"job": tools.StringProperty("Job name or ID", true),
				},
				[]string{"job"},
			),
		},
		{
			Name:        "job_delete",
			Description: "Delete a cron job by name or ID (removes permanently)",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"job": tools.StringProperty("Job name or ID", true),
				},
				[]string{"job"},
			),
		},
		{
			Name:        "job_pause",
			Description: "Pause all cron jobs (disables all enabled jobs)",
			InputSchema: tools.ObjectSchema(map[string]interface{}{}, nil),
		},
		{
			Name:        "job_resume",
			Description: "Resume all cron jobs (enables all disabled jobs)",
			InputSchema: tools.ObjectSchema(map[string]interface{}{}, nil),
		},
		{
			Name:        "job_logs",
			Description: "View execution logs for cron jobs",
			InputSchema: tools.ObjectSchema(
				map[string]interface{}{
					"job_name": tools.StringProperty("Filter logs by job name", false),
					"limit":    tools.IntProperty("Maximum number of logs to return", 10),
				},
				nil,
			),
		},
		{
			Name:        "server_status",
			Description: "Get Diane server status and statistics",
			InputSchema: tools.ObjectSchema(map[string]interface{}{}, nil),
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	for _, tool := range p.Tools() {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// Call executes a tool by name
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "job_list":
		return p.jobList(args)
	case "job_add":
		return p.jobAdd(args)
	case "job_enable":
		return p.jobEnable(args)
	case "job_disable":
		return p.jobDisable(args)
	case "job_delete":
		return p.jobDelete(args)
	case "job_pause":
		return p.jobPause()
	case "job_resume":
		return p.jobResume()
	case "job_logs":
		return p.jobLogs(args)
	case "server_status":
		return p.serverStatus()
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (p *Provider) getDB() (*db.DB, error) {
	return db.New(p.dbPath)
}

func (p *Provider) jobList(args map[string]interface{}) (interface{}, error) {
	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	enabledOnly := tools.GetBool(args, "enabled_only", false)

	jobs, err := database.ListJobs(enabledOnly)
	if err != nil {
		return nil, err
	}

	return tools.JSONContent(jobs)
}

func (p *Provider) jobAdd(args map[string]interface{}) (interface{}, error) {
	name, err := tools.GetStringRequired(args, "name")
	if err != nil {
		return nil, err
	}
	schedule, err := tools.GetStringRequired(args, "schedule")
	if err != nil {
		return nil, err
	}
	command, err := tools.GetStringRequired(args, "command")
	if err != nil {
		return nil, err
	}

	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	job, err := database.CreateJob(name, command, schedule)
	if err != nil {
		return nil, err
	}

	jobJSON, _ := json.MarshalIndent(job, "", "  ")
	return tools.TextContent(fmt.Sprintf("Job '%s' created successfully\n\n%s", name, string(jobJSON))), nil
}

func (p *Provider) jobEnable(args map[string]interface{}) (interface{}, error) {
	jobIdentifier, err := tools.GetStringRequired(args, "job")
	if err != nil {
		return nil, err
	}

	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return nil, err
	}

	enabled := true
	if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
		return nil, err
	}

	return tools.TextContent(fmt.Sprintf("Job '%s' enabled", jobIdentifier)), nil
}

func (p *Provider) jobDisable(args map[string]interface{}) (interface{}, error) {
	jobIdentifier, err := tools.GetStringRequired(args, "job")
	if err != nil {
		return nil, err
	}

	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return nil, err
	}

	enabled := false
	if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
		return nil, err
	}

	return tools.TextContent(fmt.Sprintf("Job '%s' disabled", jobIdentifier)), nil
}

func (p *Provider) jobDelete(args map[string]interface{}) (interface{}, error) {
	jobIdentifier, err := tools.GetStringRequired(args, "job")
	if err != nil {
		return nil, err
	}

	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return nil, err
	}

	if err := database.DeleteJob(job.ID); err != nil {
		return nil, err
	}

	return tools.TextContent(fmt.Sprintf("Job '%s' deleted", jobIdentifier)), nil
}

func (p *Provider) jobPause() (interface{}, error) {
	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	jobs, err := database.ListJobs(true)
	if err != nil {
		return nil, err
	}

	count := 0
	enabled := false
	for _, job := range jobs {
		if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
			return nil, err
		}
		count++
	}

	return tools.TextContent(fmt.Sprintf("Paused %d jobs", count)), nil
}

func (p *Provider) jobResume() (interface{}, error) {
	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	allJobs, err := database.ListJobs(false)
	if err != nil {
		return nil, err
	}

	count := 0
	enabled := true
	for _, job := range allJobs {
		if !job.Enabled {
			if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
				return nil, err
			}
			count++
		}
	}

	return tools.TextContent(fmt.Sprintf("Resumed %d jobs", count)), nil
}

func (p *Provider) jobLogs(args map[string]interface{}) (interface{}, error) {
	database, err := p.getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	limit := tools.GetInt(args, "limit", 10)
	jobName := tools.GetString(args, "job_name")

	var executions []*db.JobExecution
	if jobName != "" {
		job, jobErr := database.GetJobByName(jobName)
		if jobErr != nil {
			return nil, jobErr
		}
		executions, err = database.ListJobExecutions(&job.ID, limit, 0)
	} else {
		executions, err = database.ListJobExecutions(nil, limit, 0)
	}

	if err != nil {
		return nil, err
	}

	return tools.JSONContent(executions)
}

func (p *Provider) serverStatus() (interface{}, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	pidFile := filepath.Join(home, ".diane", "server.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return tools.TextContent("Server is not running"), nil
	}

	return tools.TextContent(fmt.Sprintf("Server is running (PID: %s)", string(pidBytes))), nil
}

// --- Prompts ---

// Prompts returns all prompts provided by the jobs provider
func (p *Provider) Prompts() []tools.Prompt {
	return []tools.Prompt{
		{
			Name:        "jobs_create_scheduled_task",
			Description: "Create a new scheduled job with the appropriate cron expression for the desired frequency",
			Arguments: []tools.PromptArgument{
				{Name: "task_description", Description: "What the job should do", Required: true},
				{Name: "frequency", Description: "How often to run (e.g., 'every hour', 'daily at 9am', 'every monday')", Required: true},
				{Name: "command", Description: "The shell command to execute", Required: true},
			},
		},
		{
			Name:        "jobs_review_schedules",
			Description: "Review all configured jobs and suggest optimizations for scheduling conflicts or improvements",
			Arguments:   []tools.PromptArgument{},
		},
		{
			Name:        "jobs_troubleshoot_failures",
			Description: "Analyze recent job execution logs to identify and diagnose failures",
			Arguments: []tools.PromptArgument{
				{Name: "job_name", Description: "Specific job to troubleshoot (optional, defaults to all)", Required: false},
				{Name: "limit", Description: "Number of recent logs to analyze", Required: false},
			},
		},
	}
}

// GetPrompt returns a prompt with arguments substituted
func (p *Provider) GetPrompt(name string, args map[string]string) ([]tools.PromptMessage, error) {
	switch name {
	case "jobs_create_scheduled_task":
		return p.promptCreateScheduledTask(args), nil
	case "jobs_review_schedules":
		return p.promptReviewSchedules(args), nil
	case "jobs_troubleshoot_failures":
		return p.promptTroubleshootFailures(args), nil
	default:
		return nil, fmt.Errorf("prompt not found: %s", name)
	}
}

func getArgOrDefault(args map[string]string, key, defaultVal string) string {
	if val, ok := args[key]; ok && val != "" {
		return val
	}
	return defaultVal
}

func (p *Provider) promptCreateScheduledTask(args map[string]string) []tools.PromptMessage {
	taskDesc := getArgOrDefault(args, "task_description", "a scheduled task")
	frequency := getArgOrDefault(args, "frequency", "every hour")
	command := getArgOrDefault(args, "command", "echo 'hello'")

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Create a scheduled job for the following task:

**Task Description:** %s
**Desired Frequency:** %s
**Command:** %s

**Instructions:**
1. Convert the frequency description to a proper cron expression:
   - "every minute" = "* * * * *"
   - "every hour" = "0 * * * *"
   - "every day at midnight" = "0 0 * * *"
   - "every monday at 9am" = "0 9 * * 1"
   - "every 5 minutes" = "*/5 * * * *"
   
2. Generate a meaningful job name from the task description (lowercase, hyphens, no spaces)

3. Use job_add to create the job with:
   - name: the generated name
   - schedule: the cron expression
   - command: the provided command

4. After creating, use job_list to verify the job was created correctly

5. Explain when the job will next run`, taskDesc, frequency, command),
			},
		},
	}
}

func (p *Provider) promptReviewSchedules(args map[string]string) []tools.PromptMessage {
	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: `Review all scheduled jobs and provide optimization suggestions.

**Instructions:**
1. Use job_list to get all configured jobs

2. Analyze the schedules for:
   - **Conflicts**: Jobs scheduled at the same time that might compete for resources
   - **Clustering**: Many jobs running at :00 minutes (suggest staggering)
   - **Resource-intensive times**: Jobs that should run during off-peak hours
   - **Disabled jobs**: Jobs that are disabled and might be forgotten

3. For each issue found, provide:
   - The job name(s) affected
   - The current schedule
   - A recommended new schedule (if applicable)
   - Reasoning for the suggestion

4. Summarize with a health score (good/needs attention/critical) and action items`,
			},
		},
	}
}

func (p *Provider) promptTroubleshootFailures(args map[string]string) []tools.PromptMessage {
	jobName := getArgOrDefault(args, "job_name", "")
	limit := getArgOrDefault(args, "limit", "20")

	jobFilter := ""
	if jobName != "" {
		jobFilter = fmt.Sprintf(" for job '%s'", jobName)
	}

	return []tools.PromptMessage{
		{
			Role: "user",
			Content: tools.PromptContent{
				Type: "text",
				Text: fmt.Sprintf(`Troubleshoot recent job execution failures%s.

**Instructions:**
1. Use job_logs with limit=%s%s to get recent execution history

2. Identify failed executions (exit_code != 0 or error output)

3. For each failure, analyze:
   - **Error pattern**: Common error types (permission, missing file, network, timeout)
   - **Frequency**: Is this a recurring issue or one-time?
   - **Timing**: Does it fail at specific times?
   - **Output**: What does the error message indicate?

4. Provide diagnosis and remediation steps:
   - Root cause analysis
   - Specific fixes to try
   - Commands to verify the fix

5. If there are no failures, confirm the jobs are healthy and report success rate`, jobFilter, limit, func() string {
					if jobName != "" {
						return fmt.Sprintf(" and job_name='%s'", jobName)
					}
					return ""
				}()),
			},
		},
	}
}
