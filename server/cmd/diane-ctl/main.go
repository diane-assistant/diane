package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
	"github.com/diane-assistant/diane/internal/api"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	client := api.NewClient()

	switch os.Args[1] {
	case "info":
		handleInfoCommand(client)

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

	case "mcp-servers":
		servers, err := client.GetMCPServers()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		output, _ := json.MarshalIndent(servers, "", "  ")
		fmt.Println(string(output))

	case "reload":
		if err := client.ReloadConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration reloaded")

	case "restart":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl restart <server-name>\n")
			os.Exit(1)
		}
		serverName := os.Args[2]
		if err := client.RestartMCPServer(serverName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Server '%s' restarted\n", serverName)

	// ACP Agent commands
	case "agents":
		handleAgentsCommand(client, os.Args[2:])

	case "agent":
		handleAgentCommand(client, os.Args[2:])

	// Gallery commands
	case "gallery":
		handleGalleryCommand(client, os.Args[2:])

	// Auth commands
	case "auth":
		handleAuthCommand(client, os.Args[2:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handleAgentsCommand(client *api.Client, args []string) {
	agents, err := client.ListAgents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(agents) == 0 {
		fmt.Println("No ACP agents configured.")
		fmt.Println("\nTo add an agent:")
		fmt.Println("  diane-ctl agent add <name> <url>")
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

func handleAgentCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent <subcommand> [args...]\n")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent add <name> <url> [description]\n")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent remove <name>\n")
			os.Exit(1)
		}
		if err := client.RemoveAgent(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' removed\n", args[1])

	case "enable":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent enable <name>\n")
			os.Exit(1)
		}
		if err := client.ToggleAgent(args[1], true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' enabled\n", args[1])

	case "disable":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent disable <name>\n")
			os.Exit(1)
		}
		if err := client.ToggleAgent(args[1], false); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Agent '%s' disabled\n", args[1])

	case "test":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent test <name>\n")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent run <name> <prompt>\n")
			os.Exit(1)
		}
		// Use longer timeout for agent runs (5 minutes)
		longClient := api.NewClientWithTimeout(5 * time.Minute)
		run, err := longClient.RunAgent(args[1], args[2], "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// Print the text output
		textOutput := run.GetTextOutput()
		if textOutput != "" {
			fmt.Println(textOutput)
		} else {
			output, _ := json.MarshalIndent(run, "", "  ")
			fmt.Println(string(output))
		}

	case "info", "get":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl agent info <name>\n")
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
		// Parse args: logs [name] [--limit N]
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
			direction := "→"
			if log.Direction == "response" {
				direction = "←"
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

func handleGalleryCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		// Default: list all gallery entries
		showGallery(client, false)
		return
	}

	switch args[0] {
	case "list", "ls":
		featured := false
		if len(args) > 1 && args[1] == "--featured" {
			featured = true
		}
		showGallery(client, featured)

	case "featured":
		showGallery(client, true)

	case "info":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl gallery info <agent-id>\n")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl gallery install <agent-id> [--name <name>] [--workdir <path>] [--port <port>]\n")
			os.Exit(1)
		}

		agentID := args[1]
		customName := ""
		workDir := ""
		port := 0

		// Parse optional flags
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

		// Get info first
		info, err := client.GetGalleryAgent(agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Installing %s (%s)...\n", info.Name, info.Version)

		// Build the agent name
		finalName := agentID
		if customName != "" {
			finalName = customName
		} else if workDir != "" {
			// Use type@folder format for workdir-specific agents
			finalName = fmt.Sprintf("%s@%s", agentID, filepath.Base(workDir))
		}

		if err := client.InstallGalleryAgentWithOptions(agentID, finalName, workDir, port); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Agent '%s' configured\n", finalName)

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
		// Treat as agent ID for info
		info, err := client.GetGalleryAgent(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown gallery command or agent: %s\n", args[0])
			fmt.Fprintf(os.Stderr, "\nUsage: diane-ctl gallery <subcommand>\n")
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

func showGallery(client *api.Client, featured bool) {
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
			star = "★"
		}
		fmt.Printf("  %s %-20s %s\n", star, entry.ID, entry.Name)
		if entry.Description != "" {
			fmt.Printf("                         %s\n", truncate(entry.Description, 60))
		}
		fmt.Printf("                         Provider: %s | Install: %s\n", entry.Provider, entry.InstallType)
		fmt.Println()
	}

	fmt.Println("To install an agent:")
	fmt.Println("  diane-ctl gallery install <agent-id>")
	fmt.Println("")
	fmt.Println("To get install info:")
	fmt.Println("  diane-ctl gallery info <agent-id>")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func handleAuthCommand(client *api.Client, args []string) {
	if len(args) < 1 {
		// List all OAuth servers
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
		fmt.Println("  diane-ctl auth login <server-name>")
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "login":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl auth login <server-name>\n")
			os.Exit(1)
		}
		serverName := args[1]

		fmt.Printf("Starting OAuth login for %s...\n\n", serverName)

		// Start the device flow
		deviceInfo, err := client.StartOAuthLogin(serverName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Display instructions to user
		fmt.Printf("Please visit: %s\n", deviceInfo.VerificationURI)
		fmt.Printf("Enter code:   %s\n\n", deviceInfo.UserCode)
		fmt.Println("Waiting for authorization...")

		// Use a longer timeout client for polling
		longClient := api.NewClientWithTimeout(10 * time.Minute)

		// Poll for token
		if err := longClient.PollOAuthToken(serverName, deviceInfo.DeviceCode, deviceInfo.Interval); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nSuccessfully authenticated %s!\n", serverName)
		fmt.Println("The MCP server has been restarted with the new credentials.")

	case "status":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl auth status <server-name>\n")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl auth logout <server-name>\n")
			os.Exit(1)
		}
		serverName := args[1]

		if err := client.LogoutOAuth(serverName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Logged out from %s\n", serverName)

	default:
		// Treat as server name for status
		serverName := subcommand
		status, err := client.GetOAuthStatus(serverName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "\nUsage: diane-ctl auth <subcommand>\n")
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

func handleInfoCommand(client *api.Client) {
	home, _ := os.UserHomeDir()
	dianeBin := filepath.Join(home, ".diane", "bin", "diane")

	// Check if Diane is running
	status := "not running"
	httpStatus := "unavailable"
	toolCount := 0

	if err := client.Health(); err == nil {
		status = "running"
		httpStatus = "http://localhost:8765"
		// Get tool count
		if s, err := client.GetStatus(); err == nil {
			toolCount = s.TotalTools
		}
	}

	fmt.Println(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                             DIANE MCP SERVER                                 ║
╚══════════════════════════════════════════════════════════════════════════════╝`)

	fmt.Printf(`
  Status:     %s
  HTTP:       %s
  Tools:      %d available

`, status, httpStatus, toolCount)

	fmt.Println(`══════════════════════════════════════════════════════════════════════════════
  CONNECTING TO DIANE
══════════════════════════════════════════════════════════════════════════════`)

	fmt.Printf(`
  ── OpenCode ─────────────────────────────────────────────────────────────────

  Add to your opencode.json:

    {
      "mcp": {
        "diane": {
          "command": ["%s"]
        }
      }
    }

`, dianeBin)

	fmt.Printf(`  ── Claude Desktop ───────────────────────────────────────────────────────────

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

	fmt.Printf(`  ── Cursor / Windsurf / Continue ─────────────────────────────────────────────

  Add to your MCP settings:

    {
      "mcpServers": {
        "diane": {
          "command": "%s"
        }
      }
    }

`, dianeBin)

	fmt.Println(`  ── HTTP / Network Clients ───────────────────────────────────────────────────

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

	fmt.Println(`══════════════════════════════════════════════════════════════════════════════
  TESTING CONNECTION
══════════════════════════════════════════════════════════════════════════════

  Test HTTP endpoint:

    curl http://localhost:8765/health

  Initialize MCP session:

    curl -X POST http://localhost:8765/mcp \
      -H "Content-Type: application/json" \
      -H "Accept: application/json" \
      -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'

══════════════════════════════════════════════════════════════════════════════
  MORE INFO
══════════════════════════════════════════════════════════════════════════════

  Documentation:    ~/.diane/MCP.md
  MCP Servers:      ~/.diane/mcp-servers.json
  Logs:             ~/.diane/server.log

  Commands:
    diane-ctl status        Full status with all MCP servers
    diane-ctl mcp-servers   List connected MCP servers
    diane-ctl agents        List configured ACP agents
    diane-ctl gallery       Browse installable agents
`)
}

func printUsage() {
	fmt.Println(`diane-ctl - Diane control utility

Usage:
  diane-ctl <command> [arguments]

Commands:
  info           Show connection guide for AI tools
  status         Show full Diane status
  health         Check if Diane is running
  mcp-servers    List all MCP servers and their status
  reload         Reload MCP configuration
  restart <name> Restart a specific MCP server

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
  diane-ctl info                                              # Show connection guide
  diane-ctl auth login github                                 # Authenticate with GitHub MCP
  diane-ctl gallery                                           # Browse available agents
  diane-ctl gallery install gemini                            # Install Gemini CLI
  diane-ctl gallery install gemini --workdir ~/myproject      # Install for a specific project
  diane-ctl gallery install gemini --name gemini-work         # Install with custom name
  diane-ctl agent run gemini "what is 2+2?"                   # Run a prompt
  diane-ctl agent logs opencode                               # View logs for opencode agent
  diane-ctl agents`)
}
