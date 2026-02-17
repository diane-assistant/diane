package cli

import (
	"fmt"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newToolsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List available tools across MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := client.GetMCPServers()
			if err != nil {
				PrintError(fmt.Sprintf("Failed to get MCP servers: %v", err))
				return nil
			}

			if tryJSON(cmd, servers) {
				return nil
			}

			serverFilter, _ := cmd.Flags().GetString("server")

			totalTools := 0
			var filtered []api.MCPServerStatus
			for _, srv := range servers {
				if serverFilter != "" && srv.Name != serverFilter {
					continue
				}
				filtered = append(filtered, srv)
				totalTools += srv.ToolCount
			}

			if len(filtered) == 0 {
				if serverFilter != "" {
					PrintWarning(fmt.Sprintf("No server found matching '%s'", serverFilter))
				} else {
					PrintWarning("No MCP servers configured")
				}
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render(fmt.Sprintf("Tools (%d total)", totalTools)))

			headers := []string{"Server", "Type", "Tools"}
			var rows [][]string

			for _, srv := range filtered {
				serverType := "stdio"
				if srv.Builtin {
					serverType = "builtin"
				}
				badge := GetTypeBadge(serverType)

				rows = append(rows, []string{
					srv.Name,
					badge,
					fmt.Sprintf("%d", srv.ToolCount),
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().String("server", "", "Filter by server name")

	return cmd
}

func newPromptsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "List available prompts across MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := client.GetMCPServers()
			if err != nil {
				PrintError(fmt.Sprintf("Failed to get MCP servers: %v", err))
				return nil
			}

			if tryJSON(cmd, servers) {
				return nil
			}

			serverFilter, _ := cmd.Flags().GetString("server")

			totalPrompts := 0
			var filtered []api.MCPServerStatus
			for _, srv := range servers {
				if serverFilter != "" && srv.Name != serverFilter {
					continue
				}
				if srv.PromptCount > 0 || serverFilter != "" {
					filtered = append(filtered, srv)
				}
				totalPrompts += srv.PromptCount
			}

			if totalPrompts == 0 {
				PrintWarning("No prompts available from any MCP server")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render(fmt.Sprintf("Prompts (%d total)", totalPrompts)))

			headers := []string{"Server", "Type", "Prompts"}
			var rows [][]string

			for _, srv := range filtered {
				serverType := "stdio"
				if srv.Builtin {
					serverType = "builtin"
				}
				badge := GetTypeBadge(serverType)

				rows = append(rows, []string{
					srv.Name,
					badge,
					fmt.Sprintf("%d", srv.PromptCount),
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().String("server", "", "Filter by server name")

	return cmd
}

func newResourcesCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "List available resources across MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			servers, err := client.GetMCPServers()
			if err != nil {
				PrintError(fmt.Sprintf("Failed to get MCP servers: %v", err))
				return nil
			}

			if tryJSON(cmd, servers) {
				return nil
			}

			serverFilter, _ := cmd.Flags().GetString("server")

			totalResources := 0
			var filtered []api.MCPServerStatus
			for _, srv := range servers {
				if serverFilter != "" && srv.Name != serverFilter {
					continue
				}
				if srv.ResourceCount > 0 || serverFilter != "" {
					filtered = append(filtered, srv)
				}
				totalResources += srv.ResourceCount
			}

			if totalResources == 0 {
				PrintWarning("No resources available from any MCP server")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render(fmt.Sprintf("Resources (%d total)", totalResources)))

			headers := []string{"Server", "Type", "Resources"}
			var rows [][]string

			for _, srv := range filtered {
				serverType := "stdio"
				if srv.Builtin {
					serverType = "builtin"
				}
				badge := GetTypeBadge(serverType)

				rows = append(rows, []string{
					srv.Name,
					badge,
					fmt.Sprintf("%d", srv.ResourceCount),
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().String("server", "", "Filter by server name")

	return cmd
}
