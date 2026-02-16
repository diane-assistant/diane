package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/diane-assistant/diane/internal/config"
	"github.com/diane-assistant/diane/internal/db"
	"github.com/diane-assistant/diane/internal/logger"
	"github.com/diane-assistant/diane/internal/mcpproxy"
	"github.com/diane-assistant/diane/internal/slave"
	"github.com/diane-assistant/diane/mcp/tools"
	"github.com/diane-assistant/diane/mcp/tools/apple"
	"github.com/diane-assistant/diane/mcp/tools/downloads"
	"github.com/diane-assistant/diane/mcp/tools/files"
	"github.com/diane-assistant/diane/mcp/tools/finance"
	githubbot "github.com/diane-assistant/diane/mcp/tools/github"
	"github.com/diane-assistant/diane/mcp/tools/google"
	"github.com/diane-assistant/diane/mcp/tools/infrastructure"
	"github.com/diane-assistant/diane/mcp/tools/notifications"
	"github.com/diane-assistant/diane/mcp/tools/places"
	"github.com/diane-assistant/diane/mcp/tools/weather"
)

// MCP Server for Diane
// Provides tools for managing cron jobs and proxies other MCP servers

// Version is set at build time via ldflags
var Version = "dev"

var proxy *mcpproxy.Proxy
var slaveManager *slave.Manager
var globalEncoder *json.Encoder // For sending notifications
var appleProvider *apple.Provider
var googleProvider *google.Provider
var infrastructureProvider *infrastructure.Provider
var notificationsProvider *notifications.Provider
var financeProvider *finance.Provider
var placesProvider *places.Provider
var weatherProvider *weather.Provider
var githubProvider *githubbot.Provider    // GitHub App bot tools
var downloadsProvider *downloads.Provider // File download tools
var filesProvider *files.Provider         // File index tools
var apiServer *api.Server
var mcpHTTPServer *api.MCPHTTPServer
var database *db.DB // Shared database instance
var startTime time.Time

// DBConfigProvider implements mcpproxy.ConfigProvider, loading server configs from the database
type DBConfigProvider struct {
	db *db.DB
}

func (p *DBConfigProvider) LoadMCPServerConfigs() ([]mcpproxy.ServerConfig, error) {
	servers, err := p.db.ListMCPServers()
	if err != nil {
		return nil, err
	}

	configs := make([]mcpproxy.ServerConfig, 0, len(servers))
	for _, s := range servers {
		// Skip builtin servers — they are managed separately
		if s.Type == "builtin" {
			continue
		}
		var oauth *mcpproxy.OAuthConfig
		if s.OAuth != nil {
			oauth = &mcpproxy.OAuthConfig{
				Provider:      s.OAuth.Provider,
				ClientID:      s.OAuth.ClientID,
				ClientSecret:  s.OAuth.ClientSecret,
				Scopes:        s.OAuth.Scopes,
				DeviceAuthURL: s.OAuth.DeviceAuthURL,
				TokenURL:      s.OAuth.TokenURL,
			}
		}
		configs = append(configs, mcpproxy.ServerConfig{
			Name:    s.Name,
			Enabled: s.Enabled,
			Type:    s.Type,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			URL:     s.URL,
			Headers: s.Headers,
			OAuth:   oauth,
		})
	}
	return configs, nil
}

// DianeStatusProvider implements api.StatusProvider
type DianeStatusProvider struct{}

func (d *DianeStatusProvider) GetStatus() api.Status {
	status := api.Status{
		Running:       true,
		PID:           os.Getpid(),
		Version:       Version,
		StartedAt:     startTime,
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		Uptime:        formatDuration(time.Since(startTime)),
	}

	// Get all MCP servers (builtin providers + external)
	status.MCPServers = d.getAllMCPServers()

	// Calculate total tools
	status.TotalTools = d.countTotalTools()

	return status
}

func (d *DianeStatusProvider) GetMCPServers() []api.MCPServerStatus {
	return d.getAllMCPServers()
}

// getAllMCPServers returns all MCP servers including builtin providers
func (d *DianeStatusProvider) getAllMCPServers() []api.MCPServerStatus {
	var servers []api.MCPServerStatus

	// Jobs tools are always available (built into the server)
	servers = append(servers, api.MCPServerStatus{
		Name:        "jobs",
		Enabled:     true,
		Connected:   true,
		ToolCount:   9, // job_list, job_add, job_enable, job_disable, job_delete, job_pause, job_resume, job_logs, server_status
		PromptCount: 3, // jobs_create_scheduled_task, jobs_review_schedules, jobs_troubleshoot_failures
		Builtin:     true,
	})

	// Add builtin providers as MCP servers
	if appleProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "apple",
			Enabled:   true,
			Connected: true,
			ToolCount: len(appleProvider.Tools()),
			Builtin:   true,
		})
	}

	if googleProvider != nil {
		promptCount := 0
		if pp, ok := interface{}(googleProvider).(tools.PromptProvider); ok {
			promptCount = len(pp.Prompts())
		}
		resourceCount := 0
		if rp, ok := interface{}(googleProvider).(tools.ResourceProvider); ok {
			resourceCount = len(rp.Resources())
		}
		servers = append(servers, api.MCPServerStatus{
			Name:          "google",
			Enabled:       true,
			Connected:     true,
			ToolCount:     len(googleProvider.Tools()),
			PromptCount:   promptCount,
			ResourceCount: resourceCount,
			Builtin:       true,
		})
	}

	if infrastructureProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "infrastructure",
			Enabled:   true,
			Connected: true,
			ToolCount: len(infrastructureProvider.Tools()),
			Builtin:   true,
		})
	}

	if notificationsProvider != nil {
		promptCount := 0
		if pp, ok := interface{}(notificationsProvider).(tools.PromptProvider); ok {
			promptCount = len(pp.Prompts())
		}
		servers = append(servers, api.MCPServerStatus{
			Name:        "discord",
			Enabled:     true,
			Connected:   true,
			ToolCount:   len(notificationsProvider.Tools()),
			PromptCount: promptCount,
			Builtin:     true,
		})
	}

	if financeProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "finance",
			Enabled:   true,
			Connected: true,
			ToolCount: len(financeProvider.Tools()),
			Builtin:   true,
		})
	}

	if placesProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "places",
			Enabled:   true,
			Connected: true,
			ToolCount: len(placesProvider.Tools()),
			Builtin:   true,
		})
	}

	if weatherProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "weather",
			Enabled:   true,
			Connected: true,
			ToolCount: len(weatherProvider.Tools()),
			Builtin:   true,
		})
	}

	if githubProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "github-bot",
			Enabled:   true,
			Connected: true,
			ToolCount: len(githubProvider.Tools()),
			Builtin:   true,
		})
	}

	if downloadsProvider != nil {
		resourceCount := 0
		if rp, ok := interface{}(downloadsProvider).(tools.ResourceProvider); ok {
			resourceCount = len(rp.Resources())
		}
		servers = append(servers, api.MCPServerStatus{
			Name:          "downloads",
			Enabled:       true,
			Connected:     true,
			ToolCount:     len(downloadsProvider.Tools()),
			ResourceCount: resourceCount,
			Builtin:       true,
		})
	}

	if filesProvider != nil {
		servers = append(servers, api.MCPServerStatus{
			Name:      "file_registry",
			Enabled:   true,
			Connected: true,
			ToolCount: len(filesProvider.Tools()),
			Builtin:   true,
		})
	}

	// Add external MCP servers from proxy
	if proxy != nil {
		proxyStatuses := proxy.GetServerStatuses()
		for _, s := range proxyStatuses {
			servers = append(servers, api.MCPServerStatus{
				Name:          s.Name,
				Enabled:       s.Enabled,
				Connected:     s.Connected,
				ToolCount:     s.ToolCount,
				PromptCount:   s.PromptCount,
				ResourceCount: s.ResourceCount,
				Error:         s.Error,
				Builtin:       false,
				RequiresAuth:  s.RequiresAuth,
				Authenticated: s.Authenticated,
			})
		}
	}

	return servers
}

func (d *DianeStatusProvider) RestartMCPServer(name string) error {
	if proxy == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return proxy.RestartServer(name)
}

func (d *DianeStatusProvider) ReloadConfig() error {
	if proxy == nil {
		return fmt.Errorf("proxy not initialized")
	}
	return proxy.Reload()
}

// GetJobs returns all scheduled jobs
func (d *DianeStatusProvider) GetJobs() ([]api.Job, error) {
	database, err := getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	dbJobs, err := database.ListJobs(false)
	if err != nil {
		return nil, err
	}

	jobs := make([]api.Job, 0, len(dbJobs))
	for _, j := range dbJobs {
		jobs = append(jobs, api.Job{
			ID:         j.ID,
			Name:       j.Name,
			Command:    j.Command,
			Schedule:   j.Schedule,
			Enabled:    j.Enabled,
			ActionType: j.ActionType,
			AgentName:  j.AgentName,
			CreatedAt:  j.CreatedAt,
			UpdatedAt:  j.UpdatedAt,
		})
	}
	return jobs, nil
}

