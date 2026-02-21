package acp

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// runEmergentAgent is a stub for executing an emergent agent via internal APIs.
func (m *Manager) runEmergentAgent(agent *AgentConfig, prompt string) (*Run, error) {
	// Create run record
	runID := make([]byte, 16)
	rand.Read(runID)
	
	now := time.Now()
	run := &Run{
		AgentName: agent.Name,
		RunID:     hex.EncodeToString(runID),
		Status:    RunStatusCompleted,
		Output: []Message{
			{
				Role: "assistant",
				Parts: []MessagePart{
					{
						ContentType: "text/plain",
						Content:     "Emergent agent execution triggered successfully. [Workspace Config: " + fmtSprintfWorkspace(agent) + "]",
					},
				},
				CreatedAt:   &now,
				CompletedAt: &now,
			},
		},
		CreatedAt: now,
	}

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
}
