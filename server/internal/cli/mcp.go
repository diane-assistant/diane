package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newMCPServersCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-servers",
		Short: "List all MCP servers with status",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := client.GetMCPServers()
			if err != nil {
				PrintError(fmt.Sprintf("Failed to get MCP servers: %v", err))
				return nil
			}

			if tryJSON(cmd, servers) {
				return nil
			}

			configs, err := client.GetMCPServerConfigs()
			if err != nil {
				// Fall back to status-only view if configs aren't available
				configs = nil
			}

			// Build a map of config by name for type lookup
			configByName := make(map[string]api.MCPServerResponse)
			for _, c := range configs {
				configByName[c.Name] = c
			}

			if len(servers) == 0 {
				PrintWarning("No MCP servers configured")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render("MCP Servers"))

			headers := []string{"", "Name", "Type", "Status", "Tools", "Error"}
			var rows [][]string

			for _, srv := range servers {
				dot := GetStatusDot(srv.Connected, srv.Error != "")

				// Determine server type from config, fall back to inference
				serverType := "stdio"
				if srv.Builtin {
					serverType = "builtin"
				}
				if cfg, ok := configByName[srv.Name]; ok {
					serverType = cfg.Type
				}
				badge := GetTypeBadge(serverType)

				status := "disconnected"
				if !srv.Enabled {
					status = "disabled"
				} else if srv.Connected {
					status = "connected"
				}

				toolInfo := fmt.Sprintf("%d", srv.ToolCount)
				if srv.PromptCount > 0 || srv.ResourceCount > 0 {
					toolInfo = fmt.Sprintf("%d tools, %d prompts, %d resources",
						srv.ToolCount, srv.PromptCount, srv.ResourceCount)
				}

				errStr := ""
				if srv.Error != "" {
					errStr = srv.Error
				}

				rows = append(rows, []string{dot, srv.Name, badge, status, toolInfo, errStr})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}
}

func newMCPCmd(client *api.Client) *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP server configurations",
	}

	mcpCmd.AddCommand(newMCPAddCmd(client))
	mcpCmd.AddCommand(newMCPAddStdioCmd(client))
	mcpCmd.AddCommand(newMCPEditCmd(client))
	mcpCmd.AddCommand(newMCPDeleteCmd(client))
	mcpCmd.AddCommand(newMCPInstallCmd(client))

	return mcpCmd
}

func newMCPAddCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add an HTTP or SSE MCP server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			url := args[1]

			serverType, _ := cmd.Flags().GetString("type")
			headers, _ := cmd.Flags().GetStringSlice("header")
			enabled, _ := cmd.Flags().GetBool("enabled")

			headerMap := make(map[string]string)
			for _, h := range headers {
				parts := strings.SplitN(h, "=", 2)
				if len(parts) == 2 {
					headerMap[parts[0]] = parts[1]
				} else {
					PrintWarning(fmt.Sprintf("Ignoring malformed header: %s (expected key=value)", h))
				}
			}

			req := api.CreateMCPServerRequest{
				Name:    name,
				Type:    serverType,
				URL:     url,
				Enabled: &enabled,
			}
			if len(headerMap) > 0 {
				req.Headers = headerMap
			}

			server, err := client.CreateMCPServer(req)
			if err != nil {
				PrintError(fmt.Sprintf("Failed to add server: %v", err))
				return nil
			}

			PrintSuccess(fmt.Sprintf("Added %s server '%s' (id: %d)", server.Type, server.Name, server.ID))
			return nil
		},
	}

	cmd.Flags().String("type", "http", "Server type (http or sse)")
	cmd.Flags().StringSlice("header", nil, "HTTP header as key=value (repeatable)")
	cmd.Flags().Bool("enabled", true, "Enable the server immediately")

	return cmd
}

func newMCPAddStdioCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-stdio <name> <command>",
		Short: "Add a stdio MCP server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			command := args[1]

			cmdArgs, _ := cmd.Flags().GetStringSlice("arg")
			envSlice, _ := cmd.Flags().GetStringSlice("env")
			enabled, _ := cmd.Flags().GetBool("enabled")

			envMap := make(map[string]string)
			for _, e := range envSlice {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				} else {
					PrintWarning(fmt.Sprintf("Ignoring malformed env var: %s (expected KEY=VALUE)", e))
				}
			}

			req := api.CreateMCPServerRequest{
				Name:    name,
				Type:    "stdio",
				Command: command,
				Enabled: &enabled,
			}
			if len(cmdArgs) > 0 {
				req.Args = cmdArgs
			}
			if len(envMap) > 0 {
				req.Env = envMap
			}

			server, err := client.CreateMCPServer(req)
			if err != nil {
				PrintError(fmt.Sprintf("Failed to add server: %v", err))
				return nil
			}

			PrintSuccess(fmt.Sprintf("Added stdio server '%s' (id: %d)", server.Name, server.ID))
			return nil
		},
	}

	cmd.Flags().StringSlice("arg", nil, "Command argument (repeatable)")
	cmd.Flags().StringSlice("env", nil, "Environment variable as KEY=VALUE (repeatable)")
	cmd.Flags().Bool("enabled", true, "Enable the server immediately")

	return cmd
}

func newMCPEditCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing MCP server configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server ID: %w", err)
			}

			req := api.UpdateMCPServerRequest{}
			hasChanges := false

			name, _ := cmd.Flags().GetString("name")
			if name != "" {
				req.Name = &name
				hasChanges = true
			}

			enabledStr, _ := cmd.Flags().GetString("enabled")
			if enabledStr != "" {
				enabled := enabledStr == "true"
				req.Enabled = &enabled
				hasChanges = true
			}

			url, _ := cmd.Flags().GetString("url")
			if url != "" {
				req.URL = &url
				hasChanges = true
			}

			command, _ := cmd.Flags().GetString("command")
			if command != "" {
				req.Command = &command
				hasChanges = true
			}

			if !hasChanges {
				PrintWarning("No changes specified")
				return nil
			}

			if _, err := client.UpdateMCPServerConfig(id, req); err != nil {
				return fmt.Errorf("failed to update server: %w", err)
			}

			PrintSuccess("Server updated")
			return nil
		},
	}

	cmd.Flags().String("name", "", "Update server name")
	cmd.Flags().String("enabled", "", "Enable or disable (true/false)")
	cmd.Flags().String("url", "", "Update server URL")
	cmd.Flags().String("command", "", "Update command")

	return cmd
}

func newMCPDeleteCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an MCP server configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server ID: %w", err)
			}

			if err := client.DeleteMCPServerConfig(id); err != nil {
				return fmt.Errorf("failed to delete server: %w", err)
			}

			PrintSuccess("Server deleted")
			return nil
		},
	}
}

func newMCPInstallCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <target>",
		Short: "Install MCP configuration into a target tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			switch target {
			case "opencode":
				PrintWarning("Use 'diane mcp install opencode' from the old CLI for now")
			default:
				PrintError(fmt.Sprintf("Unknown install target: %s (supported: opencode)", target))
			}
			return nil
		},
	}

	cmd.Flags().String("context", "", "Context to install into")

	return cmd
}
