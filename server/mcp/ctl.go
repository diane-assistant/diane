package main

// ctl.go contains the CLI control commands (formerly diane-ctl).
// These are invoked as subcommands of the unified diane binary:
//   diane status, diane health, diane info, diane doctor, etc.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
	"github.com/diane-assistant/diane/internal/api"
)

// runCTLCommand dispatches a control subcommand. Returns true if the command
// was handled (caller should exit), false if not recognized (caller should
// fall through to stdio MCP mode).
func runCTLCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}

	cmd := args[1]
	client := api.NewClient()

	switch cmd {
	case "serve":
		// Handled separately in main() — should not reach here
		return false

	case "version", "--version", "-v":
		fmt.Printf("diane %s\n", Version)
		os.Exit(0)

	case "info":
		ctlHandleInfoCommand(client)

	case "status":
		status, err := client.GetStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(output))

	case "health":
		if err := client.Health(); err != nil {
			fmt.Fprintf(os.Stderr, "Diane is not running: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Diane is running")

	case "doctor":
		ctlHandleDoctorCommand(client)

	case "mcp-servers":
		ctlHandleMCPServersCommand(client)

	case "reload":
		if err := client.ReloadConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration reloaded")

	case "restart":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: diane restart <server-name>\n")
			os.Exit(1)
		}
		serverName := args[2]
		if err := client.RestartMCPServer(serverName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Server '%s' restarted\n", serverName)

	case "agents":
		ctlHandleAgentsCommand(client, args[2:])

	case "agent":
		ctlHandleAgentCommand(client, args[2:])

	case "mcp":
		ctlHandleMCPCommand(client, args[2:])

	case "gallery":
		ctlHandleGalleryCommand(client, args[2:])

	case "auth":
		ctlHandleAuthCommand(client, args[2:])

	case "help", "--help", "-h":
		ctlPrintUsage()

	default:
		return false
	}

	return true
}

func ctlHandleMCPServersCommand(client *api.Client) {
	servers, err := client.GetMCPServers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting server status: %v\n", err)
		os.Exit(1)
	}

	statusMap := make(map[string]api.MCPServerStatus)
	for _, s := range servers {
		statusMap[s.Name] = s
	}

	configs, configErr := client.GetMCPServerConfigs()

	if len(servers) == 0 && (configErr != nil || len(configs) == 0) {
		fmt.Println("No MCP servers configured.")
		return
	}

	fmt.Printf("MCP Servers (%d):\n", len(servers))

	if configErr == nil && len(configs) > 0 {
		shown := make(map[string]bool)
		for _, cfg := range configs {
			shown[cfg.Name] = true
			status := statusMap[cfg.Name]
			fmt.Println()
			ctlPrintServerDetail(cfg, status)
		}
		for _, s := range servers {
			if !shown[s.Name] {
				fmt.Println()
				ctlPrintServerStatusOnly(s)
			}
		}
	} else {
		for _, s := range servers {
			fmt.Println()
			ctlPrintServerStatusOnly(s)
		}
	}
	fmt.Println()
}

func ctlPrintServerDetail(cfg api.MCPServerResponse, status api.MCPServerStatus) {
	connStatus := "disconnected"
	if status.Connected {
		connStatus = "connected"
	}
	if status.Error != "" {
		connStatus = "error"
	}

	enabledStr := "enabled"
	if !cfg.Enabled {
		enabledStr = "disabled"
	}

	fmt.Printf("  %-20s [%s] [%s] [%s]\n", cfg.Name, cfg.Type, enabledStr, connStatus)

	if status.Error != "" {
		fmt.Printf("                       Error: %s\n", status.Error)
	}

	if status.ToolCount > 0 {
		fmt.Printf("                       Tools: %d\n", status.ToolCount)
	}

	if cfg.Type == "stdio" {
		if cfg.Command != "" {
			fmt.Printf("                       Command: %s\n", cfg.Command)
		}
		if len(cfg.Args) > 0 {
			fmt.Printf("                       Args: %v\n", cfg.Args)
		}
	} else if cfg.Type == "sse" || cfg.Type == "http" {
		if cfg.URL != "" {
			fmt.Printf("                       URL: %s\n", cfg.URL)
		}
		if len(cfg.Headers) > 0 {
			fmt.Printf("                       Headers:\n")
			for k, v := range cfg.Headers {
				fmt.Printf("                         %s: %s\n", k, v)
			}
		}
	}

	if len(cfg.Env) > 0 {
		fmt.Printf("                       Env:\n")
		for k, v := range cfg.Env {
			fmt.Printf("                         %s=%s\n", k, v)
		}
	}

	if cfg.OAuth != nil {
		oauthInfo := "configured"
		if cfg.OAuth.Provider != "" {
			oauthInfo = cfg.OAuth.Provider
		}
		if status.Authenticated {
			oauthInfo += " (authenticated)"
		} else if status.RequiresAuth {
			oauthInfo += " (not authenticated)"
		}
		fmt.Printf("                       OAuth: %s\n", oauthInfo)
	}
}

