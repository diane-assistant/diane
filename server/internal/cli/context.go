package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newContextCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage contexts (server/tool groupings)",
		Long:  titleStyle.Render("Contexts") + "\n  Manage contexts that control which MCP servers and tools are available.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to listing contexts
			return listContexts(cmd, client)
		},
	}

	// list subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listContexts(cmd, client)
		},
	}

	// create subcommand
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			description, _ := cmd.Flags().GetString("description")

			ctx, err := client.CreateContext(name, description)
			if err != nil {
				return fmt.Errorf("failed to create context: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Context '%s' created (id: %d)", ctx.Name, ctx.ID))
			return nil
		},
	}
	createCmd.Flags().String("description", "", "Description for the context")

	// delete subcommand
	deleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.DeleteContext(name); err != nil {
				return fmt.Errorf("failed to delete context: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Context '%s' deleted", name))
			return nil
		},
	}

	// set-default subcommand
	setDefaultCmd := &cobra.Command{
		Use:   "set-default <name>",
		Short: "Set the default context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.SetDefaultContext(name); err != nil {
				return fmt.Errorf("failed to set default context: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Context '%s' is now default", name))
			return nil
		},
	}

	// info subcommand
	infoCmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show detailed info about a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			detail, err := client.GetContextDetail(name)
			if err != nil {
				return fmt.Errorf("failed to get context info: %w", err)
			}

			if tryJSON(cmd, detail) {
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render("Context: "+detail.Context.Name))

			if detail.Context.Description != "" {
				fmt.Printf("  Description: %s\n", detail.Context.Description)
			}

			status := "Active"
			if detail.Context.IsDefault {
				status = "Default"
			}
			fmt.Printf("  Status:      %s\n", status)
			fmt.Println()

			// Summary table
			fmt.Println(headerStyle.Render("Summary"))
			fmt.Printf("  Servers: %d enabled / %d total\n", detail.Summary.ServersEnabled, detail.Summary.ServersTotal)
			fmt.Printf("  Tools:   %d active / %d total\n", detail.Summary.ToolsActive, detail.Summary.ToolsTotal)
			fmt.Println()

			// Servers table
			if len(detail.Servers) > 0 {
				fmt.Println(headerStyle.Render("Servers"))
				headers := []string{"Server", "Type", "Status", "Tools"}
				var rows [][]string
				for _, s := range detail.Servers {
					status := "disabled"
					if s.Enabled {
						status = "enabled"
					}
					rows = append(rows, []string{
						s.Name,
						s.Type,
						status,
						fmt.Sprintf("%d/%d", s.ToolsActive, s.ToolsTotal),
					})
				}
				RenderTable(headers, rows)
				fmt.Println()
			}

			return nil
		},
	}

	// servers subcommand
	serversCmd := &cobra.Command{
		Use:   "servers <name>",
		Short: "List servers available to a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			servers, err := client.GetAvailableServersForContext(name)
			if err != nil {
				return fmt.Errorf("failed to get available servers: %w", err)
			}

			if tryJSON(cmd, servers) {
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render("Servers for context: "+name))

			headers := []string{"Server", "Type", "Tools", "In Context"}
			var rows [][]string
			for _, s := range servers {
				inContext := ""
				if s.InContext {
					inContext = "yes"
				}
				sType := "stdio/http"
				if s.Builtin {
					sType = "builtin"
				}
				rows = append(rows, []string{
					s.Name,
					sType,
					fmt.Sprintf("%d", s.ToolCount),
					inContext,
				})
			}
			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}

	// tools subcommand (sync)
	syncCmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Sync tools from running servers into context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			count, err := client.SyncContextTools(name)
			if err != nil {
				return fmt.Errorf("failed to sync tools: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Synced %d new tools to context '%s'", count, name))
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)
	cmd.AddCommand(deleteCmd)
	cmd.AddCommand(setDefaultCmd)
	cmd.AddCommand(infoCmd)
	cmd.AddCommand(serversCmd)
	cmd.AddCommand(syncCmd)

	return cmd
}

func listContexts(cmd *cobra.Command, client *api.Client) error {
	contexts, err := client.ListContexts()
	if err != nil {
		return fmt.Errorf("failed to list contexts: %w", err)
	}

	if tryJSON(cmd, contexts) {
		return nil
	}

	if len(contexts) == 0 {
		PrintWarning("No contexts configured")
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("Contexts"))

	// Build table
	headers := []string{"Name", "Description", "Default"}
	rows := make([][]string, 0, len(contexts))
	for _, ctx := range contexts {
		defaultStr := ""
		if ctx.IsDefault {
			defaultStr = lipgloss.NewStyle().Foreground(special).Bold(true).Render("*")
		}
		rows = append(rows, []string{ctx.Name, ctx.Description, defaultStr})
	}

	RenderTable(headers, rows)
	fmt.Println()

	return nil
}
