package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newDoctorCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks on the Diane installation",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := client.Doctor()
			if err != nil {
				PrintError(fmt.Sprintf("Could not run diagnostics: %v", err))
				return nil
			}

			if tryJSON(cmd, report) {
				return nil
			}

			renderDoctorReport(report)
			return nil
		},
	}
}

func renderDoctorReport(report *api.DoctorReport) {
	// Styled check indicators
	checkOK := lipgloss.NewStyle().Foreground(special).Bold(true).Render("✓")
	checkFail := lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✗")
	checkWarn := lipgloss.NewStyle().Foreground(warning).Bold(true).Render("!")

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("Diane Doctor"))
	fmt.Println()

	// Calculate max name width for alignment
	maxNameLen := 0
	for _, c := range report.Checks {
		if len(c.Name) > maxNameLen {
			maxNameLen = len(c.Name)
		}
	}

	for _, check := range report.Checks {
		var indicator string
		var msgStyle lipgloss.Style

		switch check.Status {
		case "ok":
			indicator = checkOK
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		case "fail":
			indicator = checkFail
			msgStyle = lipgloss.NewStyle().Foreground(errorColor)
		case "warn":
			indicator = checkWarn
			msgStyle = lipgloss.NewStyle().Foreground(warning)
		default:
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("?")
			msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		}

		nameStr := lipgloss.NewStyle().
			Width(maxNameLen + 1).
			Foreground(lipgloss.Color("252")).
			Render(check.Name)

		message := msgStyle.Render(check.Message)

		fmt.Printf("    %s %s  %s\n", indicator, nameStr, message)
	}

	fmt.Println()

	// Summary
	if report.Healthy {
		PrintSuccess("All checks passed")
	} else {
		failCount := 0
		warnCount := 0
		for _, c := range report.Checks {
			switch c.Status {
			case "fail":
				failCount++
			case "warn":
				warnCount++
			}
		}

		parts := []string{}
		if failCount > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", failCount))
		}
		if warnCount > 0 {
			parts = append(parts, fmt.Sprintf("%d warnings", warnCount))
		}

		if failCount > 0 {
			PrintError(fmt.Sprintf("Issues found: %s", joinParts(parts)))
		} else {
			PrintWarning(fmt.Sprintf("Issues found: %s", joinParts(parts)))
		}
	}

	fmt.Println()
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}
