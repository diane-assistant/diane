package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View server logs",
		Long:  "View Diane server logs. Defaults to showing the last 100 lines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				PrintError(fmt.Sprintf("could not determine home directory: %v", err))
				os.Exit(1)
			}

			logPath := filepath.Join(home, ".diane", "server.log")

			lines, _ := cmd.Flags().GetInt("lines")
			follow, _ := cmd.Flags().GetBool("follow")

			// Check if log file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				PrintError(fmt.Sprintf("log file not found at %s", logPath))
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Diane may not be running, or logs may be written elsewhere.")
				fmt.Fprintln(os.Stderr, "Try: diane status")
				os.Exit(1)
			}

			if follow {
				fmt.Fprintf(os.Stderr, "Following logs at %s (Ctrl+C to stop)...\n\n", logPath)
				tailCmd := exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", lines), logPath)
				tailCmd.Stdout = os.Stdout
				tailCmd.Stderr = os.Stderr
				if err := tailCmd.Run(); err != nil {
					return fmt.Errorf("error following logs: %w", err)
				}
			} else {
				data, err := os.ReadFile(logPath)
				if err != nil {
					return fmt.Errorf("error reading log file: %w", err)
				}

				allLines := strings.Split(string(data), "\n")

				// Remove empty last line if present
				if len(allLines) > 0 && allLines[len(allLines)-1] == "" {
					allLines = allLines[:len(allLines)-1]
				}

				// Get last N lines
				start := 0
				if len(allLines) > lines {
					start = len(allLines) - lines
				}

				displayLines := allLines[start:]

				for _, line := range displayLines {
					fmt.Println(line)
				}

				fmt.Fprintf(os.Stderr, "\n(Showing last %d lines of %d total. Use -n to adjust, -f to follow)\n", len(displayLines), len(allLines))
				fmt.Fprintf(os.Stderr, "Log file: %s\n", logPath)
			}

			return nil
		},
	}

	cmd.Flags().IntP("lines", "n", 100, "Number of lines to show")
	cmd.Flags().BoolP("follow", "f", false, "Follow log output in real-time")

	return cmd
}
