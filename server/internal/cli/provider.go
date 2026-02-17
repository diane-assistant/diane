package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/spf13/cobra"
)

func newProviderCmd(client *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage LLM and storage providers",
		Long:  titleStyle.Render("Providers") + "\n  Manage LLM, embeddings, and storage providers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProviders(cmd, client)
		},
	}

	// list subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProviders(cmd, client)
		},
	}
	listCmd.Flags().String("type", "", "Filter by provider type (llm|embeddings|storage)")

	// create subcommand
	createCmd := &cobra.Command{
		Use:   "create <name> <service>",
		Short: "Create a new provider",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			service := args[1]
			providerType, _ := cmd.Flags().GetString("type")

			req := api.CreateProviderRequest{
				Name:    name,
				Service: service,
				Type:    providerType,
				Config:  make(map[string]any),
			}

			// Parse config flags
			configSlice, _ := cmd.Flags().GetStringSlice("config")
			for _, c := range configSlice {
				parts := strings.SplitN(c, "=", 2)
				if len(parts) == 2 {
					req.Config[parts[0]] = parts[1]
				}
			}

			// Parse auth flags
			authSlice, _ := cmd.Flags().GetStringSlice("auth")
			if len(authSlice) > 0 {
				req.AuthConfig = make(map[string]any)
				for _, a := range authSlice {
					parts := strings.SplitN(a, "=", 2)
					if len(parts) == 2 {
						req.AuthConfig[parts[0]] = parts[1]
					}
				}
			}

			provider, err := client.CreateProvider(req)
			if err != nil {
				return fmt.Errorf("failed to create provider: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Created provider '%s' (id: %d)", provider.Name, provider.ID))
			return nil
		},
	}
	createCmd.Flags().String("type", "", "Provider type (llm, embedding, storage)")
	createCmd.Flags().StringSlice("config", nil, "Configuration key=value")
	createCmd.Flags().StringSlice("auth", nil, "Auth configuration key=value")

	// edit subcommand
	editCmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			req := api.UpdateProviderRequest{}
			hasChanges := false

			name, _ := cmd.Flags().GetString("name")
			if name != "" {
				req.Name = &name
				hasChanges = true
			}

			// Config changes
			configSlice, _ := cmd.Flags().GetStringSlice("config")
			if len(configSlice) > 0 {
				cfg := make(map[string]any)
				for _, c := range configSlice {
					parts := strings.SplitN(c, "=", 2)
					if len(parts) == 2 {
						cfg[parts[0]] = parts[1]
					}
				}
				req.Config = &cfg
				hasChanges = true
			}

			// Auth changes
			authSlice, _ := cmd.Flags().GetStringSlice("auth")
			if len(authSlice) > 0 {
				auth := make(map[string]any)
				for _, a := range authSlice {
					parts := strings.SplitN(a, "=", 2)
					if len(parts) == 2 {
						auth[parts[0]] = parts[1]
					}
				}
				req.AuthConfig = &auth
				hasChanges = true
			}

			if !hasChanges {
				PrintWarning("No changes specified")
				return nil
			}

			if _, err := client.UpdateProvider(id, req); err != nil {
				return fmt.Errorf("failed to update provider: %w", err)
			}

			PrintSuccess("Provider updated")
			return nil
		},
	}
	editCmd.Flags().String("name", "", "New name")
	editCmd.Flags().StringSlice("config", nil, "Update config (key=value)")
	editCmd.Flags().StringSlice("auth", nil, "Update auth (key=value)")

	// delete subcommand
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			if err := client.DeleteProvider(id); err != nil {
				return fmt.Errorf("failed to delete provider: %w", err)
			}

			PrintSuccess("Provider deleted")
			return nil
		},
	}

	// test subcommand
	testCmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Test a provider connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			result, err := client.TestProvider(id)
			if err != nil {
				return fmt.Errorf("test failed: %w", err)
			}

			if tryJSON(cmd, result) {
				return nil
			}

			if result.Success {
				PrintSuccess(fmt.Sprintf("%s (%.0fms)", result.Message, result.ResponseTime))
			} else {
				PrintError(fmt.Sprintf("%s (%.0fms)", result.Message, result.ResponseTime))
			}
			return nil
		},
	}

	// enable subcommand
	enableCmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			if _, err := client.EnableProvider(id); err != nil {
				return fmt.Errorf("failed to enable provider: %w", err)
			}

			PrintSuccess("Provider enabled")
			return nil
		},
	}

	// disable subcommand
	disableCmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			if _, err := client.DisableProvider(id); err != nil {
				return fmt.Errorf("failed to disable provider: %w", err)
			}

			PrintSuccess("Provider disabled")
			return nil
		},
	}

	// set-default subcommand
	setDefaultCmd := &cobra.Command{
		Use:   "set-default <id>",
		Short: "Set the default provider for its type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid provider ID: %w", err)
			}

			p, err := client.SetDefaultProvider(id)
			if err != nil {
				return fmt.Errorf("failed to set default provider: %w", err)
			}

			PrintSuccess(fmt.Sprintf("Provider '%s' set as default for %s", p.Name, p.Type))
			return nil
		},
	}

	// models subcommand
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "List available models for a provider service",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, _ := cmd.Flags().GetString("service")
			providerType, _ := cmd.Flags().GetString("type")
			projectID, _ := cmd.Flags().GetString("project")

			if service == "" {
				return fmt.Errorf("service is required (e.g. vertex_ai_llm)")
			}

			models, err := client.ListProviderModels(service, providerType, projectID)
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			if tryJSON(cmd, models) {
				return nil
			}

			fmt.Println()
			title := fmt.Sprintf("Available Models (%s)", service)
			fmt.Printf("  %s\n", titleStyle.Render(title))

			headers := []string{"ID", "Name", "Input Cost", "Output Cost"}
			var rows [][]string

			for _, m := range models {
				inputCost := "-"
				outputCost := "-"
				if m.Cost != nil {
					inputCost = fmt.Sprintf("$%.2f", m.Cost.Input)
					outputCost = fmt.Sprintf("$%.2f", m.Cost.Output)
				}

				rows = append(rows, []string{
					m.ID,
					m.DisplayName,
					inputCost,
					outputCost,
				})
			}

			RenderTable(headers, rows)
			fmt.Println()

			return nil
		},
	}
	modelsCmd.Flags().String("service", "", "Service name (required)")
	modelsCmd.Flags().String("type", "llm", "Provider type")
	modelsCmd.Flags().String("project", "", "Google Cloud Project ID")

	cmd.AddCommand(listCmd)
	cmd.AddCommand(createCmd)
	cmd.AddCommand(editCmd)
	cmd.AddCommand(deleteCmd)
	cmd.AddCommand(testCmd)
	cmd.AddCommand(enableCmd)
	cmd.AddCommand(disableCmd)
	cmd.AddCommand(setDefaultCmd)
	cmd.AddCommand(modelsCmd)

	return cmd
}

func listProviders(cmd *cobra.Command, client *api.Client) error {
	typeFilter, _ := cmd.Flags().GetString("type")
	providers, err := client.ListProviders(typeFilter)
	if err != nil {
		return fmt.Errorf("failed to list providers: %w", err)
	}

	if tryJSON(cmd, providers) {
		return nil
	}

	if len(providers) == 0 {
		fmt.Println("No providers configured.")
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s\n", titleStyle.Render("Providers"))

	headers := []string{"ID", "Name", "Service", "Type", "Status", "Default"}
	var rows [][]string

	for _, p := range providers {
		status := "disabled"
		if p.Enabled {
			status = "enabled"
		}

		isDefault := ""
		if p.IsDefault {
			isDefault = "*"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", p.ID),
			p.Name,
			p.Service,
			p.Type,
			status,
			isDefault,
		})
	}

	RenderTable(headers, rows)
	fmt.Println()

	return nil
}