// GetJobLogs returns job execution logs
func (d *DianeStatusProvider) GetJobLogs(jobName string, limit int) ([]api.JobExecution, error) {
	database, err := getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	var jobID *int64
	var jobNameMap = make(map[int64]string)

	// Build job name map for all jobs
	allJobs, _ := database.ListJobs(false)
	for _, j := range allJobs {
		jobNameMap[j.ID] = j.Name
	}

	// If filtering by job name, get the job ID
	if jobName != "" {
		job, err := database.GetJobByName(jobName)
		if err != nil {
			return nil, fmt.Errorf("job not found: %s", jobName)
		}
		jobID = &job.ID
	}

	dbExecs, err := database.ListJobExecutions(jobID, limit, 0)
	if err != nil {
		return nil, err
	}

	execs := make([]api.JobExecution, 0, len(dbExecs))
	for _, e := range dbExecs {
		exec := api.JobExecution{
			ID:        e.ID,
			JobID:     e.JobID,
			JobName:   jobNameMap[e.JobID],
			StartedAt: e.StartedAt,
			EndedAt:   e.EndedAt,
			ExitCode:  e.ExitCode,
			Stdout:    e.Stdout,
			Stderr:    e.Stderr,
			Error:     e.Error,
		}
		execs = append(execs, exec)
	}
	return execs, nil
}

// ToggleJob enables or disables a job
func (d *DianeStatusProvider) ToggleJob(name string, enabled bool) error {
	database, err := getDB()
	if err != nil {
		return err
	}
	defer database.Close()

	job, err := database.GetJobByName(name)
	if err != nil {
		return fmt.Errorf("job not found: %s", name)
	}

	return database.UpdateJob(job.ID, nil, nil, &enabled)
}

// GetAgentLogs returns agent communication logs
func (d *DianeStatusProvider) GetAgentLogs(agentName string, limit int) ([]api.AgentLog, error) {
	database, err := getDB()
	if err != nil {
		return nil, err
	}
	defer database.Close()

	var agentNamePtr *string
	if agentName != "" {
		agentNamePtr = &agentName
	}

	dbLogs, err := database.ListAgentLogs(agentNamePtr, limit, 0)
	if err != nil {
		return nil, err
	}

	logs := make([]api.AgentLog, len(dbLogs))
	for i, l := range dbLogs {
		logs[i] = api.AgentLog{
			ID:          l.ID,
			AgentName:   l.AgentName,
			Direction:   l.Direction,
			MessageType: l.MessageType,
			Content:     l.Content,
			Error:       l.Error,
			DurationMs:  l.DurationMs,
			Timestamp:   l.CreatedAt,
		}
	}

	return logs, nil
}

// CreateAgentLog creates an agent communication log entry
func (d *DianeStatusProvider) CreateAgentLog(agentName, direction, messageType string, content, errMsg *string, durationMs *int) error {
	database, err := getDB()
	if err != nil {
		return err
	}
	defer database.Close()

	_, err = database.CreateAgentLog(agentName, direction, messageType, content, errMsg, durationMs)
	return err
}

// OAuth methods for MCP server authentication

// GetOAuthServers returns all MCP servers with OAuth configuration
func (d *DianeStatusProvider) GetOAuthServers() []api.OAuthServerInfo {
	if proxy == nil {
		return nil
	}

	oauthMgr := mcpproxy.GetOAuthManager()
	servers := proxy.GetServersWithOAuth()
	result := make([]api.OAuthServerInfo, 0, len(servers))

	for _, s := range servers {
		info := api.OAuthServerInfo{
			Name: s.Name,
		}

		if s.OAuth != nil {
			info.Provider = s.OAuth.Provider
		}

		if oauthMgr != nil {
			status := oauthMgr.GetTokenStatus(s.Name)
			if authenticated, ok := status["authenticated"].(bool); ok {
				info.Authenticated = authenticated
			}
			if st, ok := status["status"].(string); ok {
				info.Status = st
			}
		} else {
			info.Status = "oauth_unavailable"
		}

		result = append(result, info)
	}

	return result
}

