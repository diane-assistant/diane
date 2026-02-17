package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/acp"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newAgentsCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "List all ACP agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			agents, err := client.ListAgents()
			if err != nil {
				return fmt.Errorf("failed to list agents: %w", err)
			}

			if tryJSON(cmd, agents) {
				return nil
			}

			if len(agents) == 0 {
				fmt.Println("No agents configured. Use 'diane agent add' or 'diane gallery install' to add one.")
				return nil
			}

			fmt.Println(titleStyle.Render("ACP Agents"))
			fmt.Println()

			for _, a := range agents {
				dot := GetStatusDot(a.Enabled, false)
				name := headerStyle.Render(a.Name)
				desc := a.Description
				if desc == "" {
					desc = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(no description)")
				}
				url := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(a.URL)

				fmt.Printf("  %s %s  %s\n", dot, name, url)
				if desc != "" {
					fmt.Printf("      %s\n", desc)
				}
			}

			return nil
		},
	}
}

func newAgentCmd(client *api.Client) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage ACP agents",
	}

	agentCmd.AddCommand(newAgentAddCmd(client))
	agentCmd.AddCommand(newAgentRemoveCmd(client))
	agentCmd.AddCommand(newAgentEnableCmd(client))
	agentCmd.AddCommand(newAgentDisableCmd(client))
	agentCmd.AddCommand(newAgentTestCmd(client))
	agentCmd.AddCommand(newAgentRunCmd(client))
	agentCmd.AddCommand(newAgentInfoCmd(client))
	agentCmd.AddCommand(newAgentLogsCmd(client))

	return agentCmd
}

func newAgentAddCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <url> [description]",
		Short: "Add a new ACP agent",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent := acp.AgentConfig{
				Name:    args[0],
				URL:     args[1],
				Enabled: true,
			}
			if len(args) > 2 {
				agent.Description = args[2]
			}

			if err := client.AddAgent(agent); err != nil {
				return fmt.Errorf("failed to add agent: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Agent '%s' added", agent.Name))
			return nil
		},
	}
}

func newAgentRemoveCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove an ACP agent",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.RemoveAgent(name); err != nil {
				return fmt.Errorf("failed to remove agent: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Agent '%s' removed", name))
			return nil
		},
	}
}

func newAgentEnableCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable an ACP agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.ToggleAgent(name, true); err != nil {
				return fmt.Errorf("failed to enable agent: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Agent '%s' enabled", name))
			return nil
		},
	}
}

func newAgentDisableCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable an ACP agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.ToggleAgent(name, false); err != nil {
				return fmt.Errorf("failed to disable agent: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Agent '%s' disabled", name))
			return nil
		},
	}
}

func newAgentTestCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test connectivity to an ACP agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			result, err := client.TestAgent(name)
			if err != nil {
				return fmt.Errorf("failed to test agent: %w", err)
			}

			if tryJSON(cmd, result) {
				return nil
			}

			fmt.Println(titleStyle.Render(fmt.Sprintf("Agent Test: %s", name)))
			fmt.Println()

			statusColor := lipgloss.Color("82") // green
			switch result.Status {
			case "unreachable":
				statusColor = lipgloss.Color("196") // red
			case "error":
				statusColor = lipgloss.Color("208") // orange
			case "disabled":
				statusColor = lipgloss.Color("240") // grey
			}

			fmt.Printf("  Status:  %s\n", lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(result.Status))
			if result.URL != "" {
				fmt.Printf("  URL:     %s\n", result.URL)
			}
			if result.WorkDir != "" {
				fmt.Printf("  WorkDir: %s\n", result.WorkDir)
			}
			if result.Version != "" {
				fmt.Printf("  Version: %s\n", result.Version)
			}
			if result.AgentCount > 0 {
				fmt.Printf("  Agents:  %d (%s)\n", result.AgentCount, strings.Join(result.Agents, ", "))
			}
			if result.Error != "" {
				fmt.Printf("  Error:   %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(result.Error))
			}

			return nil
		},
	}
}

func newAgentRunCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "run <name> <prompt>",
		Short: "Run a prompt against an ACP agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			prompt := args[1]

			// Use a longer timeout for agent runs
			runClient := api.NewClientWithTimeout(5 * time.Minute)
			run, err := runClient.RunAgent(name, prompt, "")
			if err != nil {
				return fmt.Errorf("failed to run agent: %w", err)
			}

			if tryJSON(cmd, run) {
				return nil
			}

			// Print the text output directly
			output := run.GetTextOutput()
			if output != "" {
				fmt.Print(output)
				if !strings.HasSuffix(output, "\n") {
					fmt.Println()
				}
			} else if run.Error != nil {
				PrintError(fmt.Sprintf("Agent error: %s", run.Error.Message))
			} else {
				fmt.Println("(no output)")
			}

			return nil
		},
	}
}

func newAgentInfoCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:     "info <name>",
		Aliases: []string{"get"},
		Short:   "Show detailed information about an agent",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			agent, err := client.GetAgent(name)
			if err != nil {
				return fmt.Errorf("failed to get agent: %w", err)
			}

			if tryJSON(cmd, agent) {
				return nil
			}

			fmt.Println(titleStyle.Render(fmt.Sprintf("Agent: %s", agent.Name)))
			fmt.Println()

			dot := GetStatusDot(agent.Enabled, false)
			enabledText := "enabled"
			if !agent.Enabled {
				enabledText = "disabled"
			}
			fmt.Printf("  Status:      %s %s\n", dot, enabledText)
			if agent.URL != "" {
				fmt.Printf("  URL:         %s\n", agent.URL)
			}
			if agent.Type != "" {
				fmt.Printf("  Type:        %s\n", agent.Type)
			}
			if agent.Description != "" {
				fmt.Printf("  Description: %s\n", agent.Description)
			}
			if agent.Command != "" {
				fmt.Printf("  Command:     %s\n", agent.Command)
			}
			if len(agent.Args) > 0 {
				fmt.Printf("  Args:        %s\n", strings.Join(agent.Args, " "))
			}
			if agent.WorkDir != "" {
				fmt.Printf("  WorkDir:     %s\n", agent.WorkDir)
			}
			if agent.Port > 0 {
				fmt.Printf("  Port:        %d\n", agent.Port)
			}
			if len(agent.Tags) > 0 {
				fmt.Printf("  Tags:        %s\n", strings.Join(agent.Tags, ", "))
			}

			return nil
		},
	}
}

func newAgentLogsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "Show agent communication logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")

			agentName := ""
			if len(args) > 0 {
				agentName = args[0]
			}

			logs, err := client.GetAgentLogs(agentName, limit)
			if err != nil {
				return fmt.Errorf("failed to get agent logs: %w", err)
			}

			if tryJSON(cmd, logs) {
				return nil
			}

			if len(logs) == 0 {
				fmt.Println("No agent logs found.")
				return nil
			}

			title := "Agent Logs"
			if agentName != "" {
				title = fmt.Sprintf("Agent Logs: %s", agentName)
			}
			fmt.Println(titleStyle.Render(title))
			fmt.Println()

			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			reqArrow := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("->")
			resArrow := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("<-")

			for _, log := range logs {
				arrow := reqArrow
				if log.Direction == "response" {
					arrow = resArrow
				}

				ts := dimStyle.Render(log.Timestamp.Format("15:04:05"))
				agent := headerStyle.Render(log.AgentName)
				msgType := log.MessageType

				durationStr := ""
				if log.DurationMs != nil {
					durationStr = dimStyle.Render(fmt.Sprintf(" (%dms)", *log.DurationMs))
				}

				fmt.Printf("  %s %s %s %s%s\n", ts, arrow, agent, msgType, durationStr)

				if log.Error != nil && *log.Error != "" {
					fmt.Printf("         %s\n", errStyle.Render(*log.Error))
				}
			}

			return nil
		},
	}

	cmd.Flags().IntP("limit", "n", 50, "Maximum number of log entries to show")

	return cmd
}
