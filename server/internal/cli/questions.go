package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newQuestionsCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "questions",
		Short: "Manage pending agent questions",
		Long:  titleStyle.Render("Questions") + "\n  View and answer pending questions from emergent agents.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listQuestions(cmd, client)
		},
	}

	cmd.AddCommand(newQuestionsListCmd(client))
	cmd.AddCommand(newQuestionsAnswerCmd(client))
	return cmd
}

func newQuestionsListCmd(client *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pending agent questions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listQuestions(cmd, client)
		},
	}
}

func listQuestions(cmd *cobra.Command, client *api.Client) error {
	questions, err := client.ListQuestions("pending")
	if err != nil {
		return fmt.Errorf("failed to list questions: %w", err)
	}

	if tryJSON(cmd, questions) {
		return nil
	}

	if len(questions) == 0 {
		fmt.Println("No pending questions.")
		return nil
	}

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	agentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	optionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	fmt.Println(titleStyle.Render("Pending Agent Questions"))
	fmt.Println()

	for _, q := range questions {
		shortID := q.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		optionsSummary := ""
		if len(q.Options) > 0 {
			optionsSummary = fmt.Sprintf(" (%d options)", len(q.Options))
		}

		fmt.Printf("  %s  %s%s\n",
			idStyle.Render("["+shortID+"]"),
			agentStyle.Render(q.AgentID),
			optionStyle.Render(optionsSummary),
		)
		fmt.Printf("      %s\n", q.Question)
		fmt.Println()
	}
	return nil
}

func newQuestionsAnswerCmd(client *api.Client) *cobra.Command {
	var responseFlagVal string

	cmd := &cobra.Command{
		Use:   "answer <id>",
		Short: "Answer a pending agent question",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			questionID := args[0]

			// Fetch current pending questions to find this one.
			questions, err := client.ListQuestions("pending")
			if err != nil {
				return fmt.Errorf("failed to fetch questions: %w", err)
			}

			// Match by full ID or prefix.
			var found *struct {
				ID       string
				Question string
				Options  []struct {
					Label string
					Value string
				}
			}
			for _, q := range questions {
				if q.ID == questionID || strings.HasPrefix(q.ID, questionID) {
					found = &struct {
						ID       string
						Question string
						Options  []struct {
							Label string
							Value string
						}
					}{
						ID:       q.ID,
						Question: q.Question,
					}
					for _, o := range q.Options {
						found.Options = append(found.Options, struct {
							Label string
							Value string
						}{Label: o.Label, Value: o.Value})
					}
					break
				}
			}

			if found == nil {
				return fmt.Errorf("question %q not found in pending questions", questionID)
			}

			var response string

			if responseFlagVal != "" {
				// Non-interactive mode.
				response = responseFlagVal
			} else {
				fmt.Println()
				fmt.Println(headerStyle.Render("Question:"))
				fmt.Printf("  %s\n\n", found.Question)

				if len(found.Options) > 0 {
					fmt.Println(headerStyle.Render("Options:"))
					for i, opt := range found.Options {
						fmt.Printf("  %d. %s\n", i+1, opt.Label)
					}
					fmt.Println()
					fmt.Print("Enter option number: ")
					scanner := bufio.NewScanner(os.Stdin)
					scanner.Scan()
					input := strings.TrimSpace(scanner.Text())
					idx, err := strconv.Atoi(input)
					if err != nil || idx < 1 || idx > len(found.Options) {
						return fmt.Errorf("invalid selection %q; must be 1–%d", input, len(found.Options))
					}
					response = found.Options[idx-1].Value
				} else {
					fmt.Print("Your answer: ")
					scanner := bufio.NewScanner(os.Stdin)
					scanner.Scan()
					response = strings.TrimSpace(scanner.Text())
					if response == "" {
						return fmt.Errorf("response cannot be empty")
					}
				}
			}

			if err := client.RespondToQuestion(found.ID, response); err != nil {
				return fmt.Errorf("failed to submit answer: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Answer submitted for question %s", found.ID[:8]))
			return nil
		},
	}

	cmd.Flags().StringVarP(&responseFlagVal, "response", "r", "", "Answer to submit (skips interactive prompt)")
	return cmd
}
