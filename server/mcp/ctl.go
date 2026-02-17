package main

// ctl.go contains the CLI control commands.
// All CLI dispatch is now handled by Cobra commands in internal/cli/.
// This file retains only the runCTLCommand entry point that delegates to Cobra.

import (
	"fmt"
	"os"

	"github.com/diane-assistant/diane/internal/api"
	"github.com/diane-assistant/diane/internal/cli"
)

// runCTLCommand dispatches a control subcommand via Cobra.
// Returns true if the command was handled (caller should exit),
// false if not recognized (caller should fall through to stdio MCP mode).
func runCTLCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}

	cmd := args[1]

	// "serve" is handled in main(), not here
	if cmd == "serve" {
		return false
	}

	// Delegate to Cobra CLI
	client := api.NewClient()
	rootCmd := cli.NewRootCmd(client, Version)
	rootCmd.SetArgs(args[1:])

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	return true
}
