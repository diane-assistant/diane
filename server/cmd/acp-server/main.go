package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/diane-assistant/diane/internal/acp"
)

func main() {
	var (
		agentType   = flag.String("type", "opencode", "Agent type: opencode, gemini, or custom")
		port        = flag.Int("port", 8100, "Port to listen on")
		workDir     = flag.String("workdir", "", "Working directory for the agent")
		command     = flag.String("command", "", "Custom command (for type=custom)")
		args        = flag.String("args", "", "Custom arguments (comma-separated, for type=custom)")
		name        = flag.String("name", "", "Agent name (for type=custom)")
		description = flag.String("description", "", "Agent description (for type=custom)")
	)
	flag.Parse()

	var config acp.LocalAgentConfig

	switch *agentType {
	case "opencode":
		config = acp.OpenCodeConfig(*port, *workDir)
	case "gemini":
		config = acp.GeminiCLIConfig(*port, *workDir)
	case "custom":
		if *command == "" || *name == "" {
			fmt.Fprintln(os.Stderr, "Error: -command and -name are required for custom agent type")
			flag.Usage()
			os.Exit(1)
		}
		var argsList []string
		if *args != "" {
			for _, a := range splitArgs(*args) {
				argsList = append(argsList, a)
			}
		}
		config = acp.LocalAgentConfig{
			Name:        *name,
			Command:     *command,
			Args:        argsList,
			Description: *description,
			Port:        *port,
			WorkDir:     *workDir,
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown agent type: %s\n", *agentType)
		os.Exit(1)
	}

	// Override port if specified
	if *port != 8100 {
		config.Port = *port
	}

	// Override workdir if specified
	if *workDir != "" {
		config.WorkDir = *workDir
	}

	server := acp.NewLocalAgentServer(config)

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		server.Stop()
		os.Exit(0)
	}()

	fmt.Printf("Starting ACP server for %s on port %d\n", config.Name, config.Port)
	fmt.Printf("Test with: curl http://localhost:%d/ping\n", config.Port)
	fmt.Printf("List agents: curl http://localhost:%d/agents\n", config.Port)
	fmt.Println("")

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range splitComma(s) {
		part = trimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitComma(s string) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
