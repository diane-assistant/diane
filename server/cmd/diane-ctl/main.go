package main

import (
	"encoding/json"
	"fmt"
	"os"

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
		run, err := client.RunAgent(args[1], args[2], "")
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
			fmt.Fprintf(os.Stderr, "Usage: diane-ctl gallery install <agent-id>\n")
			os.Exit(1)
		}

		// Get info first
		info, err := client.GetGalleryAgent(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Installing %s (%s)...\n", info.Name, info.Version)

		if err := client.InstallGalleryAgent(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Agent '%s' configured\n", args[1])

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

func printUsage() {
	fmt.Println(`diane-ctl - Diane control utility

Usage:
  diane-ctl <command> [arguments]

Commands:
  status         Show full Diane status
  health         Check if Diane is running
  mcp-servers    List all MCP servers and their status
  reload         Reload MCP configuration
  restart <name> Restart a specific MCP server

ACP Agent Commands:
  agents                      List all configured ACP agents
  agent add <name> <url>      Add a new ACP agent
  agent remove <name>         Remove an ACP agent
  agent enable <name>         Enable an ACP agent
  agent disable <name>        Disable an ACP agent
  agent test <name>           Test connectivity to an ACP agent
  agent run <name> <prompt>   Run a prompt against an ACP agent
  agent info <name>           Show detailed info for an ACP agent

Agent Gallery (one-click install):
  gallery                     List all available agents
  gallery featured            List featured agents
  gallery info <id>           Show install info for an agent
  gallery install <id>        Configure an agent from the gallery
  gallery refresh             Refresh the agent registry

Examples:
  diane-ctl gallery                          # Browse available agents
  diane-ctl gallery install claude-code-acp  # Install Claude Code
  diane-ctl gallery install gemini           # Install Gemini CLI
  diane-ctl agent add beeai http://localhost:8000 "BeeAI Platform"
  diane-ctl agents`)
}
