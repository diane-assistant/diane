package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

// Version is set by the caller when creating the root command
var cliVersion string

// NewRootCmd creates the root command with all subcommands.
// The client is used for all API calls to the Diane daemon.
func NewRootCmd(client *api.Client, version string) *cobra.Command {
	cliVersion = version

	rootCmd := &cobra.Command{
		Use:   "diane",
		Short: "Diane MCP server and control utility",
		Long: titleStyle.Render("Diane") + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(version) + "\n" +
			"  A powerful MCP server and agent platform for AI tools.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")

	// Add all command groups
	rootCmd.AddCommand(newStatusCmd(client))
	rootCmd.AddCommand(newHealthCmd(client))
	rootCmd.AddCommand(newDoctorCmd(client))
	rootCmd.AddCommand(newMCPServersCmd(client))
	rootCmd.AddCommand(newMCPCmd(client))
	rootCmd.AddCommand(newReloadCmd(client))
	rootCmd.AddCommand(newRestartCmd(client))
	rootCmd.AddCommand(newAgentsCmd(client))
	rootCmd.AddCommand(newAgentCmd(client))
	rootCmd.AddCommand(newSessionsCmd(client))
	rootCmd.AddCommand(newSessionCmd(client))
	rootCmd.AddCommand(newGalleryCmd(client))
	rootCmd.AddCommand(newAuthCmd(client))
	rootCmd.AddCommand(newContextCmd(client))
	rootCmd.AddCommand(newProviderCmd(client))
	rootCmd.AddCommand(newJobsCmd(client))
	rootCmd.AddCommand(newToolsCmd(client))
	rootCmd.AddCommand(newPromptsCmd(client))
	rootCmd.AddCommand(newResourcesCmd(client))
	rootCmd.AddCommand(newHostsCmd(client))
	rootCmd.AddCommand(newUsageCmd(client))
	rootCmd.AddCommand(newInfoCmd(client))
	rootCmd.AddCommand(newPairCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newSlaveCmd(client))
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

// --- Utility commands that are simple enough to live here ---

func newHealthCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check if Diane is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client.Health(); err != nil {
				PrintError(fmt.Sprintf("Diane is not running: %v", err))
				os.Exit(1)
			}
			PrintSuccess("Diane is running")
			return nil
		},
	}
}

func newReloadCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload MCP configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client.ReloadConfig(); err != nil {
				return fmt.Errorf("reload failed: %w", err)
			}
			PrintSuccess("Configuration reloaded")
			return nil
		},
	}
}

func newRestartCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <server-name>",
		Short: "Restart a specific MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.RestartMCPServer(name); err != nil {
				return fmt.Errorf("restart failed: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Server '%s' restarted", name))
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("diane %s\n", cliVersion)
		},
	}
}

// --- JSON output helper ---

func outputJSON(cmd *cobra.Command, v interface{}) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	return fmt.Errorf("not json mode")
}

// tryJSON returns true if --json was set and data was printed
func tryJSON(cmd *cobra.Command, v interface{}) bool {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if !jsonFlag {
		return false
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return false
	}
	fmt.Println(string(out))
	return true
}

// formatDuration formats a duration as human-readable
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