// GetOAuthStatus returns OAuth status for a specific server
func (d *DianeStatusProvider) GetOAuthStatus(serverName string) (map[string]interface{}, error) {
	if proxy == nil {
		return nil, fmt.Errorf("proxy not initialized")
	}

	config := proxy.GetServerConfig(serverName)
	if config == nil {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if config.OAuth == nil {
		return nil, fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	oauthMgr := mcpproxy.GetOAuthManager()
	if oauthMgr == nil {
		return nil, fmt.Errorf("OAuth manager not available")
	}

	status := oauthMgr.GetTokenStatus(serverName)
	status["server"] = serverName
	if config.OAuth.Provider != "" {
		status["provider"] = config.OAuth.Provider
	}

	return status, nil
}

// StartOAuthLogin starts the OAuth device flow for a server
func (d *DianeStatusProvider) StartOAuthLogin(serverName string) (*api.DeviceCodeInfo, error) {
	if proxy == nil {
		return nil, fmt.Errorf("proxy not initialized")
	}

	config := proxy.GetServerConfig(serverName)
	if config == nil {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if config.OAuth == nil {
		return nil, fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	providerConfig := mcpproxy.GetProviderConfig(config.OAuth)
	if providerConfig == nil {
		return nil, fmt.Errorf("invalid OAuth configuration for server %s", serverName)
	}

	oauthMgr := mcpproxy.GetOAuthManager()
	if oauthMgr == nil {
		return nil, fmt.Errorf("OAuth manager not available")
	}

	deviceResp, err := oauthMgr.StartDeviceFlow(serverName, providerConfig)
	if err != nil {
		return nil, err
	}

	return &api.DeviceCodeInfo{
		UserCode:        deviceResp.UserCode,
		VerificationURI: deviceResp.VerificationURI,
		ExpiresIn:       deviceResp.ExpiresIn,
		Interval:        deviceResp.Interval,
		DeviceCode:      deviceResp.DeviceCode,
	}, nil
}

// PollOAuthToken polls for the OAuth token after user authorization
func (d *DianeStatusProvider) PollOAuthToken(serverName string, deviceCode string, interval int) error {
	if proxy == nil {
		return fmt.Errorf("proxy not initialized")
	}

	config := proxy.GetServerConfig(serverName)
	if config == nil {
		return fmt.Errorf("server not found: %s", serverName)
	}

	if config.OAuth == nil {
		return fmt.Errorf("server %s does not have OAuth configured", serverName)
	}

	providerConfig := mcpproxy.GetProviderConfig(config.OAuth)
	if providerConfig == nil {
		return fmt.Errorf("invalid OAuth configuration for server %s", serverName)
	}

	oauthMgr := mcpproxy.GetOAuthManager()
	if oauthMgr == nil {
		return fmt.Errorf("OAuth manager not available")
	}

	_, err := oauthMgr.PollForToken(serverName, providerConfig, deviceCode, interval)
	if err != nil {
		return err
	}

	// After successful auth, restart the server to re-initialize with the new token
	slog.Info("OAuth authentication successful, restarting server", "server", serverName)
	if err := proxy.RestartServer(serverName); err != nil {
		slog.Warn("Failed to restart server after OAuth", "server", serverName, "error", err)
	}

	return nil
}

// DeleteOAuthToken removes OAuth token for a server
func (d *DianeStatusProvider) DeleteOAuthToken(serverName string) error {
	oauthMgr := mcpproxy.GetOAuthManager()
	if oauthMgr == nil {
		return fmt.Errorf("OAuth manager not available")
	}

	return oauthMgr.DeleteToken(serverName)
}

// GetAllTools returns detailed information about all available tools
func (d *DianeStatusProvider) GetAllTools() []api.ToolInfo {
	var tools []api.ToolInfo

	// Built-in job tools
	builtinTools := []struct{ name, desc string }{
		{"job_list", "List all cron jobs with their schedules and enabled status"},
		{"job_add", "Add a new cron job with schedule and command"},
		{"job_enable", "Enable a previously disabled job"},
		{"job_disable", "Disable a job without deleting it"},
		{"job_delete", "Delete a job permanently"},
		{"job_pause", "Pause all job execution temporarily"},
		{"job_resume", "Resume paused job execution"},
		{"job_logs", "View recent execution logs for a job"},
		{"server_status", "Get Diane server status and statistics"},
	}
	for _, t := range builtinTools {
		tools = append(tools, api.ToolInfo{
			Name:        t.name,
			Description: t.desc,
			Server:      "jobs",
			Builtin:     true,
		})
	}

	// Apple tools
	if appleProvider != nil {
		for _, tool := range appleProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "apple",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Google tools
	if googleProvider != nil {
		for _, tool := range googleProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "google",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Infrastructure tools
	if infrastructureProvider != nil {
		for _, tool := range infrastructureProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "infrastructure",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Notifications tools
	if notificationsProvider != nil {
		for _, tool := range notificationsProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "discord",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Finance tools
	if financeProvider != nil {
		for _, tool := range financeProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "finance",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Places tools
	if placesProvider != nil {
		for _, tool := range placesProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "places",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Weather tools
	if weatherProvider != nil {
		for _, tool := range weatherProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "weather",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// GitHub tools
	if githubProvider != nil {
		for _, tool := range githubProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "github-bot",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Downloads tools
	if downloadsProvider != nil {
		for _, tool := range downloadsProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "downloads",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// Files tools
	if filesProvider != nil {
		for _, tool := range filesProvider.Tools() {
			tools = append(tools, api.ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				Server:      "file_registry",
				Builtin:     true,
				InputSchema: tool.InputSchema,
			})
		}
	}

	// External MCP server tools (from proxy)
	if proxy != nil {
		proxiedTools, err := proxy.ListAllTools()
		if err == nil {
			for _, t := range proxiedTools {
				name, _ := t["name"].(string)
				desc, _ := t["description"].(string)
				server, _ := t["_server"].(string)
				schema, _ := t["inputSchema"].(map[string]interface{})
				tools = append(tools, api.ToolInfo{
					Name:        name,
					Description: desc,
					Server:      server,
					Builtin:     false,
					InputSchema: schema,
				})
			}
		}
	}

	return tools
}

// GetAllPrompts returns all prompts from builtin and external MCP servers
func (d *DianeStatusProvider) GetAllPrompts() []api.PromptInfo {
	var prompts []api.PromptInfo

	// Built-in job prompts
	builtinPrompts := []struct{ name, desc string }{
		{"job_report", "Generate a report of job executions over a time period"},
		{"job_summary", "Summarize the current state of all jobs"},
		{"job_troubleshoot", "Troubleshoot a failing job"},
	}
	for _, p := range builtinPrompts {
		prompts = append(prompts, api.PromptInfo{
			Name:        p.name,
			Description: p.desc,
			Server:      "jobs",
			Builtin:     true,
		})
	}

	// Google prompts
	if googleProvider != nil {
		for _, prompt := range googleProvider.Prompts() {
			args := make([]api.PromptArgument, 0, len(prompt.Arguments))
			for _, a := range prompt.Arguments {
				args = append(args, api.PromptArgument{
					Name:        a.Name,
					Description: a.Description,
					Required:    a.Required,
				})
			}
			prompts = append(prompts, api.PromptInfo{
				Name:        prompt.Name,
				Description: prompt.Description,
				Server:      "google",
				Builtin:     true,
				Arguments:   args,
			})
		}
	}

	// Notifications prompts
	if notificationsProvider != nil {
		for _, prompt := range notificationsProvider.Prompts() {
			args := make([]api.PromptArgument, 0, len(prompt.Arguments))
			for _, a := range prompt.Arguments {
				args = append(args, api.PromptArgument{
					Name:        a.Name,
					Description: a.Description,
					Required:    a.Required,
				})
			}
			prompts = append(prompts, api.PromptInfo{
				Name:        prompt.Name,
				Description: prompt.Description,
				Server:      "discord",
				Builtin:     true,
				Arguments:   args,
			})
		}
	}

	// External MCP server prompts (from proxy)
	if proxy != nil {
		proxiedPrompts, err := proxy.ListAllPrompts()
		if err == nil {
			for _, p := range proxiedPrompts {
				name, _ := p["name"].(string)
				desc, _ := p["description"].(string)
				server, _ := p["_server"].(string)
				var args []api.PromptArgument
				if rawArgs, ok := p["arguments"].([]interface{}); ok {
					for _, ra := range rawArgs {
						if argMap, ok := ra.(map[string]interface{}); ok {
							arg := api.PromptArgument{
								Name: fmt.Sprintf("%v", argMap["name"]),
							}
							if d, ok := argMap["description"].(string); ok {
								arg.Description = d
							}
							if r, ok := argMap["required"].(bool); ok {
								arg.Required = r
							}
							args = append(args, arg)
						}
					}
				}
				prompts = append(prompts, api.PromptInfo{
					Name:        name,
					Description: desc,
					Server:      server,
					Builtin:     false,
					Arguments:   args,
				})
			}
		}
	}

	return prompts
}

// GetAllResources returns all resources from builtin and external MCP servers
func (d *DianeStatusProvider) GetAllResources() []api.ResourceInfo {
	var resources []api.ResourceInfo

	// Google resources
	if googleProvider != nil {
		for _, resource := range googleProvider.Resources() {
			resources = append(resources, api.ResourceInfo{
				Name:        resource.Name,
				Description: resource.Description,
				URI:         resource.URI,
				MimeType:    resource.MimeType,
				Server:      "google",
				Builtin:     true,
			})
		}
	}

	// Downloads resources
	if downloadsProvider != nil {
		for _, resource := range downloadsProvider.Resources() {
			resources = append(resources, api.ResourceInfo{
				Name:        resource.Name,
				Description: resource.Description,
				URI:         resource.URI,
				MimeType:    resource.MimeType,
				Server:      "downloads",
				Builtin:     true,
			})
		}
	}

	// External MCP server resources (from proxy)
	if proxy != nil {
		proxiedResources, err := proxy.ListAllResources()
		if err == nil {
			for _, r := range proxiedResources {
				name, _ := r["name"].(string)
				desc, _ := r["description"].(string)
				uri, _ := r["uri"].(string)
				mimeType, _ := r["mimeType"].(string)
				server, _ := r["_server"].(string)
				resources = append(resources, api.ResourceInfo{
					Name:        name,
					Description: desc,
					URI:         uri,
					MimeType:    mimeType,
					Server:      server,
					Builtin:     false,
				})
			}
		}
	}

	return resources
}

// GetPromptContent returns the full content of a prompt by calling the appropriate provider
func (d *DianeStatusProvider) GetPromptContent(serverName string, promptName string) (json.RawMessage, error) {
	// Check builtin providers first
	switch serverName {
	case "jobs":
		// Builtin job prompts don't have real content, return a simple message
		return json.Marshal(map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "assistant",
					"content": map[string]interface{}{
						"type": "text",
						"text": "This is a builtin prompt template. Use it by name with the MCP client.",
					},
				},
			},
		})
	case "google":
		if googleProvider != nil {
			if pp, ok := interface{}(googleProvider).(interface {
				GetPrompt(string, map[string]string) ([]tools.PromptMessage, error)
			}); ok {
				msgs, err := pp.GetPrompt(promptName, map[string]string{})
				if err != nil {
					return nil, err
				}
				return json.Marshal(map[string]interface{}{"messages": msgs})
			}
		}
		return nil, fmt.Errorf("google provider not available")
	case "discord":
		if notificationsProvider != nil {
			if pp, ok := interface{}(notificationsProvider).(interface {
				GetPrompt(string, map[string]string) ([]tools.PromptMessage, error)
			}); ok {
				msgs, err := pp.GetPrompt(promptName, map[string]string{})
				if err != nil {
					return nil, err
				}
				return json.Marshal(map[string]interface{}{"messages": msgs})
			}
		}
		return nil, fmt.Errorf("discord provider not available")
	}

	// External MCP server - use proxy
	if proxy != nil {
		// The proxy expects the prefixed name: serverName_promptName
		prefixedName := serverName + "_" + promptName
		return proxy.GetPrompt(prefixedName, map[string]string{})
	}

	return nil, fmt.Errorf("prompt not found: %s/%s", serverName, promptName)
}

// ReadResourceContent returns the full content of a resource
func (d *DianeStatusProvider) ReadResourceContent(serverName string, uri string) (json.RawMessage, error) {
	// Check builtin providers first
	switch serverName {
	case "google":
		if googleProvider != nil {
			if rp, ok := interface{}(googleProvider).(interface {
				ReadResource(string) (*tools.ResourceContent, error)
			}); ok {
				content, err := rp.ReadResource(uri)
				if err != nil {
					return nil, err
				}
				return json.Marshal(map[string]interface{}{
					"contents": []interface{}{content},
				})
			}
		}
		return nil, fmt.Errorf("google provider not available")
	case "downloads":
		if downloadsProvider != nil {
			if rp, ok := interface{}(downloadsProvider).(interface {
				ReadResource(string) (*tools.ResourceContent, error)
			}); ok {
				content, err := rp.ReadResource(uri)
				if err != nil {
					return nil, err
				}
				return json.Marshal(map[string]interface{}{
					"contents": []interface{}{content},
				})
			}
		}
		return nil, fmt.Errorf("downloads provider not available")
	}

	// External MCP server - use proxy
	if proxy != nil {
		// The proxy expects the prefixed URI: serverName://actualURI
		prefixedURI := serverName + "://" + uri
		return proxy.ReadResource(prefixedURI)
	}

	return nil, fmt.Errorf("resource not found: %s/%s", serverName, uri)
}

// GetToolsForContext returns tools filtered by context settings
func (d *DianeStatusProvider) GetToolsForContext(contextName string) []api.ToolInfo {
	// If no context specified, return all tools
	if contextName == "" {
		return d.GetAllTools()
	}

	// Get context filter from database
	database, err := getDB()
	if err != nil {
		slog.Warn("Failed to get database for context filtering", "error", err)
		return d.GetAllTools() // Fail open
	}
	defer database.Close()

	contextFilter := db.NewContextFilterAdapter(database)

	var tools []api.ToolInfo

	// Built-in job tools - check if "jobs" server is enabled in context
	builtinTools := []struct{ name, desc string }{
		{"job_list", "List all cron jobs with their schedules and enabled status"},
		{"job_add", "Add a new cron job with schedule and command"},
		{"job_enable", "Enable a previously disabled job"},
		{"job_disable", "Disable a job without deleting it"},
		{"job_delete", "Delete a job permanently"},
		{"job_pause", "Pause all job execution temporarily"},
		{"job_resume", "Resume paused job execution"},
		{"job_logs", "View recent execution logs for a job"},
		{"server_status", "Get Diane server status and statistics"},
	}
	for _, t := range builtinTools {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "jobs", t.name); enabled {
			tools = append(tools, api.ToolInfo{
				Name:        t.name,
				Description: t.desc,
				Server:      "jobs",
				Builtin:     true,
			})
		}
	}

	// Helper function to check and add provider tools
	addProviderTools := func(providerTools []struct{ Name, Description string }, serverName string) {
		for _, tool := range providerTools {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, serverName, tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      serverName,
					Builtin:     true,
				})
			}
		}
	}

	// Apple tools
	if appleProvider != nil {
		for _, tool := range appleProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "apple", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "apple",
					Builtin:     true,
				})
			}
		}
	}

	// Google tools
	if googleProvider != nil {
		for _, tool := range googleProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "google", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "google",
					Builtin:     true,
				})
			}
		}
	}

	// Infrastructure tools
	if infrastructureProvider != nil {
		for _, tool := range infrastructureProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "infrastructure", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "infrastructure",
					Builtin:     true,
				})
			}
		}
	}

	// Notifications tools
	if notificationsProvider != nil {
		for _, tool := range notificationsProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "discord", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "discord",
					Builtin:     true,
				})
			}
		}
	}

	// Finance tools
	if financeProvider != nil {
		for _, tool := range financeProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "finance", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "finance",
					Builtin:     true,
				})
			}
		}
	}

	// Places tools
	if placesProvider != nil {
		for _, tool := range placesProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "places", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "places",
					Builtin:     true,
				})
			}
		}
	}

	// Weather tools
	if weatherProvider != nil {
		for _, tool := range weatherProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "weather", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "weather",
					Builtin:     true,
				})
			}
		}
	}

	// GitHub tools
	if githubProvider != nil {
		for _, tool := range githubProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "github-bot", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "github-bot",
					Builtin:     true,
				})
			}
		}
	}

	// Downloads tools
	if downloadsProvider != nil {
		for _, tool := range downloadsProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "downloads", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "downloads",
					Builtin:     true,
				})
			}
		}
	}

	// Files tools
	if filesProvider != nil {
		for _, tool := range filesProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "file_registry", tool.Name); enabled {
				tools = append(tools, api.ToolInfo{
					Name:        tool.Name,
					Description: tool.Description,
					Server:      "file_registry",
					Builtin:     true,
				})
			}
		}
	}

	// External MCP server tools (from proxy) with context filtering
	if proxy != nil {
		proxiedTools, err := proxy.ListToolsForContext(contextName, contextFilter)
		if err == nil {
			for _, t := range proxiedTools {
				name, _ := t["name"].(string)
				desc, _ := t["description"].(string)
				server, _ := t["_server"].(string)
				schema, _ := t["inputSchema"].(map[string]interface{})
				tools = append(tools, api.ToolInfo{
					Name:        name,
					Description: desc,
					Server:      server,
					Builtin:     false,
					InputSchema: schema,
				})
			}
		}
	}

	// Suppress unused variable warning
	_ = addProviderTools

	return tools
}