func ctlPrintServerStatusOnly(s api.MCPServerStatus) {
	connStatus := "disconnected"
	if s.Connected {
		connStatus = "connected"
	}
	if s.Error != "" {
		connStatus = "error"
	}

	enabledStr := "enabled"
	if !s.Enabled {
		enabledStr = "disabled"
	}

	serverType := "unknown"
	if s.Builtin {
		serverType = "builtin"
	}

	fmt.Printf("  %-20s [%s] [%s] [%s]\n", s.Name, serverType, enabledStr, connStatus)

	if s.Error != "" {
		fmt.Printf("                       Error: %s\n", s.Error)
	}

	if s.ToolCount > 0 {
		fmt.Printf("                       Tools: %d\n", s.ToolCount)
	}

	if s.RequiresAuth {
		authStr := "not authenticated"
		if s.Authenticated {
			authStr = "authenticated"
		}
		fmt.Printf("                       OAuth: %s\n", authStr)
	}
}

func ctlHandleMCPCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: diane mcp <subcommand> [args...]\n")
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  add <name> <url>         Add a new remote MCP server (http/sse)\n")
		fmt.Fprintf(os.Stderr, "  add-stdio <name> <cmd>   Add a new stdio MCP server\n")
		fmt.Fprintf(os.Stderr, "  install opencode         Install Diane MCP config into OpenCode project\n")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		ctlHandleMCPAdd(client, args[1:])
	case "add-stdio":
		ctlHandleMCPAddStdio(client, args[1:])
	case "install":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane mcp install <target> [--context <name>]\n")
			fmt.Fprintf(os.Stderr, "\nTargets:\n")
			fmt.Fprintf(os.Stderr, "  opencode    Install Diane MCP config into OpenCode project\n")
			os.Exit(1)
		}
		switch args[1] {
		case "opencode":
			contextName := ""
			for i := 2; i < len(args); i++ {
				if (args[i] == "--context" || args[i] == "-c") && i+1 < len(args) {
					contextName = args[i+1]
					i++
				}
			}
			ctlHandleMCPInstallOpenCode(client, contextName)
		default:
			fmt.Fprintf(os.Stderr, "Unknown install target: %s\n", args[1])
			fmt.Fprintf(os.Stderr, "Available targets: opencode\n")
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mcp subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func ctlHandleMCPAdd(client *api.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: diane mcp add <name> <url> [options]\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  --type <type>               Server type (http, sse) [default: http]\n")
		fmt.Fprintf(os.Stderr, "  --header <key>=<value>      Add HTTP header (can be used multiple times)\n")
		fmt.Fprintf(os.Stderr, "  --enabled=<true/false>      Enable/disable server [default: true]\n")
		fmt.Fprintf(os.Stderr, "  --timeout <ms>              Request timeout in milliseconds [default: 30000]\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  diane mcp add context7 https://mcp.context7.com/mcp --header \"CONTEXT7_API_KEY=your-key\"\n")
		fmt.Fprintf(os.Stderr, "  diane mcp add github https://api.github.com/mcp --type sse\n")
		os.Exit(1)
	}

	name := args[0]
	url := args[1]
	serverType := "http"
	headers := make(map[string]string)
	enabled := true

	for i := 2; i < len(args); i++ {
		arg := args[i]

		if arg == "--type" && i+1 < len(args) {
			serverType = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--header") {
			var headerArg string
			if arg == "--header" && i+1 < len(args) {
				headerArg = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--header=") {
				headerArg = strings.TrimPrefix(arg, "--header=")
			}

			if headerArg != "" {
				parts := strings.SplitN(headerArg, "=", 2)
				if len(parts) == 2 {
					headers[parts[0]] = parts[1]
				} else {
					fmt.Fprintf(os.Stderr, "Error: Invalid header format '%s'. Use --header key=value\n", headerArg)
					os.Exit(1)
				}
			}
		} else if strings.HasPrefix(arg, "--enabled") {
			var enabledArg string
			if arg == "--enabled" && i+1 < len(args) {
				enabledArg = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--enabled=") {
				enabledArg = strings.TrimPrefix(arg, "--enabled=")
			}

			if enabledArg == "false" || enabledArg == "0" {
				enabled = false
			} else if enabledArg == "true" || enabledArg == "1" {
				enabled = true
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid enabled value '%s'. Use true or false\n", enabledArg)
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "--timeout") {
			if arg == "--timeout" && i+1 < len(args) {
				i++
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: Unknown option '%s'\n", arg)
			os.Exit(1)
		}
	}

	if serverType != "http" && serverType != "sse" {
		fmt.Fprintf(os.Stderr, "Error: Invalid server type '%s'. Must be 'http' or 'sse'\n", serverType)
		os.Exit(1)
	}

	req := api.CreateMCPServerRequest{
		Name:    name,
		Type:    serverType,
		URL:     url,
		Headers: headers,
		Enabled: &enabled,
	}

	server, err := client.CreateMCPServer(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("MCP server '%s' added successfully\n", server.Name)
	fmt.Printf("  Type: %s\n", server.Type)
	fmt.Printf("  URL: %s\n", server.URL)
	fmt.Printf("  Enabled: %t\n", server.Enabled)
	if len(server.Headers) > 0 {
		fmt.Printf("  Headers:\n")
		for k, v := range server.Headers {
			displayValue := v
			if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") {
				if len(v) > 8 {
					displayValue = v[:4] + "..." + v[len(v)-4:]
				} else {
					displayValue = "***"
				}
			}
			fmt.Printf("    %s: %s\n", k, displayValue)
		}
	}
	fmt.Printf("  ID: %d\n", server.ID)
	fmt.Printf("  Created: %s\n", server.CreatedAt)
	fmt.Println()
	fmt.Println("Run 'diane reload' to reload the configuration and start the server.")
}

func ctlHandleMCPAddStdio(client *api.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: diane mcp add-stdio <name> <command> [options]\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  --arg <value>               Add command argument (can be used multiple times)\n")
		fmt.Fprintf(os.Stderr, "  --env <KEY>=<VALUE>          Set environment variable (can be used multiple times)\n")
		fmt.Fprintf(os.Stderr, "  --enabled=<true/false>       Enable/disable server [default: true]\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  diane mcp add-stdio specmcp /path/to/specmcp --env \"EMERGENT_TOKEN=emt_xxx\" --env \"EMERGENT_URL=http://localhost:3002\"\n")
		fmt.Fprintf(os.Stderr, "  diane mcp add-stdio myserver /usr/bin/myserver --arg \"--verbose\" --arg \"--port=8080\"\n")
		os.Exit(1)
	}

	name := args[0]
	command := args[1]
	envVars := make(map[string]string)
	var cmdArgs []string
	enabled := true

	for i := 2; i < len(args); i++ {
		arg := args[i]

		if arg == "--env" && i+1 < len(args) {
			envArg := args[i+1]
			i++
			parts := strings.SplitN(envArg, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid env format '%s'. Use --env KEY=VALUE\n", envArg)
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "--env=") {
			envArg := strings.TrimPrefix(arg, "--env=")
			parts := strings.SplitN(envArg, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid env format '%s'. Use --env KEY=VALUE\n", envArg)
				os.Exit(1)
			}
		} else if arg == "--arg" && i+1 < len(args) {
			cmdArgs = append(cmdArgs, args[i+1])
			i++
		} else if strings.HasPrefix(arg, "--arg=") {
			cmdArgs = append(cmdArgs, strings.TrimPrefix(arg, "--arg="))
		} else if strings.HasPrefix(arg, "--enabled") {
			var enabledArg string
			if arg == "--enabled" && i+1 < len(args) {
				enabledArg = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--enabled=") {
				enabledArg = strings.TrimPrefix(arg, "--enabled=")
			}

			if enabledArg == "false" || enabledArg == "0" {
				enabled = false
			} else if enabledArg == "true" || enabledArg == "1" {
				enabled = true
			} else {
				fmt.Fprintf(os.Stderr, "Error: Invalid enabled value '%s'. Use true or false\n", enabledArg)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: Unknown option '%s'\n", arg)
			os.Exit(1)
		}
	}

	req := api.CreateMCPServerRequest{
		Name:    name,
		Type:    "stdio",
		Command: command,
		Args:    cmdArgs,
		Env:     envVars,
		Enabled: &enabled,
	}

	server, err := client.CreateMCPServer(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("MCP server '%s' added successfully\n", server.Name)
	fmt.Printf("  Type: %s\n", server.Type)
	fmt.Printf("  Command: %s\n", server.Command)
	if len(server.Args) > 0 {
		fmt.Printf("  Args: %v\n", server.Args)
	}
	fmt.Printf("  Enabled: %t\n", server.Enabled)
	if len(server.Env) > 0 {
		fmt.Printf("  Env:\n")
		for k, v := range server.Env {
			displayValue := v
			if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "secret") {
				if len(v) > 8 {
					displayValue = v[:4] + "..." + v[len(v)-4:]
				} else {
					displayValue = "***"
				}
			}
			fmt.Printf("    %s=%s\n", k, displayValue)
		}
	}
	fmt.Printf("  ID: %d\n", server.ID)
	fmt.Printf("  Created: %s\n", server.CreatedAt)
	fmt.Println()
	fmt.Println("Run 'diane reload' to reload the configuration and start the server.")
}

// ctlResolveContext determines which context to use.
func ctlResolveContext(client *api.Client, contextName string) string {
	contexts, err := client.ListContexts()
	if err != nil {
		if contextName != "" {
			return contextName
		}
		fmt.Println("Diane is not running, using default context: personal")
		return "personal"
	}

	if len(contexts) == 0 {
		if contextName != "" {
			return contextName
		}
		return "personal"
	}

	if contextName != "" {
		for _, c := range contexts {
			if c.Name == contextName {
				return contextName
			}
		}
		fmt.Fprintf(os.Stderr, "Context '%s' not found. Available contexts:\n", contextName)
		for _, c := range contexts {
			def := ""
			if c.IsDefault {
				def = " (default)"
			}
			fmt.Fprintf(os.Stderr, "  - %s%s\n", c.Name, def)
		}
		os.Exit(1)
	}

	if len(contexts) == 1 {
		return contexts[0].Name
	}

	fmt.Println("Multiple contexts available. Please specify one with --context:")
	for _, c := range contexts {
		def := ""
		if c.IsDefault {
			def = " (default)"
		}
		desc := ""
		if c.Description != "" {
			desc = " - " + c.Description
		}
		fmt.Printf("  - %s%s%s\n", c.Name, def, desc)
	}
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  diane mcp install opencode --context personal")
	os.Exit(1)
	return "" // unreachable
}

func ctlHandleMCPInstallOpenCode(client *api.Client, contextFlag string) {
	contextName := ctlResolveContext(client, contextFlag)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not get current directory: %v\n", err)
		os.Exit(1)
	}

	mcpKey := "diane-" + contextName
	mcpURL := "http://localhost:8765/mcp/sse?context=" + contextName
	dianeMCP := map[string]interface{}{
		"type":  "remote",
		"url":   mcpURL,
		"oauth": false,
	}

	configFiles := []string{"opencode.json", "opencode.jsonc"}
	var configPath string
	for _, f := range configFiles {
		candidate := filepath.Join(cwd, f)
		if _, err := os.Stat(candidate); err == nil {
			configPath = candidate
			break
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", configPath, err)
			os.Exit(1)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", configPath, err)
			os.Exit(1)
		}

		mcpSection, ok := config["mcp"].(map[string]interface{})
		if !ok {
			mcpSection = make(map[string]interface{})
		}

		if existing, exists := mcpSection[mcpKey]; exists {
			if existingMap, ok := existing.(map[string]interface{}); ok {
				existingURL, _ := existingMap["url"].(string)
				if existingURL == mcpURL {
					fmt.Printf("%s MCP config already up to date in %s\n", mcpKey, filepath.Base(configPath))
					return
				}
			}
		}

		upgraded := ctlDetectAndUpgradeOldConfig(mcpSection, mcpKey, dianeMCP)

		if !upgraded {
			if _, exists := mcpSection[mcpKey]; exists {
				fmt.Printf("Updating %s MCP config in %s\n", mcpKey, filepath.Base(configPath))
			} else {
				fmt.Printf("Adding %s MCP config to %s\n", mcpKey, filepath.Base(configPath))
			}
			mcpSection[mcpKey] = dianeMCP
		}

		config["mcp"] = mcpSection

		if _, hasSchema := config["$schema"]; !hasSchema {
			config["$schema"] = "https://opencode.ai/config.json"
		}

		output, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(configPath, append(output, '\n'), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", configPath, err)
			os.Exit(1)
		}
	} else {
		configPath = filepath.Join(cwd, "opencode.json")
		config := map[string]interface{}{
			"$schema": "https://opencode.ai/config.json",
			"mcp": map[string]interface{}{
				mcpKey: dianeMCP,
			},
		}

		output, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(configPath, append(output, '\n'), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing opencode.json: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created opencode.json with %s MCP config\n", mcpKey)
	}
}

func ctlDetectAndUpgradeOldConfig(mcpSection map[string]interface{}, newKey string, newConfig map[string]interface{}) bool {
	upgraded := false

	if old, exists := mcpSection["diane"]; exists {
		oldMap, ok := old.(map[string]interface{})
		if !ok {
			return false
		}

		isOldConfig := false
		oldType, _ := oldMap["type"].(string)
		oldURL, _ := oldMap["url"].(string)

		if oldType == "local" || oldType == "" {
			if _, hasCmd := oldMap["command"]; hasCmd {
				isOldConfig = true
			}
		}

		if oldType == "remote" || oldType == "http" {
			if oldURL != "" {
				if oldURL == "http://localhost:8765/mcp" ||
					oldURL == "http://localhost:8765/mcp/sse" ||
					(len(oldURL) > 0 && !ctlContainsContextParam(oldURL)) {
					isOldConfig = true
				}
			}
		}

		if isOldConfig {
			fmt.Printf("Upgrading old 'diane' config to '%s' in %s format\n", newKey, "SSE+context")
			delete(mcpSection, "diane")
			mcpSection[newKey] = newConfig
			upgraded = true
		}
	}

	return upgraded
}

func ctlContainsContextParam(rawURL string) bool {
	for _, s := range []string{"?context=", "&context="} {
		if len(rawURL) > len(s) {
			for i := 0; i <= len(rawURL)-len(s); i++ {
				if rawURL[i:i+len(s)] == s {
					return true
				}
			}
		}
	}
	return false
}

func ctlHandleAgentsCommand(client *api.Client, args []string) {
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(agents) == 0 {
		fmt.Println("No ACP agents configured.")
		fmt.Println("\nTo add an agent:")
		fmt.Println("  diane agent add <name> <url>")
		return
	}

	fmt.Printf("ACP Agents (%d):\n\n", len(agents))
	for _, agent := range agents {
		status := "enabled"
		if !agent.Enabled {
			status = "disabled"
		}
		fmt.Printf("  %-20s %s [%s]\n", agent.Name, agent.URL, status)
		if agent.Description != "" {
			fmt.Printf("                       %s\n", agent.Description)
		}
	}
}

func ctlHandleAgentCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: diane agent <subcommand> [args...]\n")
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  add <name> <url>    Add a new ACP agent\n")
		fmt.Fprintf(os.Stderr, "  remove <name>       Remove an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  enable <name>       Enable an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  disable <name>      Disable an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  test <name>         Test connectivity to an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  run <name> <prompt> Run a prompt against an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  info <name>         Show detailed info for an ACP agent\n")
		fmt.Fprintf(os.Stderr, "  logs [name]         Show agent communication logs\n")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent add <name> <url> [description]\n")
			os.Exit(1)
		}
		agent := acp.AgentConfig{
			Name:    args[1],
			URL:     args[2],
			Enabled: true,
			Type:    "acp",
		}
		if len(args) > 3 {
			agent.Description = args[3]
		}
		if err := client.AddAgent(agent); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' added successfully\n", agent.Name)

	case "remove", "rm", "delete":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent remove <name>\n")
			os.Exit(1)
		}
		if err := client.RemoveAgent(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' removed\n", args[1])

	case "enable":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent enable <name>\n")
			os.Exit(1)
		}
		if err := client.ToggleAgent(args[1], true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' enabled\n", args[1])

	case "disable":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent disable <name>\n")
			os.Exit(1)
		}
		if err := client.ToggleAgent(args[1], false); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' disabled\n", args[1])

	case "test":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent test <name>\n")
			os.Exit(1)
		}
		result, err := client.TestAgent(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))

	case "run":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent run <name> <prompt>\n")
			os.Exit(1)
		}
		longClient := api.NewClientWithTimeout(5 * time.Minute)
		run, err := longClient.RunAgent(args[1], args[2], "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		textOutput := run.GetTextOutput()
		if textOutput != "" {
			fmt.Println(textOutput)
		} else {
			output, _ := json.MarshalIndent(run, "", "  ")
			fmt.Println(string(output))
		}

	case "info", "get":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane agent info <name>\n")
			os.Exit(1)
		}
		agent, err := client.GetAgent(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(agent, "", "  ")
		fmt.Println(string(output))

	case "logs":
		agentName := ""
		limit := 50
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--limit", "-n":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &limit)
					i++
				}
			default:
				if agentName == "" {
					agentName = args[i]
				}
			}
		}
		logs, err := client.GetAgentLogs(agentName, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(logs) == 0 {
			if agentName != "" {
				fmt.Printf("No logs found for agent '%s'\n", agentName)
			} else {
				fmt.Println("No agent logs found")
			}
			return
		}
		for _, log := range logs {
			direction := "->"
			if log.Direction == "response" {
				direction = "<-"
			}
			ts := log.Timestamp.Format("15:04:05")
			duration := ""
			if log.DurationMs != nil {
				duration = fmt.Sprintf(" (%dms)", *log.DurationMs)
			}
			errStr := ""
			if log.Error != nil {
				errStr = fmt.Sprintf(" ERROR: %s", *log.Error)
			}
			fmt.Printf("%s %s %s %s%s%s\n", ts, direction, log.AgentName, log.MessageType, duration, errStr)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown agent subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func ctlHandleGalleryCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		ctlShowGallery(client, false)
		return
	}

	switch args[0] {
	case "list", "ls":
		featured := false
		if len(args) > 1 && args[1] == "--featured" {
			featured = true
		}
		ctlShowGallery(client, featured)

	case "featured":
		ctlShowGallery(client, true)

	case "info":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane gallery info <agent-id>\n")
			os.Exit(1)
		}
		info, err := client.GetGalleryAgent(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(output))

	case "install", "add":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane gallery install <agent-id> [--name <name>] [--workdir <path>] [--port <port>]\n")
			os.Exit(1)
		}

		agentID := args[1]
		customName := ""
		workDir := ""
		port := 0

		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--name", "-n":
				if i+1 < len(args) {
					customName = args[i+1]
					i++
				}
			case "--workdir", "-w", "--cwd":
				if i+1 < len(args) {
					workDir = args[i+1]
					i++
				}
			case "--port", "-p":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &port)
					i++
				}
			}
		}

		info, err := client.GetGalleryAgent(agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Installing %s (%s)...\n", info.Name, info.Version)

		finalName := agentID
		if customName != "" {
			finalName = customName
		} else if workDir != "" {
			finalName = fmt.Sprintf("%s@%s", agentID, filepath.Base(workDir))
		}

		if err := client.InstallGalleryAgentWithOptions(agentID, finalName, workDir, port); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Agent '%s' configured\n", finalName)

		if info.InstallCmd != "" {
			fmt.Printf("\nTo install the agent binary/package, run:\n  %s\n", info.InstallCmd)
		}

	case "refresh":
		fmt.Println("Refreshing agent registry...")
		if err := client.RefreshGallery(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Registry refreshed")

	default:
		info, err := client.GetGalleryAgent(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown gallery command or agent: %s\n", args[0])
			fmt.Fprintf(os.Stderr, "\nUsage: diane gallery <subcommand>\n")
			fmt.Fprintf(os.Stderr, "  list [--featured]  List available agents\n")
			fmt.Fprintf(os.Stderr, "  featured           List featured agents\n")
			fmt.Fprintf(os.Stderr, "  info <id>          Show agent install info\n")
			fmt.Fprintf(os.Stderr, "  install <id>       Configure an agent from the gallery\n")
			fmt.Fprintf(os.Stderr, "  refresh            Refresh the agent registry\n")
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(output))
	}
}

