package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newSessionsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List all ACP sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName, _ := cmd.Flags().GetString("agent")
			status, _ := cmd.Flags().GetString("status")

			sessions, err := client.ListSessions(agentName, status)
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			if tryJSON(cmd, sessions) {
				return nil
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found. Use 'diane session start <agent>' to create one.")
				return nil
			}

			title := "ACP Sessions"
			if agentName != "" {
				title = fmt.Sprintf("ACP Sessions: %s", agentName)
			}
			fmt.Println(titleStyle.Render(title))
			fmt.Println()

			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

			for _, s := range sessions {
				statusColor := sessionStatusColor(string(s.Status))
				dot := lipgloss.NewStyle().Foreground(statusColor).Render("●")
				name := headerStyle.Render(s.AgentName)
				sid := dimStyle.Render(s.SessionID[:12] + "...")
				statusText := lipgloss.NewStyle().Foreground(statusColor).Render(string(s.Status))
				turns := fmt.Sprintf("%d turns", s.TurnCount)
				age := formatDuration(time.Since(s.CreatedAt))

				fmt.Printf("  %s %s  %s  %s  %s  %s ago\n", dot, name, sid, statusText, turns, age)
				if s.Title != "" {
					fmt.Printf("      %s\n", dimStyle.Render(s.Title))
				}
				if s.WorkDir != "" {
					fmt.Printf("      %s\n", dimStyle.Render(s.WorkDir))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringP("agent", "a", "", "Filter by agent name")
	cmd.Flags().StringP("status", "s", "", "Filter by status (active, idle, closed, disconnected)")

	return cmd
}

func newSessionCmd(client *api.Client) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage ACP sessions",
	}

	sessionCmd.AddCommand(newSessionStartCmd(client))
	sessionCmd.AddCommand(newSessionPromptCmd(client))
	sessionCmd.AddCommand(newSessionInfoCmd(client))
	sessionCmd.AddCommand(newSessionCloseCmd(client))
	sessionCmd.AddCommand(newSessionConfigCmd(client))
	sessionCmd.AddCommand(newSessionMessagesCmd(client))

	return sessionCmd
}

func newSessionStartCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <agent>",
		Short: "Start a new multi-turn session with an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			workDir, _ := cmd.Flags().GetString("workdir")
			title, _ := cmd.Flags().GetString("title")

			// Use a longer timeout since spawning can be slow
			longClient := api.NewClientWithTimeout(60 * time.Second)
			info, err := longClient.StartSession(agentName, workDir, title)
			if err != nil {
				return fmt.Errorf("failed to start session: %w", err)
			}

			if tryJSON(cmd, info) {
				return nil
			}

			PrintSuccess(fmt.Sprintf("Session started: %s", info.SessionID))
			fmt.Printf("  Agent:   %s\n", info.AgentName)
			fmt.Printf("  WorkDir: %s\n", info.WorkDir)
			if info.ModelID != "" {
				fmt.Printf("  Model:   %s\n", info.ModelID)
			}
			if info.ModeID != "" {
				fmt.Printf("  Mode:    %s\n", info.ModeID)
			}

			return nil
		},
	}

	cmd.Flags().StringP("workdir", "w", "", "Working directory for the session")
	cmd.Flags().StringP("title", "t", "", "Optional session title")

	return cmd
}

func newSessionPromptCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "prompt <agent> <session-id> <prompt>",
		Short: "Send a prompt to an existing session",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			sessionID := args[1]
			prompt := args[2]

			// Use a longer timeout for agent runs
			longClient := api.NewClientWithTimeout(5 * time.Minute)
			run, err := longClient.PromptSession(agentName, sessionID, prompt)
			if err != nil {
				return fmt.Errorf("failed to prompt session: %w", err)
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
				PrintError(fmt.Sprintf("Session error: %s", run.Error.Message))
			} else {
				fmt.Println("(no output)")
			}

			return nil
		},
	}
}

func newSessionInfoCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:     "info <agent> <session-id>",
		Aliases: []string{"get"},
		Short:   "Show information about a session",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			sessionID := args[1]

			info, err := client.GetSessionInfo(agentName, sessionID)
			if err != nil {
				return fmt.Errorf("failed to get session: %w", err)
			}

			if tryJSON(cmd, info) {
				return nil
			}

			fmt.Println(titleStyle.Render(fmt.Sprintf("Session: %s", info.SessionID)))
			fmt.Println()

			statusColor := sessionStatusColor(string(info.Status))
			statusText := lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(string(info.Status))

			fmt.Printf("  Agent:      %s\n", info.AgentName)
			fmt.Printf("  Status:     %s\n", statusText)
			fmt.Printf("  Turns:      %d\n", info.TurnCount)
			fmt.Printf("  WorkDir:    %s\n", info.WorkDir)
			fmt.Printf("  Created:    %s\n", info.CreatedAt.Format(time.RFC3339))
			fmt.Printf("  Last Active: %s\n", info.LastActiveAt.Format(time.RFC3339))
			if info.Title != "" {
				fmt.Printf("  Title:      %s\n", info.Title)
			}
			if info.ModelID != "" {
				fmt.Printf("  Model:      %s\n", info.ModelID)
			}
			if info.ModeID != "" {
				fmt.Printf("  Mode:       %s\n", info.ModeID)
			}

			return nil
		},
	}
}

func newSessionCloseCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:     "close <agent> <session-id>",
		Aliases: []string{"kill", "stop"},
		Short:   "Close an active session",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			sessionID := args[1]

			if err := client.CloseSession(agentName, sessionID); err != nil {
				return fmt.Errorf("failed to close session: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Session '%s' closed", sessionID))
			return nil
		},
	}
}

func newSessionConfigCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "config <agent> <session-id> <config-id> <value>",
		Short: "Set a configuration option on a live session (e.g., model, mode)",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			sessionID := args[1]
			configID := args[2]
			value := args[3]

			if err := client.SetSessionConfig(agentName, sessionID, configID, value); err != nil {
				return fmt.Errorf("failed to set session config: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Set %s=%s on session %s", configID, value, sessionID))
			return nil
		},
	}
}

func newSessionMessagesCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:     "messages <agent> <session-id>",
		Aliases: []string{"history"},
		Short:   "Show full message history for a session",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			sessionID := args[1]

			messages, err := client.GetSessionMessages(agentName, sessionID)
			if err != nil {
				return fmt.Errorf("failed to get session messages: %w", err)
			}

			if tryJSON(cmd, messages) {
				return nil
			}

			if len(messages) == 0 {
				fmt.Println("No messages in this session.")
				return nil
			}

			fmt.Println(titleStyle.Render(fmt.Sprintf("Session Messages (%d turns)", len(messages))))
			fmt.Println()

			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

			for _, m := range messages {
				// Turn header
				ts := dimStyle.Render(m.CreatedAt.Format("15:04:05"))
				duration := ""
				if m.DurationMs > 0 {
					duration = dimStyle.Render(fmt.Sprintf(" (%dms)", m.DurationMs))
				}
				fmt.Printf("  %s Turn %d%s\n", ts, m.TurnNumber, duration)

				// Prompt
				promptPreview := m.Prompt
				if len(promptPreview) > 120 {
					promptPreview = promptPreview[:120] + "..."
				}
				fmt.Printf("    %s %s\n", promptStyle.Render(">"), promptPreview)

				// Tool calls
				if len(m.ToolCalls) > 0 {
					for _, tc := range m.ToolCalls {
						toolInfo := tc.Title
						if toolInfo == "" {
							toolInfo = tc.Kind
						}
						status := dimStyle.Render(tc.Status)
						fmt.Printf("    %s %s %s\n", dimStyle.Render("T"), toolInfo, status)
					}
				}

				// Response preview
				if m.Response != "" {
					respPreview := m.Response
					if len(respPreview) > 200 {
						respPreview = respPreview[:200] + "..."
					}
					// Replace newlines for compact display
					respPreview = strings.ReplaceAll(respPreview, "\n", " ")
					fmt.Printf("    %s\n", respPreview)
				}

				// Error
				if m.Error != "" {
					fmt.Printf("    %s\n", errStyle.Render(m.Error))
				}
				fmt.Println()
			}

			return nil
		},
	}
}

// sessionStatusColor returns a lipgloss.Color for a session status string.
func sessionStatusColor(status string) lipgloss.Color {
	switch status {
	case "active":
		return lipgloss.Color("82") // green
	case "idle":
		return lipgloss.Color("226") // yellow
	case "closed":
		return lipgloss.Color("240") // grey
	case "disconnected":
		return lipgloss.Color("196") // red
	default:
		return lipgloss.Color("245")
	}
}
