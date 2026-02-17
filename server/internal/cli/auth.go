package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newAuthCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage OAuth authentication for MCP servers",
		Long:  titleStyle.Render("Auth") + "\n  Manage OAuth authentication for MCP servers that require it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listOAuthServers(cmd, client)
		},
	}

	// login subcommand
	loginCmd := &cobra.Command{
		Use:   "login <server>",
		Short: "Authenticate with an OAuth-enabled MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauthLogin(cmd, client, args[0])
		},
	}

	// status subcommand
	statusCmd := &cobra.Command{
		Use:   "status <server>",
		Short: "Show OAuth status for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauthStatus(cmd, client, args[0])
		},
	}

	// logout subcommand
	logoutCmd := &cobra.Command{
		Use:   "logout <server>",
		Short: "Remove OAuth credentials for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauthLogout(client, args[0])
		},
	}

	cmd.AddCommand(loginCmd)
	cmd.AddCommand(statusCmd)
	cmd.AddCommand(logoutCmd)

	return cmd
}

func listOAuthServers(cmd *cobra.Command, client *api.Client) error {
	servers, err := client.ListOAuthServers()
	if err != nil {
		return fmt.Errorf("failed to list OAuth servers: %w", err)
	}

	if tryJSON(cmd, servers) {
		return nil
	}

	if len(servers) == 0 {
		PrintWarning("No OAuth-enabled MCP servers configured")
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("OAuth Servers"))

	headers := []string{"Server", "Provider", "Status"}
	rows := make([][]string, 0, len(servers))
	for _, s := range servers {
		dot := GetStatusDot(s.Authenticated, false)
		statusStr := s.Status
		if s.Authenticated {
			statusStr = lipgloss.NewStyle().Foreground(special).Render(statusStr)
		} else {
			statusStr = lipgloss.NewStyle().Foreground(warning).Render(statusStr)
		}
		rows = append(rows, []string{dot + " " + s.Name, s.Provider, statusStr})
	}

	RenderTable(headers, rows)
	fmt.Println()

	return nil
}

func oauthLogin(cmd *cobra.Command, client *api.Client, serverName string) error {
	deviceInfo, err := client.StartOAuthLogin(serverName)
	if err != nil {
		return fmt.Errorf("failed to start OAuth login: %w", err)
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("OAuth Login"))
	fmt.Println()

	urlStyle := lipgloss.NewStyle().Foreground(highlight).Bold(true)
	codeStyle := lipgloss.NewStyle().Foreground(special).Bold(true)

	fmt.Printf("  Open this URL in your browser:\n")
	fmt.Printf("  %s\n\n", urlStyle.Render(deviceInfo.VerificationURI))
	fmt.Printf("  Enter this code: %s\n\n", codeStyle.Render(deviceInfo.UserCode))

	fmt.Printf("  Waiting for authorization...")

	// Poll with a long-timeout client
	pollClient := api.NewClientWithTimeout(10 * time.Minute)
	if err := pollClient.PollOAuthToken(serverName, deviceInfo.DeviceCode, deviceInfo.Interval); err != nil {
		fmt.Println()
		PrintError(fmt.Sprintf("Authentication failed: %v", err))
		return nil
	}

	fmt.Println()
	PrintSuccess(fmt.Sprintf("Successfully authenticated with '%s'", serverName))
	return nil
}

func oauthStatus(cmd *cobra.Command, client *api.Client, serverName string) error {
	status, err := client.GetOAuthStatus(serverName)
	if err != nil {
		return fmt.Errorf("failed to get OAuth status: %w", err)
	}

	// Always output as JSON since it's unstructured map data
	out, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

func oauthLogout(client *api.Client, serverName string) error {
	if err := client.LogoutOAuth(serverName); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	PrintSuccess(fmt.Sprintf("Logged out from '%s'", serverName))
	return nil
}