func ctlShowGallery(client *api.Client, featured bool) {
	entries, err := client.ListGallery(featured)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No agents available in gallery.")
		return
	}

	title := "Agent Gallery"
	if featured {
		title = "Featured Agents"
	}

	fmt.Printf("%s (%d):\n\n", title, len(entries))

	for _, entry := range entries {
		star := " "
		if entry.Featured {
			star = "*"
		}
		fmt.Printf("  %s %-20s %s\n", star, entry.ID, entry.Name)
		if entry.Description != "" {
			fmt.Printf("                         %s\n", ctlTruncate(entry.Description, 60))
		}
		fmt.Printf("                         Provider: %s | Install: %s\n", entry.Provider, entry.InstallType)
		fmt.Println()
	}

	fmt.Println("To install an agent:")
	fmt.Println("  diane gallery install <agent-id>")
	fmt.Println("")
	fmt.Println("To get install info:")
	fmt.Println("  diane gallery info <agent-id>")
}

func ctlTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func ctlHandleAuthCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		servers, err := client.ListOAuthServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(servers) == 0 {
			fmt.Println("No MCP servers with OAuth configured.")
			fmt.Println("\nTo configure OAuth, add an 'oauth' section to your server in ~/.diane/mcp-servers.json")
			return
		}

		fmt.Printf("OAuth-enabled MCP Servers (%d):\n\n", len(servers))
		for _, server := range servers {
			status := "not authenticated"
			if server.Authenticated {
				status = "authenticated"
			}
			provider := ""
			if server.Provider != "" {
				provider = fmt.Sprintf(" (%s)", server.Provider)
			}
			fmt.Printf("  %-20s %s%s [%s]\n", server.Name, status, provider, server.Status)
		}
		fmt.Println("\nTo authenticate:")
		fmt.Println("  diane auth login <server-name>")
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "login":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane auth login <server-name>\n")
			os.Exit(1)
		}
		serverName := args[1]

		fmt.Printf("Starting OAuth login for %s...\n\n", serverName)

		deviceInfo, err := client.StartOAuthLogin(serverName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Please visit: %s\n", deviceInfo.VerificationURI)
		fmt.Printf("Enter code:   %s\n\n", deviceInfo.UserCode)
		fmt.Println("Waiting for authorization...")

		longClient := api.NewClientWithTimeout(10 * time.Minute)

		if err := longClient.PollOAuthToken(serverName, deviceInfo.DeviceCode, deviceInfo.Interval); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nSuccessfully authenticated %s!\n", serverName)
		fmt.Println("The MCP server has been restarted with the new credentials.")

	case "status":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane auth status <server-name>\n")
			os.Exit(1)
		}
		serverName := args[1]

		status, err := client.GetOAuthStatus(serverName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(output))

	case "logout":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane auth logout <server-name>\n")
			os.Exit(1)
		}
		serverName := args[1]

		if err := client.LogoutOAuth(serverName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Logged out from %s\n", serverName)

	default:
		serverName := subcommand
		status, err := client.GetOAuthStatus(serverName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nUsage: diane auth <subcommand>\n")
			fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
			fmt.Fprintf(os.Stderr, "  login <server>   Start OAuth login for a server\n")
			fmt.Fprintf(os.Stderr, "  status <server>  Show OAuth status for a server\n")
			fmt.Fprintf(os.Stderr, "  logout <server>  Remove OAuth credentials for a server\n")
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(output))
	}
}

