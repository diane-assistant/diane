package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newHostsCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "hosts",
		Short: "List connected hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			hosts, err := client.GetHosts()
			if err != nil {
				return fmt.Errorf("failed to get hosts: %w", err)
			}

			if tryJSON(cmd, hosts) {
				return nil
			}

			if len(hosts) == 0 {
				fmt.Println("No hosts connected.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n", titleStyle.Render("Connected Hosts"))

			headers := []string{"ID", "Name", "Type", "Platform", "Status"}
			var rows [][]string

			for _, h := range hosts {
				dot := GetStatusDot(h.Online, false)
				status := "offline"
				if h.Online {
					status = "online"
				}

				rows = append(rows, []string{
					h.ID,
					h.Name,
					h.Type,
					h.Platform,
					dot + " " + status,
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}
}

func newUsageCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show usage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromStr, _ := cmd.Flags().GetString("from")
			toStr, _ := cmd.Flags().GetString("to")

			// Default range: last 30 days
			now := time.Now()
			from := now.AddDate(0, -1, 0)
			to := now

			if fromStr != "" {
				if t, err := time.Parse("2006-01-02", fromStr); err == nil {
					from = t
				} else if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
					from = t
				} else {
					return fmt.Errorf("invalid 'from' date format (use YYYY-MM-DD or RFC3339)")
				}
			}

			if toStr != "" {
				if t, err := time.Parse("2006-01-02", toStr); err == nil {
					to = t
				} else if t, err := time.Parse(time.RFC3339, toStr); err == nil {
					to = t
				} else {
					return fmt.Errorf("invalid 'to' date format (use YYYY-MM-DD or RFC3339)")
				}
			}

			summary, err := client.GetUsageSummary(from, to)
			if err != nil {
				return fmt.Errorf("failed to get usage summary: %w", err)
			}

			if tryJSON(cmd, summary) {
				return nil
			}

			if len(summary.Summary) == 0 {
				fmt.Println("No usage data found for the specified period.")
				return nil
			}

			fmt.Println()
			title := fmt.Sprintf("Usage Summary (%s to %s)", from.Format("2006-01-02"), to.Format("2006-01-02"))
			fmt.Printf("  %s\n", titleStyle.Render(title))

			headers := []string{"Provider", "Service", "Model", "Requests", "Input", "Output", "Cost ($)"}
			var rows [][]string

			for _, s := range summary.Summary {
				rows = append(rows, []string{
					s.ProviderName,
					s.Service,
					s.Model,
					fmt.Sprintf("%d", s.TotalRequests),
					fmt.Sprintf("%d", s.TotalInput),
					fmt.Sprintf("%d", s.TotalOutput),
					fmt.Sprintf("%.4f", s.TotalCost),
				})
			}

			RenderTable(headers, rows)
			fmt.Println()
			fmt.Printf("  %s %s\n",
				lipgloss.NewStyle().Bold(true).Render("Total Cost:"),
				lipgloss.NewStyle().Foreground(special).Render(fmt.Sprintf("$%.4f", summary.TotalCost)))
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