func (d *DianeStatusProvider) countTotalTools() int {
	total := 9 // Built-in job tools count

	if appleProvider != nil {
		total += len(appleProvider.Tools())
	}
	if googleProvider != nil {
		total += len(googleProvider.Tools())
	}
	if infrastructureProvider != nil {
		total += len(infrastructureProvider.Tools())
	}
	if notificationsProvider != nil {
		total += len(notificationsProvider.Tools())
	}
	if financeProvider != nil {
		total += len(financeProvider.Tools())
	}
	if placesProvider != nil {
		total += len(placesProvider.Tools())
	}
	if weatherProvider != nil {
		total += len(weatherProvider.Tools())
	}
	if githubProvider != nil {
		total += len(githubProvider.Tools())
	}
	if downloadsProvider != nil {
		total += len(downloadsProvider.Tools())
	}
	if filesProvider != nil {
		total += len(filesProvider.Tools())
	}
	if proxy != nil {
		total += proxy.GetTotalToolCount()
	}

	return total
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// MCPHandlerAdapter adapts the existing handleRequest to the api.MCPHandler interface
type MCPHandlerAdapter struct {
	statusProvider *DianeStatusProvider
}

// HandleRequest implements api.MCPHandler
func (h *MCPHandlerAdapter) HandleRequest(req api.MCPRequest) api.MCPResponse {
	// Convert api.MCPRequest to local MCPRequest
	localReq := MCPRequest{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}

	// Call the existing handleRequest function
	localResp := handleRequest(localReq)

	// Convert local MCPResponse to api.MCPResponse
	var result json.RawMessage
	if localResp.Result != nil {
		result, _ = json.Marshal(localResp.Result)
	}

	var apiErr *api.MCPError
	if localResp.Error != nil {
		apiErr = &api.MCPError{
			Code:    localResp.Error.Code,
			Message: localResp.Error.Message,
		}
	}

	return api.MCPResponse{
		JSONRPC: localResp.JSONRPC,
		ID:      localResp.ID,
		Result:  result,
		Error:   apiErr,
	}
}

// GetTools implements api.MCPHandler
func (h *MCPHandlerAdapter) GetTools() ([]api.ToolInfo, error) {
	return h.statusProvider.GetAllTools(), nil
}

// HandleRequestWithContext implements api.MCPHandler for context-aware requests
func (h *MCPHandlerAdapter) HandleRequestWithContext(req api.MCPRequest, contextName string) api.MCPResponse {
	// Convert api.MCPRequest to local MCPRequest
	localReq := MCPRequest{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}

	// Call the context-aware handleRequest function
	localResp := handleRequestWithContext(localReq, contextName)

	// Convert local MCPResponse to api.MCPResponse
	var result json.RawMessage
	if localResp.Result != nil {
		result, _ = json.Marshal(localResp.Result)
	}

	var apiErr *api.MCPError
	if localResp.Error != nil {
		apiErr = &api.MCPError{
			Code:    localResp.Error.Code,
			Message: localResp.Error.Message,
		}
	}

	return api.MCPResponse{
		JSONRPC: localResp.JSONRPC,
		ID:      localResp.ID,
		Result:  result,
		Error:   apiErr,
	}
}

// GetToolsForContext implements api.MCPHandler for context-aware tool listing
func (h *MCPHandlerAdapter) GetToolsForContext(contextName string) ([]api.ToolInfo, error) {
	return h.statusProvider.GetToolsForContext(contextName), nil
}

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// acquireLock tries to acquire an exclusive lock on the lock file.
// Returns the file handle if successful, error if another instance is running.
func acquireLock(lockPath string) (*os.File, error) {
	// Ensure the directory exists
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create the lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire an exclusive lock (non-blocking)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to acquire lock (another instance running?): %w", err)
	}

	// Write our PID to the lock file for debugging
	file.Truncate(0)
	file.Seek(0, 0)
	fmt.Fprintf(file, "%d\n", os.Getpid())

	return file, nil
}

// releaseLock releases the file lock and removes the lock file.
func releaseLock(file *os.File, lockPath string) {
	if file != nil {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}
	os.Remove(lockPath)
}