func ctlHandleInfoCommand(client *api.Client) {
	home, _ := os.UserHomeDir()
	dianeBin := filepath.Join(home, ".diane", "bin", "diane")

	status := "not running"
	httpStatus := "unavailable"
	toolCount := 0

	if err := client.Health(); err == nil {
		status = "running"
		httpStatus = "http://localhost:8765"
		if s, err := client.GetStatus(); err == nil {
			toolCount = s.TotalTools
		}
	}

	fmt.Println(`
+------------------------------------------------------------------------------+
|                             DIANE MCP SERVER                                 |
+------------------------------------------------------------------------------+`)

	fmt.Printf(`
  Status:     %s
  HTTP:       %s
  Tools:      %d available

`, status, httpStatus, toolCount)

	fmt.Println(`==============================================================================
  CONNECTING TO DIANE
==============================================================================`)

	fmt.Print(`
  -- OpenCode ----------------------------------------------------------------

  Add to your opencode.json:

    {
      "$schema": "https://opencode.ai/config.json",
      "mcp": {
        "diane-personal": {
          "type": "remote",
          "url": "http://localhost:8765/mcp/sse?context=personal",
          "oauth": false
        }
      }
    }

  Or install automatically with:

    diane mcp install opencode
`)

	fmt.Printf(`  -- Claude Desktop ----------------------------------------------------------

  Add to claude_desktop_config.json:

  macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
  Linux: ~/.config/claude/claude_desktop_config.json

    {
      "mcpServers": {
        "diane": {
          "command": "%s"
        }
      }
    }

`, dianeBin)

	fmt.Printf(`  -- Cursor / Windsurf / Continue ---------------------------------------------

  Add to your MCP settings:

    {
      "mcpServers": {
        "diane": {
          "command": "%s"
        }
      }
    }

`, dianeBin)

	fmt.Print(`  -- HTTP / Network Clients ---------------------------------------------------

  Diane exposes an HTTP Streamable MCP endpoint when running:

    URL:     http://localhost:8765/mcp
    SSE:     http://localhost:8765/mcp/sse
    Health:  http://localhost:8765/health

  Example configuration:

    {
      "mcpServers": {
        "diane": {
          "type": "http",
          "url": "http://localhost:8765/mcp"
        }
      }
    }
`)

	fmt.Print(`==============================================================================
  TESTING CONNECTION
==============================================================================

  Test HTTP endpoint:

    curl http://localhost:8765/health

  Initialize MCP session:

    curl -X POST http://localhost:8765/mcp \
      -H "Content-Type: application/json" \
      -H "Accept: application/json" \
      -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'

==============================================================================
  MORE INFO
==============================================================================

  Documentation:    ~/.diane/MCP.md
  Database:         ~/.diane/cron.db
  Logs:             ~/.diane/server.log

  Commands:
    diane status        Full status with all MCP servers
    diane mcp-servers   List connected MCP servers
    diane agents        List configured ACP agents
    diane gallery       Browse installable agents
`)
}

