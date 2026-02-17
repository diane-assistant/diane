package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newInfoCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show connection guide for AI tools",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Header box
			headerBox := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(highlight).
				Padding(1, 4).
				Align(lipgloss.Center).
				Width(80)

			fmt.Println(headerBox.Render(
				titleStyle.Render("DIANE MCP SERVER"),
			))
			fmt.Println()

			// Status info
			statusDot := okDot.String()
			if status != "running" {
				statusDot = errDot.String()
			}

			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

			fmt.Printf("  %s Status:   %s\n", statusDot, valStyle.Render(status))
			fmt.Printf("    HTTP:     %s\n", valStyle.Render(httpStatus))
			fmt.Printf("    Tools:    %s\n", valStyle.Render(fmt.Sprintf("%d available", toolCount)))
			fmt.Println()

			// Section helper
			sectionHeader := lipgloss.NewStyle().
				Foreground(highlight).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(subtle).
				Width(78)

			codeBlock := lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("235")).
				Padding(0, 1)

			// Connecting section
			fmt.Println(sectionHeader.Render("CONNECTING TO DIANE"))
			fmt.Println()

			// OpenCode
			fmt.Println(dimStyle.Render("  -- OpenCode"))
			fmt.Println()
			fmt.Println("  Add to your opencode.json:")
			fmt.Println()
			fmt.Println(codeBlock.Render(`  {
    "$schema": "https://opencode.ai/config.json",
    "mcp": {
      "diane-personal": {
        "type": "remote",
        "url": "http://localhost:8765/mcp/sse?context=personal",
        "oauth": false
      }
    }
  }`))
			fmt.Println()
			fmt.Println("  Or install automatically with:")
			fmt.Printf("    %s\n", valStyle.Render("diane mcp install opencode"))
			fmt.Println()

			// Claude Desktop
			fmt.Println(dimStyle.Render("  -- Claude Desktop"))
			fmt.Println()
			fmt.Println("  Add to claude_desktop_config.json:")
			fmt.Println("  macOS: ~/Library/Application Support/Claude/claude_desktop_config.json")
			fmt.Println("  Linux: ~/.config/claude/claude_desktop_config.json")
			fmt.Println()
			fmt.Println(codeBlock.Render(fmt.Sprintf(`  {
    "mcpServers": {
      "diane": {
        "command": "%s"
      }
    }
  }`, dianeBin)))
			fmt.Println()

			// Cursor / Windsurf / Continue
			fmt.Println(dimStyle.Render("  -- Cursor / Windsurf / Continue"))
			fmt.Println()
			fmt.Println("  Add to your MCP settings:")
			fmt.Println()
			fmt.Println(codeBlock.Render(fmt.Sprintf(`  {
    "mcpServers": {
      "diane": {
        "command": "%s"
      }
    }
  }`, dianeBin)))
			fmt.Println()

			// HTTP / Network
			fmt.Println(dimStyle.Render("  -- HTTP / Network Clients"))
			fmt.Println()
			fmt.Println("  Diane exposes an HTTP Streamable MCP endpoint when running:")
			fmt.Println()
			fmt.Printf("    URL:     %s\n", valStyle.Render("http://localhost:8765/mcp"))
			fmt.Printf("    SSE:     %s\n", valStyle.Render("http://localhost:8765/mcp/sse"))
			fmt.Printf("    Health:  %s\n", valStyle.Render("http://localhost:8765/health"))
			fmt.Println()

			// Testing
			fmt.Println(sectionHeader.Render("TESTING CONNECTION"))
			fmt.Println()
			fmt.Println("  Test HTTP endpoint:")
			fmt.Printf("    %s\n", valStyle.Render("curl http://localhost:8765/health"))
			fmt.Println()

			// More info
			fmt.Println(sectionHeader.Render("MORE INFO"))
			fmt.Println()
			fmt.Printf("  Documentation:    %s\n", dimStyle.Render("~/.diane/MCP.md"))
			fmt.Printf("  Database:         %s\n", dimStyle.Render("~/.diane/cron.db"))
			fmt.Printf("  Logs:             %s\n", dimStyle.Render("~/.diane/server.log"))
			fmt.Println()
			fmt.Println("  Commands:")
			fmt.Printf("    %s        Full status with all MCP servers\n", valStyle.Render("diane status"))
			fmt.Printf("    %s   List connected MCP servers\n", valStyle.Render("diane mcp-servers"))
			fmt.Printf("    %s        List configured ACP agents\n", valStyle.Render("diane agents"))
			fmt.Printf("    %s       Browse installable agents\n", valStyle.Render("diane gallery"))
			fmt.Println()

			return nil
		},
	}
}
