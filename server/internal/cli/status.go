package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newStatusCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Diane daemon status and MCP server overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := client.GetStatus()
			if err != nil {
				PrintError(fmt.Sprintf("Could not reach Diane daemon: %v", err))
				return nil
			}

			if tryJSON(cmd, status) {
				return nil
			}

			renderStatusDashboard(status)
			return nil
		},
	}
}

func renderStatusDashboard(s *api.Status) {
	// --- Header line: Diane v1.14.5  ●  PID 12345  ●  Uptime 3d 12h ---
	version := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Diane " + s.Version)

	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ●  ")

	pidStr := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(fmt.Sprintf("PID %d", s.PID))

	uptimeDur := time.Duration(s.UptimeSeconds) * time.Second
	uptimeStr := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render("Uptime " + formatDuration(uptimeDur))

	fmt.Println()
	fmt.Printf("  %s%s%s%s%s\n", version, separator, pidStr, separator, uptimeStr)

	// Platform line
	platform := s.Platform
	if s.Architecture != "" {
		platform = s.Platform + "/" + s.Architecture
	}
	platformLine := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Platform: " + platform)
	fmt.Printf("  %s\n", platformLine)

	// Slave mode info
	if s.SlaveMode {
		slaveLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Slave mode:")
		if s.SlaveConnected {
			fmt.Printf("  %s %s %s\n", slaveLabel, okDot.String(),
				lipgloss.NewStyle().Foreground(special).Render("connected to "+s.MasterURL))
		} else {
			errMsg := "disconnected"
			if s.SlaveError != "" {
				errMsg = s.SlaveError
			}
			fmt.Printf("  %s %s %s\n", slaveLabel, errDot.String(),
				lipgloss.NewStyle().Foreground(errorColor).Render(errMsg))
		}
	}

	// --- MCP Servers section ---
	fmt.Println()

	if len(s.MCPServers) == 0 {
		sectionHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true).Render("MCP Servers")
		fmt.Printf("  %s %s\n", sectionHeader,
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(none configured)"))
		fmt.Println()
		return
	}

	connected := 0
	total := 0
	for _, srv := range s.MCPServers {
		if srv.Enabled {
			total++
			if srv.Connected {
				connected++
			}
		}
	}

	countStr := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("(%d/%d connected)", connected, total))
	sectionHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true).
		Render("MCP Servers")
	fmt.Printf("  %s %s\n", sectionHeader, countStr)

	// Calculate max name width for alignment
	maxNameLen := 0
	for _, srv := range s.MCPServers {
		if len(srv.Name) > maxNameLen {
			maxNameLen = len(srv.Name)
		}
	}

	for _, srv := range s.MCPServers {
		dot := GetStatusDot(srv.Connected, srv.Error != "")

		nameStyle := lipgloss.NewStyle().Width(maxNameLen + 1)
		name := nameStyle.Render(srv.Name)

		// Determine badge type
		badgeType := "stdio"
		if srv.Builtin {
			badgeType = "builtin"
		}

		badge := GetTypeBadge(badgeType)

		// Build the detail string
		var detail string
		if srv.Error != "" {
			detail = lipgloss.NewStyle().Foreground(errorColor).Render("error: " + srv.Error)
		} else if !srv.Enabled {
			detail = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("disabled")
		} else {
			parts := []string{}
			if srv.ToolCount > 0 {
				parts = append(parts, fmt.Sprintf("%d tools", srv.ToolCount))
			}
			if srv.PromptCount > 0 {
				parts = append(parts, fmt.Sprintf("%d prompts", srv.PromptCount))
			}
			if srv.ResourceCount > 0 {
				parts = append(parts, fmt.Sprintf("%d resources", srv.ResourceCount))
			}
			if len(parts) == 0 {
				parts = append(parts, "0 tools")
			}
			detail = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(strings.Join(parts, ", "))
		}

		// Auth indicator
		authStr := ""
		if srv.RequiresAuth && !srv.Authenticated {
			authStr = "  " + lipgloss.NewStyle().Foreground(warning).Render("(auth required)")
		}

		fmt.Printf("    %s %s  %s  %s%s\n", dot, name, badge, detail, authStr)
	}

	// --- Footer with total tools ---
	fmt.Println()
	totalLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("  %d total tools available", s.TotalTools))
	fmt.Println(totalLabel)
	fmt.Println()
}
