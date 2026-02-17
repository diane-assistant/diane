package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	// Colors
	subtle     = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight  = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special    = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	warning    = lipgloss.AdaptiveColor{Light: "#F29F05", Dark: "#F29F05"}
	errorColor = lipgloss.AdaptiveColor{Light: "#E05252", Dark: "#E05252"}

	// Text styles
	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true).
			Padding(0, 1)

	// Status dots
	dotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).SetString("•")
	okDot    = lipgloss.NewStyle().Foreground(special).SetString("●")
	warnDot  = lipgloss.NewStyle().Foreground(warning).SetString("●")
	errDot   = lipgloss.NewStyle().Foreground(errorColor).SetString("●")

	// Badges
	badgeStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true)

	httpBadge    = badgeStyle.Copy().Background(lipgloss.Color("#3C8AFF")) // Blue
	sseBadge     = badgeStyle.Copy().Background(lipgloss.Color("#8E44AD")) // Purple
	stdioBadge   = badgeStyle.Copy().Background(lipgloss.Color("#27AE60")) // Green
	builtinBadge = badgeStyle.Copy().Background(lipgloss.Color("#7F8C8D")) // Grey

	// Table styles
	tableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(subtle)
)

// Helper functions for common output patterns

func PrintSuccess(msg string) {
	fmt.Printf("%s %s\n", okDot.String(), msg)
}

func PrintError(msg string) {
	fmt.Printf("%s %s\n", errDot.String(), msg)
}

func PrintWarning(msg string) {
	fmt.Printf("%s %s\n", warnDot.String(), msg)
}

func RenderTable(headers []string, rows [][]string) {
	// Simple column width calculation for now
	// In a real implementation, we'd use lipgloss/table or calculate max widths
	if len(rows) == 0 {
		return
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, h := range headers {
		fmt.Print(headerStyle.Copy().Width(widths[i] + 2).Render(h))
	}
	fmt.Println()

	// Print separator
	// fmt.Println(strings.Repeat("-", sum(widths) + len(widths)*2))

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Print(lipgloss.NewStyle().Width(widths[i]+2).Padding(0, 1).Render(cell))
			}
		}
		fmt.Println()
	}
}

func GetStatusDot(connected bool, hasError bool) string {
	if hasError {
		return errDot.String()
	}
	if connected {
		return okDot.String()
	}
	return dotStyle.String()
}

func GetTypeBadge(serverType string) string {
	switch serverType {
	case "http":
		return httpBadge.Render("HTTP")
	case "sse":
		return sseBadge.Render("SSE")
	case "stdio":
		return stdioBadge.Render("STDIO")
	case "builtin":
		return builtinBadge.Render("BUILTIN")
	default:
		return badgeStyle.Copy().Background(lipgloss.Color("#95A5A6")).Render(serverType)
	}
}
