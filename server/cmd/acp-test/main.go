package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/diane-assistant/diane/internal/acp"
)

func main() {
	// Use the diane project directory so OpenCode picks up the config
	workDir := "/Users/mcj/code/emgt/diane"

	client, err := acp.NewStdioClient("opencode", []string{"acp"}, workDir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Initializing...")
	if err := client.Initialize(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized: %+v\n", client.GetAgentInfo())

	fmt.Println("Creating session...")
	// Model is configured in opencode.json in the project directory
	sessionID, err := client.NewSession(ctx, workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Session created: %s\n", sessionID)

	fmt.Println("Sending prompt...")
	var outputText string
	result, err := client.Prompt(ctx, sessionID, "What is 2+2? Just answer with the number.", func(update *acp.SessionUpdateParams) {
		// Print raw update for debugging
		raw, _ := json.MarshalIndent(update, "", "  ")
		fmt.Printf("Raw Update:\n%s\n\n", string(raw))

		if update.Update.SessionUpdate == "agent_message_chunk" && update.Update.Content != nil {
			if update.Update.Content.Type == "text" {
				outputText += update.Update.Content.Text
			}
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send prompt: %v\n", err)
		os.Exit(1)
	}

	// Give time for remaining notifications
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("Stop reason: %s\n", result.StopReason)
	fmt.Printf("Output: %s\n", outputText)
}