func main() {
	// --- Subcommand dispatch (before any server initialization) ---
	// CTL commands (status, health, info, etc.) only need the API client,
	// not the full server. Dispatch them immediately and exit.
	if runCTLCommand(os.Args) {
		// runCTLCommand calls os.Exit internally for handled commands.
		// If we get here, it means the command was handled but didn't exit
		// (shouldn't happen, but be safe).
		return
	}

	// Determine run mode: "serve" = daemon (no stdio), default = stdio MCP
	serveMode := len(os.Args) >= 2 && os.Args[1] == "serve"

	// If there are unrecognized subcommands, show usage and exit
	if len(os.Args) >= 2 && !serveMode {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		ctlPrintUsage()
		os.Exit(1)
	}

	// If no arguments and stdin is a terminal (interactive), show help.
	// Stdio MCP mode is only entered when stdin is piped (AI tool spawning the binary).
	if !serveMode {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			// stdin is a terminal — user ran "diane" interactively
			ctlPrintUsage()
			os.Exit(0)
		}
	}

	// --- Server initialization (shared by both stdio and serve modes) ---

	// Track start time
	startTime = time.Now()

	// Get home directory for pid/lock files
	home, err := os.UserHomeDir()
	if err != nil {
		// Can't use logger yet, use stderr
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	// Load configuration from ~/.diane/config.json (with env var overrides)
	cfg := config.Load()

	// Initialize structured logging
	logDir := filepath.Join(home, ".diane")
	if err := logger.Init(logger.Config{
		LogDir:    logDir,
		Debug:     cfg.Debug,
		JSON:      true,
		Component: "diane",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	mode := "stdio"
	if serveMode {
		mode = "serve"
	}
	slog.Info("Diane server starting", "version", Version, "pid", os.Getpid(), "mode", mode)

	// Single instance check: try to acquire exclusive lock on lock file
	lockFile := filepath.Join(home, ".diane", "diane.lock")
	lock, err := acquireLock(lockFile)
	if err != nil {
		logger.Fatal("Another instance of Diane is already running", "error", err)
	}
	defer releaseLock(lock, lockFile)

	// Write PID file for reload command
	pidFile := filepath.Join(home, ".diane", "mcp.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		slog.Warn("Failed to write PID file", "error", err)
	}
	defer os.Remove(pidFile)

	// Initialize database (shared across proxy, API, and other subsystems)
	dbPath := filepath.Join(home, ".diane", "cron.db")
	var err2 error
	database, err2 = db.New(dbPath)
	if err2 != nil {
		slog.Warn("Failed to initialize database", "error", err2)
	} else {
		// One-time migration: import servers from mcp-servers.json if DB is empty
		jsonPath := filepath.Join(home, ".diane", "mcp-servers.json")
		imported, err := database.MigrateFromJSON(jsonPath)
		if err != nil {
			slog.Warn("Failed to migrate MCP servers from JSON", "error", err)
		} else if imported > 0 {
			slog.Info("Migrated MCP servers from JSON to database", "count", imported)
		}
	}
	defer func() {
		if database != nil {
			database.Close()
		}
	}()

	// Initialize MCP proxy from database
	if database != nil {
		provider := &DBConfigProvider{db: database}
		proxy, err = mcpproxy.NewProxy(provider)
		if err != nil {
			slog.Warn("Failed to initialize MCP proxy", "error", err)
		}
	} else {
		slog.Warn("MCP proxy not available: database not initialized")
	}
	defer func() {
		if proxy != nil {
			proxy.Close()
		}
	}()

	// Initialize slave manager (for master/slave pairing)
	if database != nil && proxy != nil {
		dianeDir := filepath.Join(home, ".diane")
		ca, err := slave.NewCertificateAuthority(dianeDir)
		if err != nil {
			slog.Warn("Failed to initialize CA for slave manager", "error", err)
		} else {
			slaveManager, err = slave.NewManager(database, proxy, ca)
			if err != nil {
				slog.Warn("Failed to initialize slave manager", "error", err)
			} else {
				// Initialize the slave server (doesn't start HTTP yet, just sets up handlers)
				if err := slaveManager.StartServer(":8765", ca); err != nil {
					slog.Warn("Failed to initialize slave server", "error", err)
				} else {
					slog.Info("Slave manager initialized")
				}
			}
		}
	}
	defer func() {
		if slaveManager != nil {
			slaveManager.Stop()
		}
	}()

	// Initialize Apple tools provider (only on macOS)
	appleProvider = apple.NewProvider()
	if err := appleProvider.CheckDependencies(); err != nil {
		slog.Warn("Apple tools not available", "error", err)
		appleProvider = nil
	} else {
		slog.Info("Apple tools initialized successfully")
	}

	// Initialize Google tools provider
	googleProvider = google.NewProvider()
	if err := googleProvider.CheckDependencies(); err != nil {
		slog.Warn("Google tools not available", "error", err)
		googleProvider = nil
	} else {
		slog.Info("Google tools initialized successfully")
	}

	// Initialize Infrastructure tools provider (Cloudflare DNS)
	infrastructureProvider = infrastructure.NewProvider()
	if err := infrastructureProvider.CheckDependencies(); err != nil {
		slog.Warn("Infrastructure tools not available", "error", err)
		infrastructureProvider = nil
	} else {
		slog.Info("Infrastructure tools initialized successfully")
	}

	// Initialize Notifications tools provider (Discord, Home Assistant)
	notificationsProvider = notifications.NewProvider()
	if err := notificationsProvider.CheckDependencies(); err != nil {
		slog.Warn("Notifications tools not available", "error", err)
		notificationsProvider = nil
	} else {
		slog.Info("Notifications tools initialized successfully")
	}

	// Initialize Finance tools provider (Enable Banking, Actual Budget, Bank Sync)
	financeProvider = finance.NewProvider()
	if err := financeProvider.CheckDependencies(); err != nil {
		slog.Warn("Finance tools not available", "error", err)
		financeProvider = nil
	} else {
		slog.Info("Finance tools initialized successfully")
	}

	// Initialize Google Places tools provider
	placesProvider = places.NewProvider()
	if err := placesProvider.CheckDependencies(); err != nil {
		slog.Warn("Google Places tools not available", "error", err)
		placesProvider = nil
	} else {
		slog.Info("Google Places tools initialized successfully")
	}

	// Initialize Weather tools provider
	weatherProvider = weather.NewProvider()
	if err := weatherProvider.CheckDependencies(); err != nil {
		slog.Warn("Weather tools not available", "error", err)
		weatherProvider = nil
	} else {
		slog.Info("Weather tools initialized successfully")
	}

	// Initialize GitHub Bot tools provider
	var githubErr error
	githubProvider, githubErr = githubbot.NewProvider()
	if githubErr != nil {
		slog.Warn("GitHub Bot tools not available", "error", githubErr)
		githubProvider = nil
	} else {
		slog.Info("GitHub Bot tools initialized successfully")
	}

	// Initialize Downloads tools provider
	var downloadsErr error
	downloadsProvider, downloadsErr = downloads.NewProvider()
	if downloadsErr != nil {
		slog.Warn("Downloads tools not available", "error", downloadsErr)
		downloadsProvider = nil
	} else {
		slog.Info("Downloads tools initialized successfully")
	}

	// Initialize Files tools provider (uses Emergent backend via env vars)
	var filesErr error
	filesProvider, filesErr = files.NewProvider()
	if filesErr != nil {
		slog.Warn("Files tools not available", "error", filesErr)
		filesProvider = nil
	} else {
		slog.Info("Files tools initialized successfully")
	}
	defer func() {
		if filesProvider != nil {
			filesProvider.Close()
		}
	}()

	// Start the Unix socket API server for companion app
	statusProvider := &DianeStatusProvider{}
	apiServer, err = api.NewServer(statusProvider, database, cfg, slaveManager)
	if err != nil {
		slog.Warn("Failed to create API server", "error", err)
	} else {
		if err := apiServer.Start(); err != nil {
			slog.Warn("Failed to start API server", "error", err)
		} else {
			slog.Info("API server started successfully")
		}
	}
	defer func() {
		if apiServer != nil {
			apiServer.Stop()
		}
	}()

	// Start the MCP HTTP/SSE server for network-based MCP clients
	mcpHandler := &MCPHandlerAdapter{statusProvider: statusProvider}
	mcpHTTPServer = api.NewMCPHTTPServer(statusProvider, mcpHandler, 8765)

	// Register slave routes on the public-facing MCP server so slaves can pair remotely
	// This exposes /api/slaves/... endpoints
	mcpHTTPServer.RegisterRoutes(func(mux *http.ServeMux) {
		slaveMux := http.NewServeMux()
		api.RegisterSlaveRoutes(slaveMux, apiServer)

		// Wrap with logging
		handler := http.StripPrefix("/api", slaveMux)
		loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Info("DEBUG: Request to /api prefix", "path", r.URL.Path, "method", r.Method)
			handler.ServeHTTP(w, r)
		})

		mux.Handle("/api/", loggingHandler)
	})

	// If slave manager is initialized, register its handlers and configure TLS
	if slaveManager != nil {
		slaveServer := slaveManager.GetServer()
		if slaveServer != nil {
			// Register slave WebSocket and pairing handlers
			mcpHTTPServer.RegisterRoutes(func(mux *http.ServeMux) {
				slaveServer.RegisterHandlers(mux)
			})

			// Configure TLS for client certificate authentication
			tlsConfig, err := slaveServer.GetTLSConfig()
			if err != nil {
				slog.Warn("Failed to get TLS config from slave server", "error", err)
			} else {
				certPath, keyPath, err := slaveServer.GetCertPaths()
				if err != nil {
					slog.Warn("Failed to get cert paths from slave server", "error", err)
				} else {
					mcpHTTPServer.SetTLS(tlsConfig, certPath, keyPath)
					slog.Info("MCP HTTP server configured with TLS for slave connections")
				}
			}
		}
	}

	if err := mcpHTTPServer.Start(); err != nil {
		slog.Warn("Failed to start MCP HTTP server", "error", err)
	} else {
		slog.Info("MCP HTTP server started", "port", 8765)
	}
	defer func() {
		if mcpHTTPServer != nil {
			mcpHTTPServer.Stop()
		}
	}()

	// Setup signal handler for reload (SIGUSR1)
	if proxy != nil {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGUSR1)
		go func() {
			for range sigChan {
				slog.Info("Received SIGUSR1, reloading MCP configuration")
				if err := proxy.Reload(); err != nil {
					slog.Error("Failed to reload MCP config", "error", err)
				}
			}
		}()
	}

	if serveMode {
		// --- Serve mode: daemon without stdio ---
		// Block until SIGINT or SIGTERM for graceful shutdown.
		slog.Info("Diane running in serve mode (no stdio). Press Ctrl+C to stop.")
		fmt.Fprintf(os.Stderr, "Diane %s running in serve mode (pid %d)\n", Version, os.Getpid())
		fmt.Fprintf(os.Stderr, "  Unix socket: ~/.diane/diane.sock\n")
		fmt.Fprintf(os.Stderr, "  MCP HTTP: http://localhost:8765\n")
		if cfg.HTTP.Port > 0 {
			if cfg.HTTP.APIKey != "" {
				fmt.Fprintf(os.Stderr, "  Remote API: http://0.0.0.0:%d (API key auth)\n", cfg.HTTP.Port)
			} else {
				fmt.Fprintf(os.Stderr, "  Remote API: http://0.0.0.0:%d (read-only)\n", cfg.HTTP.Port)
			}
		}
		fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		slog.Info("Received shutdown signal", "signal", sig)
		fmt.Fprintf(os.Stderr, "\nShutting down...\n")
		// Deferred cleanup runs on return
		return
	}

	// --- Stdio mode: MCP JSON-RPC over stdin/stdout (default) ---
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	globalEncoder = encoder // Store for notification forwarding

	// Start notification forwarder if proxy is available
	if proxy != nil {
		go forwardProxiedNotifications(proxy)
	}

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				// stdin not ready yet or closed temporarily
				// Wait briefly and continue listening
				time.Sleep(50 * time.Millisecond)
				continue
			}
			slog.Error("Failed to decode request", "error", err)
			break
		}

		resp := handleRequest(req)
		resp.JSONRPC = "2.0"
		resp.ID = req.ID
		if err := encoder.Encode(resp); err != nil {
			slog.Error("Failed to encode response", "error", err)
			break
		}
	}
}

func handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return initialize()
	case "tools/list":
		return listTools()
	case "tools/call":
		return callTool(req.Params)
	case "prompts/list":
		return listPrompts()
	case "prompts/get":
		return getPrompt(req.Params)
	case "resources/list":
		return listResources()
	case "resources/read":
		return readResource(req.Params)
	default:
		return MCPResponse{
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleRequestWithContext handles MCP requests with context-aware filtering
func handleRequestWithContext(req MCPRequest, contextName string) MCPResponse {
	switch req.Method {
	case "initialize":
		return initialize()
	case "tools/list":
		return listToolsForContext(contextName)
	case "tools/call":
		return callToolForContext(req.Params, contextName)
	case "prompts/list":
		return listPrompts()
	case "prompts/get":
		return getPrompt(req.Params)
	case "resources/list":
		return listResources()
	case "resources/read":
		return readResource(req.Params)
	default:
		return MCPResponse{
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// forwardProxiedNotifications monitors the proxy for tool list changes
// and forwards them to the MCP client (OpenCode)
func forwardProxiedNotifications(p *mcpproxy.Proxy) {
	for serverName := range p.NotificationChan() {
		slog.Info("Received tools/list_changed notification from proxied server", "server", serverName)

		// Send notification to stdout (to OpenCode via stdio)
		notification := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "notifications/tools/list_changed",
		}

		if err := globalEncoder.Encode(notification); err != nil {
			slog.Error("Failed to send notification", "error", err)
		} else {
			slog.Debug("Forwarded tools/list_changed notification to MCP client")
		}

		// Also send to HTTP/SSE clients
		if mcpHTTPServer != nil {
			mcpHTTPServer.SendNotification("notifications/tools/list_changed", nil)
			slog.Debug("Forwarded tools/list_changed notification to HTTP/SSE clients")
		}
	}
}

func initialize() MCPResponse {
	return MCPResponse{
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true, // Diane supports dynamic tool list updates from proxied servers
				},
				"prompts": map[string]interface{}{
					"listChanged": false,
				},
				"resources": map[string]interface{}{
					"subscribe":   false,
					"listChanged": false,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "diane",
				"version": Version,
			},
		},
	}
}

func listTools() MCPResponse {
	// Built-in tools
	tools := []map[string]interface{}{
		{
			"name":        "job_list",
			"description": "List all cron jobs with their schedules and enabled status",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"enabled_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Filter to show only enabled jobs",
					},
				},
			},
		},
		{
			"name":        "job_add",
			"description": "Add a new cron job with schedule and command",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Unique name for the job",
					},
					"schedule": map[string]interface{}{
						"type":        "string",
						"description": "Cron schedule expression (e.g., '* * * * *' for every minute)",
					},
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute",
					},
				},
				"required": []string{"name", "schedule", "command"},
			},
		},
		{
			"name":        "job_enable",
			"description": "Enable a cron job by name or ID",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"job": map[string]interface{}{
						"type":        "string",
						"description": "Job name or ID",
					},
				},
				"required": []string{"job"},
			},
		},
		{
			"name":        "job_disable",
			"description": "Disable a cron job by name or ID",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"job": map[string]interface{}{
						"type":        "string",
						"description": "Job name or ID",
					},
				},
				"required": []string{"job"},
			},
		},
		{
			"name":        "job_delete",
			"description": "Delete a cron job by name or ID (removes permanently)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"job": map[string]interface{}{
						"type":        "string",
						"description": "Job name or ID",
					},
				},
				"required": []string{"job"},
			},
		},
		{
			"name":        "job_pause",
			"description": "Pause all cron jobs (disables all enabled jobs)",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "job_resume",
			"description": "Resume all cron jobs (enables all disabled jobs)",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "job_logs",
			"description": "View execution logs for cron jobs",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"job_name": map[string]interface{}{
						"type":        "string",
						"description": "Filter logs by job name",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of logs to return (default 10)",
					},
				},
			},
		},
		{
			"name":        "server_status",
			"description": "Check if diane server is running",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	// Add Apple tools (reminders + contacts)
	if appleProvider != nil {
		for _, tool := range appleProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Google tools (gmail, drive, sheets, calendar)
	if googleProvider != nil {
		for _, tool := range googleProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Infrastructure tools (Cloudflare DNS)
	if infrastructureProvider != nil {
		for _, tool := range infrastructureProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Notifications tools (Discord, Home Assistant)
	if notificationsProvider != nil {
		for _, tool := range notificationsProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Finance tools (Enable Banking, Actual Budget, Bank Sync)
	if financeProvider != nil {
		for _, tool := range financeProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Google Places tools
	if placesProvider != nil {
		for _, tool := range placesProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Weather tools
	if weatherProvider != nil {
		for _, tool := range weatherProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add GitHub Bot tools
	if githubProvider != nil {
		for _, tool := range githubProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Downloads tools
	if downloadsProvider != nil {
		for _, tool := range downloadsProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add Files tools
	if filesProvider != nil {
		for _, tool := range filesProvider.Tools() {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
	}

	// Add proxied tools from other MCP servers
	if proxy != nil {
		proxiedTools, err := proxy.ListAllTools()
		if err != nil {
			slog.Warn("Failed to list proxied tools", "error", err)
		} else {
			tools = append(tools, proxiedTools...)
		}
	}

	return MCPResponse{
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func callTool(params json.RawMessage) MCPResponse {
	var call struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &call); err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	switch call.Name {
	case "job_list":
		return jobList(call.Arguments)
	case "job_add":
		return jobAdd(call.Arguments)
	case "job_enable":
		return jobEnable(call.Arguments)
	case "job_disable":
		return jobDisable(call.Arguments)
	case "job_delete":
		return jobDelete(call.Arguments)
	case "job_pause":
		return pauseAll()
	case "job_resume":
		return resumeAll()
	case "job_logs":
		return getLogs(call.Arguments)
	case "server_status":
		return getStatus()
	default:
		// Try Apple tools first
		if appleProvider != nil && appleProvider.HasTool(call.Name) {
			result, err := appleProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Google tools
		if googleProvider != nil && googleProvider.HasTool(call.Name) {
			result, err := googleProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Infrastructure tools (Cloudflare DNS)
		if infrastructureProvider != nil && infrastructureProvider.HasTool(call.Name) {
			result, err := infrastructureProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Notifications tools (Discord, Home Assistant)
		if notificationsProvider != nil && notificationsProvider.HasTool(call.Name) {
			result, err := notificationsProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Finance tools (Enable Banking, Actual Budget, Bank Sync)
		if financeProvider != nil && financeProvider.HasTool(call.Name) {
			result, err := financeProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Google Places tools
		if placesProvider != nil && placesProvider.HasTool(call.Name) {
			result, err := placesProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Weather tools
		if weatherProvider != nil && weatherProvider.HasTool(call.Name) {
			result, err := weatherProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try GitHub Bot tools
		if githubProvider != nil && githubProvider.HasTool(call.Name) {
			result, err := githubProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Downloads tools
		if downloadsProvider != nil && downloadsProvider.HasTool(call.Name) {
			result, err := downloadsProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try Files tools
		if filesProvider != nil && filesProvider.HasTool(call.Name) {
			result, err := filesProvider.Call(call.Name, call.Arguments)
			if err != nil {
				return MCPResponse{
					Error: &MCPError{
						Code:    -1,
						Message: err.Error(),
					},
				}
			}
			return MCPResponse{Result: result}
		}

		// Try proxied tools
		if proxy != nil {
			result, err := proxy.CallTool(call.Name, call.Arguments)
			if err == nil {
				return MCPResponse{Result: result}
			}
		}
		return MCPResponse{
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Tool not found: %s", call.Name),
			},
		}
	}
}

// listToolsForContext returns tools filtered by context
func listToolsForContext(contextName string) MCPResponse {
	// Get context filter from database
	database, err := getDB()
	if err != nil {
		slog.Warn("Failed to get database for context filtering", "error", err)
		return listTools() // Fail open - return all tools
	}
	defer database.Close()

	contextFilter := db.NewContextFilterAdapter(database)

	var tools []map[string]interface{}

	// Built-in job tools - check context
	builtinTools := []struct {
		name, desc string
		schema     map[string]interface{}
	}{
		{"job_list", "List all cron jobs with their schedules and enabled status", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"enabled_only": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter to show only enabled jobs",
				},
			},
		}},
		{"job_add", "Add a new cron job with schedule and command", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":     map[string]interface{}{"type": "string", "description": "Unique name for the job"},
				"schedule": map[string]interface{}{"type": "string", "description": "Cron schedule expression"},
				"command":  map[string]interface{}{"type": "string", "description": "Shell command to execute"},
			},
			"required": []string{"name", "schedule", "command"},
		}},
		{"job_enable", "Enable a cron job by name or ID", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job": map[string]interface{}{"type": "string", "description": "Job name or ID"},
			},
			"required": []string{"job"},
		}},
		{"job_disable", "Disable a cron job by name or ID", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job": map[string]interface{}{"type": "string", "description": "Job name or ID"},
			},
			"required": []string{"job"},
		}},
		{"job_delete", "Delete a cron job by name or ID", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job": map[string]interface{}{"type": "string", "description": "Job name or ID"},
			},
			"required": []string{"job"},
		}},
		{"job_pause", "Pause all job execution temporarily", map[string]interface{}{"type": "object"}},
		{"job_resume", "Resume paused job execution", map[string]interface{}{"type": "object"}},
		{"job_logs", "View recent execution logs for a job", map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"job":   map[string]interface{}{"type": "string", "description": "Job name or ID"},
				"limit": map[string]interface{}{"type": "integer", "description": "Maximum number of logs to return"},
			},
		}},
		{"server_status", "Get Diane server status and statistics", map[string]interface{}{"type": "object"}},
	}

	for _, t := range builtinTools {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "jobs", t.name); enabled {
			tools = append(tools, map[string]interface{}{
				"name":        t.name,
				"description": t.desc,
				"inputSchema": t.schema,
			})
		}
	}

	// Helper to add provider tools with context check
	addProviderToolsWithContext := func(providerTools []struct {
		Name        string
		Description string
		InputSchema map[string]interface{}
	}, serverName string) {
		for _, tool := range providerTools {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, serverName, tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}
	_ = addProviderToolsWithContext // suppress unused warning

	// Apple tools
	if appleProvider != nil {
		for _, tool := range appleProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "apple", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Google tools
	if googleProvider != nil {
		for _, tool := range googleProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "google", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Infrastructure tools
	if infrastructureProvider != nil {
		for _, tool := range infrastructureProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "infrastructure", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Notifications tools
	if notificationsProvider != nil {
		for _, tool := range notificationsProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "discord", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Finance tools
	if financeProvider != nil {
		for _, tool := range financeProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "finance", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Places tools
	if placesProvider != nil {
		for _, tool := range placesProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "places", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Weather tools
	if weatherProvider != nil {
		for _, tool := range weatherProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "weather", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// GitHub tools
	if githubProvider != nil {
		for _, tool := range githubProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "github-bot", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Downloads tools
	if downloadsProvider != nil {
		for _, tool := range downloadsProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "downloads", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Files tools
	if filesProvider != nil {
		for _, tool := range filesProvider.Tools() {
			if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "file_registry", tool.Name); enabled {
				tools = append(tools, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": tool.InputSchema,
				})
			}
		}
	}

	// Add proxied tools with context filtering
	if proxy != nil {
		proxiedTools, err := proxy.ListToolsForContext(contextName, contextFilter)
		if err != nil {
			slog.Warn("Failed to list proxied tools for context", "context", contextName, "error", err)
		} else {
			tools = append(tools, proxiedTools...)
		}
	}

	return MCPResponse{
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// callToolForContext calls a tool with context validation
func callToolForContext(params json.RawMessage, contextName string) MCPResponse {
	var call struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &call); err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	// Get context filter from database
	database, err := getDB()
	if err != nil {
		slog.Warn("Failed to get database for context validation", "error", err)
		return callTool(params) // Fail open
	}
	defer database.Close()

	contextFilter := db.NewContextFilterAdapter(database)

	// Check if tool is enabled in context for built-in tools
	isBuiltinTool := map[string]string{
		"job_list":      "jobs",
		"job_add":       "jobs",
		"job_enable":    "jobs",
		"job_disable":   "jobs",
		"job_delete":    "jobs",
		"job_pause":     "jobs",
		"job_resume":    "jobs",
		"job_logs":      "jobs",
		"server_status": "jobs",
	}

	if serverName, isBuiltin := isBuiltinTool[call.Name]; isBuiltin {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, serverName, call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		// Call the tool as normal
		switch call.Name {
		case "job_list":
			return jobList(call.Arguments)
		case "job_add":
			return jobAdd(call.Arguments)
		case "job_enable":
			return jobEnable(call.Arguments)
		case "job_disable":
			return jobDisable(call.Arguments)
		case "job_delete":
			return jobDelete(call.Arguments)
		case "job_pause":
			return pauseAll()
		case "job_resume":
			return resumeAll()
		case "job_logs":
			return getLogs(call.Arguments)
		case "server_status":
			return getStatus()
		}
	}

	// Check Apple tools
	if appleProvider != nil && appleProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "apple", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := appleProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Google tools
	if googleProvider != nil && googleProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "google", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := googleProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Infrastructure tools
	if infrastructureProvider != nil && infrastructureProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "infrastructure", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := infrastructureProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Notifications tools
	if notificationsProvider != nil && notificationsProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "discord", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := notificationsProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Finance tools
	if financeProvider != nil && financeProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "finance", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := financeProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Places tools
	if placesProvider != nil && placesProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "places", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := placesProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Weather tools
	if weatherProvider != nil && weatherProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "weather", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := weatherProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check GitHub tools
	if githubProvider != nil && githubProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "github-bot", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := githubProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Downloads tools
	if downloadsProvider != nil && downloadsProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "downloads", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := downloadsProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Check Files tools
	if filesProvider != nil && filesProvider.HasTool(call.Name) {
		if enabled, _ := contextFilter.IsToolEnabledInContext(contextName, "file_registry", call.Name); !enabled {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Tool %s is not enabled in context %s", call.Name, contextName),
				},
			}
		}
		result, err := filesProvider.Call(call.Name, call.Arguments)
		if err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		return MCPResponse{Result: result}
	}

	// Try proxied tools with context validation
	if proxy != nil {
		result, err := proxy.CallToolForContext(contextName, call.Name, call.Arguments, contextFilter)
		if err == nil {
			return MCPResponse{Result: result}
		}
		// Check if it's a context access error
		if err.Error() != fmt.Sprintf("unknown tool: %s", call.Name) {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32601,
					Message: err.Error(),
				},
			}
		}
	}

	return MCPResponse{
		Error: &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Tool not found: %s", call.Name),
		},
	}
}

// --- Prompts ---

func listPrompts() MCPResponse {
	var prompts []map[string]interface{}

	// Helper to add prompts from a PromptProvider
	addPrompts := func(pp tools.PromptProvider) {
		for _, p := range pp.Prompts() {
			prompt := map[string]interface{}{
				"name":        p.Name,
				"description": p.Description,
			}
			if len(p.Arguments) > 0 {
				args := make([]map[string]interface{}, len(p.Arguments))
				for i, arg := range p.Arguments {
					args[i] = map[string]interface{}{
						"name":        arg.Name,
						"description": arg.Description,
						"required":    arg.Required,
					}
				}
				prompt["arguments"] = args
			}
			prompts = append(prompts, prompt)
		}
	}

	// Collect prompts from Google provider
	if googleProvider != nil {
		if pp, ok := interface{}(googleProvider).(tools.PromptProvider); ok {
			addPrompts(pp)
		}
	}

	// Collect prompts from Discord/notifications provider
	if notificationsProvider != nil {
		if pp, ok := interface{}(notificationsProvider).(tools.PromptProvider); ok {
			addPrompts(pp)
		}
	}

	// Built-in jobs prompts
	jobsPrompts := []tools.Prompt{
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
	for _, p := range jobsPrompts {
		prompt := map[string]interface{}{
			"name":        p.Name,
			"description": p.Description,
		}
		if len(p.Arguments) > 0 {
			args := make([]map[string]interface{}, len(p.Arguments))
			for i, arg := range p.Arguments {
				args[i] = map[string]interface{}{
					"name":        arg.Name,
					"description": arg.Description,
					"required":    arg.Required,
				}
			}
			prompt["arguments"] = args
		}
		prompts = append(prompts, prompt)
	}

	// Add prompts from external MCP servers via proxy
	if proxy != nil {
		externalPrompts, err := proxy.ListAllPrompts()
		if err == nil {
			prompts = append(prompts, externalPrompts...)
		} else {
			slog.Warn("Failed to list external prompts", "error", err)
		}
	}

	return MCPResponse{
		Result: map[string]interface{}{
			"prompts": prompts,
		},
	}
}

func getPrompt(params json.RawMessage) MCPResponse {
	var req struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}

	if err := json.Unmarshal(params, &req); err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	// Helper to convert messages to response format
	convertMessages := func(messages []tools.PromptMessage) MCPResponse {
		msgs := make([]map[string]interface{}, len(messages))
		for i, m := range messages {
			msgs[i] = map[string]interface{}{
				"role": m.Role,
				"content": map[string]interface{}{
					"type": m.Content.Type,
					"text": m.Content.Text,
				},
			}
		}
		return MCPResponse{
			Result: map[string]interface{}{
				"messages": msgs,
			},
		}
	}

	// Try Google provider
	if googleProvider != nil {
		if pp, ok := interface{}(googleProvider).(tools.PromptProvider); ok {
			messages, err := pp.GetPrompt(req.Name, req.Arguments)
			if err == nil {
				return convertMessages(messages)
			}
		}
	}

	// Try Discord/notifications provider
	if notificationsProvider != nil {
		if pp, ok := interface{}(notificationsProvider).(tools.PromptProvider); ok {
			messages, err := pp.GetPrompt(req.Name, req.Arguments)
			if err == nil {
				return convertMessages(messages)
			}
		}
	}

	// Try built-in jobs prompts
	if messages := getJobsPrompt(req.Name, req.Arguments); messages != nil {
		return convertMessages(messages)
	}

	// Try external MCP servers via proxy (if name has server prefix)
	if proxy != nil {
		result, err := proxy.GetPrompt(req.Name, req.Arguments)
		if err == nil {
			return MCPResponse{
				Result: result,
			}
		}
	}

	return MCPResponse{
		Error: &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Prompt not found: %s", req.Name),
		},
	}
}

// getJobsPrompt handles the built-in jobs prompts
func getJobsPrompt(name string, args map[string]string) []tools.PromptMessage {
	getArg := func(key, defaultVal string) string {
		if val, ok := args[key]; ok && val != "" {
			return val
		}
		return defaultVal
	}

	switch name {
	case "jobs_create_scheduled_task":
		taskDesc := getArg("task_description", "a scheduled task")
		frequency := getArg("frequency", "every hour")
		command := getArg("command", "echo 'hello'")

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

	case "jobs_review_schedules":
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

	case "jobs_troubleshoot_failures":
		jobName := getArg("job_name", "")
		limit := getArg("limit", "20")

		jobFilter := ""
		logFilter := ""
		if jobName != "" {
			jobFilter = fmt.Sprintf(" for job '%s'", jobName)
			logFilter = fmt.Sprintf(" and job_name='%s'", jobName)
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

5. If there are no failures, confirm the jobs are healthy and report success rate`, jobFilter, limit, logFilter),
				},
			},
		}
	}

	return nil
}

// --- Resources ---

func listResources() MCPResponse {
	var resources []map[string]interface{}

	// Collect resources from Google provider
	if googleProvider != nil {
		if rp, ok := interface{}(googleProvider).(tools.ResourceProvider); ok {
			for _, r := range rp.Resources() {
				resources = append(resources, map[string]interface{}{
					"uri":         r.URI,
					"name":        r.Name,
					"description": r.Description,
					"mimeType":    r.MimeType,
				})
			}
		}
	}

	// Collect resources from Downloads provider
	if downloadsProvider != nil {
		if rp, ok := interface{}(downloadsProvider).(tools.ResourceProvider); ok {
			for _, r := range rp.Resources() {
				resources = append(resources, map[string]interface{}{
					"uri":         r.URI,
					"name":        r.Name,
					"description": r.Description,
					"mimeType":    r.MimeType,
				})
			}
		}
	}

	// Add resources from external MCP servers via proxy
	if proxy != nil {
		externalResources, err := proxy.ListAllResources()
		if err == nil {
			resources = append(resources, externalResources...)
		} else {
			slog.Warn("Failed to list external resources", "error", err)
		}
	}

	return MCPResponse{
		Result: map[string]interface{}{
			"resources": resources,
		},
	}
}

func readResource(params json.RawMessage) MCPResponse {
	var req struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(params, &req); err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32602,
				Message: fmt.Sprintf("Invalid params: %v", err),
			},
		}
	}

	// Try Google provider
	if googleProvider != nil {
		if rp, ok := interface{}(googleProvider).(tools.ResourceProvider); ok {
			content, err := rp.ReadResource(req.URI)
			if err == nil && content != nil {
				return MCPResponse{
					Result: map[string]interface{}{
						"contents": []map[string]interface{}{
							{
								"uri":      content.URI,
								"mimeType": content.MimeType,
								"text":     content.Text,
							},
						},
					},
				}
			}
		}
	}

	// Try Downloads provider
	if downloadsProvider != nil {
		if rp, ok := interface{}(downloadsProvider).(tools.ResourceProvider); ok {
			content, err := rp.ReadResource(req.URI)
			if err == nil && content != nil {
				return MCPResponse{
					Result: map[string]interface{}{
						"contents": []map[string]interface{}{
							{
								"uri":      content.URI,
								"mimeType": content.MimeType,
								"text":     content.Text,
							},
						},
					},
				}
			}
		}
	}

	// Try external MCP servers via proxy
	if proxy != nil {
		result, err := proxy.ReadResource(req.URI)
		if err == nil {
			return MCPResponse{
				Result: result,
			}
		}
	}

	return MCPResponse{
		Error: &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Resource not found: %s", req.URI),
		},
	}
}

func getDB() (*db.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".diane", "cron.db")
	return db.New(dbPath)
}

// Helper to format tool response in MCP content format
func mcpTextResponse(text string) MCPResponse {
	return MCPResponse{
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	}
}

func jobList(args map[string]interface{}) MCPResponse {
	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	enabledOnly := false
	if val, ok := args["enabled_only"].(bool); ok {
		enabledOnly = val
	}

	jobs, err := database.ListJobs(enabledOnly)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	// Format as JSON string for text response
	jobsJSON, _ := json.MarshalIndent(jobs, "", "  ")

	return MCPResponse{
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(jobsJSON),
				},
			},
		},
	}
}

func jobAdd(args map[string]interface{}) MCPResponse {
	name, _ := args["name"].(string)
	schedule, _ := args["schedule"].(string)
	command, _ := args["command"].(string)

	if name == "" || schedule == "" || command == "" {
		return MCPResponse{Error: &MCPError{Code: -1, Message: "name, schedule, and command are required"}}
	}

	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	job, err := database.CreateJob(name, command, schedule)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	jobJSON, _ := json.MarshalIndent(job, "", "  ")
	message := fmt.Sprintf("Job '%s' created successfully\n\n%s", name, string(jobJSON))
	return mcpTextResponse(message)
}

func jobEnable(args map[string]interface{}) MCPResponse {
	jobIdentifier, _ := args["job"].(string)
	if jobIdentifier == "" {
		return MCPResponse{Error: &MCPError{Code: -1, Message: "job identifier is required"}}
	}

	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	enabled := true
	if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	return mcpTextResponse(fmt.Sprintf("Job '%s' enabled", jobIdentifier))
}

func jobDisable(args map[string]interface{}) MCPResponse {
	jobIdentifier, _ := args["job"].(string)
	if jobIdentifier == "" {
		return MCPResponse{Error: &MCPError{Code: -1, Message: "job identifier is required"}}
	}

	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	enabled := false
	if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	return mcpTextResponse(fmt.Sprintf("Job '%s' disabled", jobIdentifier))
}

func jobDelete(args map[string]interface{}) MCPResponse {
	jobIdentifier, _ := args["job"].(string)
	if jobIdentifier == "" {
		return MCPResponse{Error: &MCPError{Code: -1, Message: "job identifier is required"}}
	}

	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	job, err := database.GetJobByName(jobIdentifier)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	if err := database.DeleteJob(job.ID); err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	return mcpTextResponse(fmt.Sprintf("Job '%s' deleted", jobIdentifier))
}

func pauseAll() MCPResponse {
	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	jobs, err := database.ListJobs(true)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	count := 0
	enabled := false
	for _, job := range jobs {
		if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
		}
		count++
	}

	return mcpTextResponse(fmt.Sprintf("Paused %d jobs", count))
}

func resumeAll() MCPResponse {
	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	allJobs, err := database.ListJobs(false)
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	count := 0
	enabled := true
	for _, job := range allJobs {
		if !job.Enabled {
			if err := database.UpdateJob(job.ID, nil, nil, &enabled); err != nil {
				return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
			}
			count++
		}
	}

	return mcpTextResponse(fmt.Sprintf("Resumed %d jobs", count))
}

func getLogs(args map[string]interface{}) MCPResponse {
	database, err := getDB()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}
	defer database.Close()

	limit := 10
	if val, ok := args["limit"].(float64); ok {
		limit = int(val)
	}

	var jobName string
	if val, ok := args["job_name"].(string); ok {
		jobName = val
	}

	// Get executions
	var executions []*db.JobExecution
	if jobName != "" {
		job, jobErr := database.GetJobByName(jobName)
		if jobErr != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: jobErr.Error()}}
		}
		var execErr error
		executions, execErr = database.ListJobExecutions(&job.ID, limit, 0)
		if execErr != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: execErr.Error()}}
		}
	} else {
		var execErr error
		executions, execErr = database.ListJobExecutions(nil, limit, 0)
		if execErr != nil {
			return MCPResponse{Error: &MCPError{Code: -1, Message: execErr.Error()}}
		}
	}

	logsJSON, _ := json.MarshalIndent(executions, "", "  ")
	return mcpTextResponse(string(logsJSON))
}

func getStatus() MCPResponse {
	home, err := os.UserHomeDir()
	if err != nil {
		return MCPResponse{Error: &MCPError{Code: -1, Message: err.Error()}}
	}

	pidFile := filepath.Join(home, ".diane", "server.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return mcpTextResponse("Server is not running")
	}

	return mcpTextResponse(fmt.Sprintf("Server is running (PID: %s)", string(pidBytes)))
}