func ctlHandleDoctorCommand(client *api.Client) {
	report, err := client.Doctor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nDiane may not be running. Try: diane health\n")
		os.Exit(1)
	}

	fmt.Println()
	for _, check := range report.Checks {
		var icon string
		switch check.Status {
		case "ok":
			icon = "  ok"
		case "warn":
			icon = "warn"
		case "fail":
			icon = "FAIL"
		}
		fmt.Printf("  [%s]  %-18s %s\n", icon, check.Name, check.Message)
	}
	fmt.Println()

	if report.Healthy {
		fmt.Println("  All checks passed.")
	} else {
		fmt.Println("  Some checks failed.")
		os.Exit(1)
	}
}

func ctlPrintUsage() {
	fmt.Printf(`diane %s - Diane MCP server and control utility

Usage:
  diane                     Show this help (or start MCP stdio mode when piped)
  diane serve               Start as daemon (HTTP + Unix socket)
  diane <command> [args]    Run a control command

Server Commands:
  serve              Start Diane as a background daemon (HTTP + Unix socket)

Control Commands:
  info               Show connection guide for AI tools
  status             Show full Diane status
  health             Check if Diane is running
  doctor             Run diagnostic checks (daemon, MCP, SSE, database)
  mcp-servers        List all MCP servers and their status
  reload             Reload MCP configuration
  restart <name>     Restart a specific MCP server
  version            Show version

MCP Commands:
  mcp add <name> <url>          Add a new remote MCP server (http/sse)
  mcp add-stdio <name> <cmd>    Add a new stdio MCP server
                                Options:
                                  --env <KEY>=<VALUE>   Set env var (repeatable)
                                  --arg <value>         Add argument (repeatable)
  mcp install opencode          Install Diane MCP config into OpenCode project
                                Options:
                                  --context <name>  Context to use (default: auto-detect)

OAuth Commands:
  auth                        List MCP servers with OAuth configured
  auth login <server>         Authenticate with an MCP server
  auth status <server>        Show OAuth status for a server
  auth logout <server>        Remove OAuth credentials

ACP Agent Commands:
  agents                      List all configured ACP agents
  agent add <name> <url>      Add a new ACP agent
  agent remove <name>         Remove an ACP agent
  agent enable <name>         Enable an ACP agent
  agent disable <name>        Disable an ACP agent
  agent test <name>           Test connectivity to an ACP agent
  agent run <name> <prompt>   Run a prompt against an ACP agent
  agent info <name>           Show detailed info for an ACP agent
  agent logs [name] [-n N]    Show agent communication logs

Agent Gallery (one-click install):
  gallery                     List all available agents
  gallery featured            List featured agents
  gallery info <id>           Show install info for an agent
  gallery install <id> [opts] Configure an agent from the gallery
                              Options:
                                --name <name>     Custom agent name
                                --workdir <path>  Working directory for the agent
  gallery refresh             Refresh the agent registry

Examples:
  diane                                                # Show help
  diane serve                                          # Start as daemon
  diane info                                           # Show connection guide
  diane mcp install opencode                           # Install MCP config in OpenCode project
  diane mcp install opencode --context work            # Install with specific context
  diane auth login github                              # Authenticate with GitHub MCP
  diane gallery                                        # Browse available agents
  diane gallery install gemini                         # Install Gemini CLI
  diane gallery install gemini --workdir ~/myproject   # Install for a specific project
  diane gallery install gemini --name gemini-work      # Install with custom name
  diane agent run gemini "what is 2+2?"                # Run a prompt
  diane agent logs opencode                            # View logs for opencode agent
  diane agents                                         # List all agents
`, Version)
}
