package cli

import (
	"fmt"
	"time"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newJobsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage scheduled jobs",
		Long:  titleStyle.Render("Jobs") + "\n  Manage scheduled jobs and view execution logs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listJobs(cmd, client)
		},
	}

	// list subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List scheduled jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listJobs(cmd, client)
		},
	}

	// logs subcommand
	logsCmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "Show job execution logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			logs, err := client.GetJobLogs(name, limit)
			if err != nil {
				return fmt.Errorf("failed to get job logs: %w", err)
			}

			if tryJSON(cmd, logs) {
				return nil
			}

			if len(logs) == 0 {
				fmt.Println("No job logs found.")
				return nil
			}

			fmt.Println()
			title := "Job Logs"
			if name != "" {
				title = fmt.Sprintf("Job Logs: %s", name)
			}
			fmt.Printf("  %s\n", titleStyle.Render(title))

			headers := []string{"Job", "Status", "Duration", "Started", "Output"}
			var rows [][]string

			for _, l := range logs {
				status := "running"
				duration := "-"
				if l.EndedAt != nil {
					if l.ExitCode != nil && *l.ExitCode == 0 {
						status = "success"
					} else {
						status = "failed"
					}
					d := l.EndedAt.Sub(l.StartedAt)
					duration = formatDuration(d)
				}

				output := ""
				if l.Error != nil {
					output = *l.Error
				} else if l.Stderr != "" {
					output = l.Stderr
				} else {
					output = l.Stdout
				}
				if len(output) > 50 {
					output = output[:47] + "..."
				}

				rows = append(rows, []string{
					l.JobName,
					status,
					duration,
					l.StartedAt.Format(time.RFC3339),
					output,
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}
	logsCmd.Flags().IntP("limit", "n", 50, "Maximum number of log entries to show")

	// enable subcommand
	enableCmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.ToggleJob(name, true); err != nil {
				return fmt.Errorf("failed to enable job: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Job '%s' enabled", name))
			return nil
		},
	}

	// disable subcommand
	disableCmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := client.ToggleJob(name, false); err != nil {
				return fmt.Errorf("failed to disable job: %w", err)
			}
			PrintSuccess(fmt.Sprintf("Job '%s' disabled", name))
			return nil
		},
	}

	cmd.AddCommand(listCmd)
	cmd.AddCommand(logsCmd)
	cmd.AddCommand(enableCmd)
	cmd.AddCommand(disableCmd)

	return cmd
}

func listJobs(cmd *cobra.Command, client *api.Client) error {
	jobs, err := client.ListJobs()
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	if tryJSON(cmd, jobs) {
		return nil
	}

	if len(jobs) == 0 {
		fmt.Println("No scheduled jobs found.")
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("Scheduled Jobs"))

	headers := []string{"Name", "Schedule", "Status", "Command"}
	var rows [][]string

	for _, j := range jobs {
		status := "disabled"
		if j.Enabled {
			status = "enabled"
		}

		cmdStr := j.Command
		if j.ActionType == "agent" && j.AgentName != nil {
			cmdStr = fmt.Sprintf("Agent: %s", *j.AgentName)
		}

		rows = append(rows, []string{
			j.Name,
			j.Schedule,
			status,
			cmdStr,
		})
	}

	RenderTable(headers, rows)
	fmt.Println()

	return nil
}
