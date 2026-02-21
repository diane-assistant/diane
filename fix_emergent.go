package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("server/internal/acp/emergent.go")
	if err != nil {
		panic(err)
	}
	str := string(content)

	oldBody := `		run.Status = RunStatusCompleted
		run.Output = append(run.Output, Message{
			Role: "assistant",
			Parts: []MessagePart{
				{
					ContentType: "text/plain",
					Content:     "Emergent agent execution triggered successfully. [Workspace Config: " + fmtSprintfWorkspace(agent) + "]",
				},
			},
			CreatedAt:   time.Now(),
			CompletedAt: time.Now(),
		})
	}()

	return run, nil
}

// testEmergentAgent is a stub for testing an emergent agent connection
func (m *Manager) testEmergentAgent(agent *AgentConfig, result *AgentTestResult) (*AgentTestResult, error) {
	// Emergent agents run on demand in sandboxes, they are always "connected" implicitly if configured.
	result.Status = "connected"
	result.Version = "emergent-sandbox"
	return result, nil
}

func fmtSprintfWorkspace(a *AgentConfig) string {
	if a.WorkspaceConfig == nil {
		return "none"
	}
	return a.WorkspaceConfig.BaseImage + " @ " + a.WorkspaceConfig.RepoURL
}`

	newBody := `		run.Status = RunStatusCompleted
		now := time.Now()
		run.Output = append(run.Output, Message{
			Role: "assistant",
			Parts: []MessagePart{
				{
					ContentType: "text/plain",
					Content:     "Emergent agent execution triggered successfully. [Workspace Config: " + fmtSprintfWorkspace(agent) + "]",
				},
			},
			CreatedAt:   &now,
			CompletedAt: &now,
		})
	}()

	return run, nil
}

// testEmergentAgent is a stub for testing an emergent agent connection
func (m *Manager) testEmergentAgent(agent *AgentConfig, result *AgentTestResult) (*AgentTestResult, error) {
	// Emergent agents run on demand in sandboxes, they are always "connected" implicitly if configured.
	result.Status = "connected"
	version := "emergent-sandbox"
	result.Version = &version
	return result, nil
}

func fmtSprintfWorkspace(a *AgentConfig) string {
	if a.WorkspaceConfig == nil {
		return "none"
	}
	return a.WorkspaceConfig.BaseImage + " @ " + a.WorkspaceConfig.RepoURL
}`

	str = strings.Replace(str, oldBody, newBody, 1)

	err = ioutil.WriteFile("server/internal/acp/emergent.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Fixed emergent.go")
}
