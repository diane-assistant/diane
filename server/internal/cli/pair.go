package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/config"
	"github.com/diane-assistant/diane/internal/pairing"
	"github.com/spf13/cobra"
)

func newPairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pair",
		Short: "Show a time-based pairing code for connecting apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			if cfg.HTTP.APIKey == "" {
				PrintError("No API key configured.")
				fmt.Fprintln(os.Stderr, "Pairing requires an API key in your config (http.api_key).")
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "To set one, add to your config file:")
				fmt.Fprintln(os.Stderr, `  {"http": {"port": 9090, "api_key": "your-secret-key"}}`)
				os.Exit(1)
			}

			code := pairing.GenerateCode(cfg.HTTP.APIKey)
			remaining := pairing.TimeRemaining()

			codeStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				Background(highlight).
				Bold(true).
				Padding(0, 2)

			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

			fmt.Println()
			fmt.Printf("  Pairing code:  %s\n", codeStyle.Render(pairing.FormatCode(code)))
			fmt.Printf("  Expires in:    %s\n", lipgloss.NewStyle().Foreground(warning).Render(fmt.Sprintf("%d seconds", remaining)))
			fmt.Println()
			fmt.Println("  Enter this code in the Diane app to connect.")
			fmt.Println(dimStyle.Render("  The code refreshes every 30 seconds."))
			fmt.Println()

			return nil
		},
	}
}
