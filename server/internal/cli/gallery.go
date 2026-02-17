package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newGalleryCmd(client *api.Client) *cobra.Command {
	galleryCmd := &cobra.Command{
		Use:   "gallery",
		Short: "Browse and install agents from the gallery",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to listing when no subcommand is given
			return galleryListRun(cmd, client, false)
		},
	}

	galleryCmd.AddCommand(newGalleryListCmd(client))
	galleryCmd.AddCommand(newGalleryFeaturedCmd(client))
	galleryCmd.AddCommand(newGalleryInfoCmd(client))
	galleryCmd.AddCommand(newGalleryInstallCmd(client))
	galleryCmd.AddCommand(newGalleryRefreshCmd(client))

	return galleryCmd
}

func newGalleryListCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available agents in the gallery",
		RunE: func(cmd *cobra.Command, args []string) error {
			featured, _ := cmd.Flags().GetBool("featured")
			return galleryListRun(cmd, client, featured)
		},
	}

	cmd.Flags().Bool("featured", false, "Show only featured agents")

	return cmd
}

func newGalleryFeaturedCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "featured",
		Short: "List featured agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return galleryListRun(cmd, client, true)
		},
	}
}

func galleryListRun(cmd *cobra.Command, client *api.Client, featured bool) error {
	entries, err := client.ListGallery(featured)
	if err != nil {
		return fmt.Errorf("failed to list gallery: %w", err)
	}

	if tryJSON(cmd, entries) {
		return nil
	}

	if len(entries) == 0 {
		fmt.Println("No gallery entries found.")
		return nil
	}

	title := "Agent Gallery"
	if featured {
		title = "Featured Agents"
	}
	fmt.Println(titleStyle.Render(title))
	fmt.Println()

	// Build table rows
	headers := []string{"", "ID", "Name", "Description", "Provider"}
	rows := make([][]string, 0, len(entries))

	for _, e := range entries {
		star := " "
		if e.Featured {
			star = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("*")
		}

		desc := e.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		rows = append(rows, []string{star, e.ID, e.Name, desc, e.Provider})
	}

	RenderTable(headers, rows)

	return nil
}

func newGalleryInfoCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "info <id>",
		Short: "Show installation info for a gallery agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			info, err := client.GetGalleryAgent(id)
			if err != nil {
				return fmt.Errorf("failed to get gallery agent: %w", err)
			}

			if tryJSON(cmd, info) {
				return nil
			}

			fmt.Println(titleStyle.Render(fmt.Sprintf("Gallery Agent: %s", info.Name)))
			fmt.Println()

			fmt.Printf("  ID:          %s\n", info.ID)
			if info.Version != "" {
				fmt.Printf("  Version:     %s\n", info.Version)
			}
			fmt.Printf("  Description: %s\n", info.Description)
			fmt.Printf("  Install:     %s\n", info.InstallType)
			if info.InstallCmd != "" {
				fmt.Printf("  Command:     %s\n", info.InstallCmd)
			}
			if info.Available {
				PrintSuccess("Available on this system")
			} else {
				if info.Error != "" {
					PrintError(fmt.Sprintf("Not available: %s", info.Error))
				} else {
					PrintWarning("Not installed")
				}
			}

			return nil
		},
	}
}

func newGalleryInstallCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <id>",
		Short: "Install and configure an agent from the gallery",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			name, _ := cmd.Flags().GetString("name")
			workdir, _ := cmd.Flags().GetString("workdir")
			port, _ := cmd.Flags().GetInt("port")

			// Get install info first to show the user what will happen
			info, err := client.GetGalleryAgent(id)
			if err != nil {
				return fmt.Errorf("failed to get gallery agent: %w", err)
			}

			// Install/configure the agent
			if err := client.InstallGalleryAgentWithOptions(id, name, workdir, port); err != nil {
				return fmt.Errorf("failed to install agent: %w", err)
			}

			agentName := name
			if agentName == "" {
				agentName = id
			}

			PrintSuccess(fmt.Sprintf("Agent '%s' configured", agentName))

			if info.InstallCmd != "" {
				fmt.Printf("\n  Install the agent binary with:\n    %s\n",
					lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(info.InstallCmd))
			}

			return nil
		},
	}

	cmd.Flags().String("name", "", "Custom name for the agent")
	cmd.Flags().String("workdir", "", "Working directory for the agent")
	cmd.Flags().Int("port", 0, "Port for ACP server")

	return cmd
}

func newGalleryRefreshCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the agent gallery registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client.RefreshGallery(); err != nil {
				return fmt.Errorf("failed to refresh gallery: %w", err)
			}

			PrintSuccess("Gallery refreshed")
			return nil
		},
	}
}
