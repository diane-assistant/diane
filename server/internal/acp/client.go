// Package acp provides a client for the Agent Communication Protocol (ACP).
// ACP is an open protocol for agent interoperability under the Linux Foundation.
// Spec: https://agentcommunicationprotocol.dev
package acp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an ACP client for communicating with ACP-compliant agents.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new ACP client.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute, // Agent runs can be long
		},
	}
}

// Error represents an ACP error response.
type Error struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("ACP error [%s]: %s", e.Code, e.Message)
}

// AgentManifest represents an agent's manifest/description.
type AgentManifest struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	InputContentTypes  []string `json:"input_content_types"`
	OutputContentTypes []string `json:"output_content_types"`
	Metadata           Metadata `json:"metadata,omitempty"`
	Status             *Status  `json:"status,omitempty"`
}

// Metadata contains static agent metadata.
type Metadata struct {
	Documentation       string   `json:"documentation,omitempty"`
	License             string   `json:"license,omitempty"`
	ProgrammingLanguage string   `json:"programming_language,omitempty"`
	NaturalLanguages    []string `json:"natural_languages,omitempty"`
	Framework           string   `json:"framework,omitempty"`
	Tags                []string `json:"tags,omitempty"`
}

// Status contains runtime agent metrics.
type Status struct {
	AvgRunTokens      float64 `json:"avg_run_tokens,omitempty"`
	AvgRunTimeSeconds float64 `json:"avg_run_time_seconds,omitempty"`
	SuccessRate       float64 `json:"success_rate,omitempty"`
}

// MessagePart represents a part of a message.
type MessagePart struct {
	Name            string      `json:"name,omitempty"`
	ContentType     string      `json:"content_type"`
	Content         string      `json:"content,omitempty"`
	ContentEncoding string      `json:"content_encoding,omitempty"`
	ContentURL      string      `json:"content_url,omitempty"`
	Metadata        interface{} `json:"metadata,omitempty"`
}

// Message represents a message in ACP.
type Message struct {
	Role        string        `json:"role"`
	Parts       []MessagePart `json:"parts"`
	CreatedAt   *time.Time    `json:"created_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// NewTextMessage creates a simple text message.
func NewTextMessage(role, text string) Message {
	return Message{
		Role: role,
		Parts: []MessagePart{
			{
				ContentType: "text/plain",
				Content:     text,
			},
		},
	}
}

// NewUserMessage creates a user message with text content.
func NewUserMessage(text string) Message {
	return NewTextMessage("user", text)
}

// RunStatus represents the status of a run.
type RunStatus string

const (
	RunStatusCreated    RunStatus = "created"
	RunStatusInProgress RunStatus = "in-progress"
	RunStatusAwaiting   RunStatus = "awaiting"
	RunStatusCancelling RunStatus = "cancelling"
	RunStatusCancelled  RunStatus = "cancelled"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusFailed     RunStatus = "failed"
)

// RunMode represents the execution mode.
type RunMode string

const (
	RunModeSync   RunMode = "sync"
	RunModeAsync  RunMode = "async"
	RunModeStream RunMode = "stream"
)

// Run represents an agent run.
type Run struct {
	AgentName    string     `json:"agent_name"`
	SessionID    string     `json:"session_id,omitempty"`
	RunID        string     `json:"run_id"`
	TurnNumber   int        `json:"turn_number,omitempty"`
	Status       RunStatus  `json:"status"`
	AwaitRequest *string    `json:"await_request,omitempty"`
	Output       []Message  `json:"output"`
	Error        *Error     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// GetTextOutput returns the concatenated text output from the run.
func (r *Run) GetTextOutput() string {
	var result string
	for _, msg := range r.Output {
		for _, part := range msg.Parts {
			if part.ContentType == "text/plain" || part.ContentType == "" {
				result += part.Content
			}
		}
	}
	return result
}

// RunCreateRequest is the request to create a new run.
type RunCreateRequest struct {
	AgentName string    `json:"agent_name"`
	SessionID string    `json:"session_id,omitempty"`
	Input     []Message `json:"input"`
	Mode      RunMode   `json:"mode,omitempty"`
}

// AgentsListResponse is the response from listing agents.
type AgentsListResponse struct {
	Agents []AgentManifest `json:"agents"`
}

// Ping checks if the ACP server is reachable.
func (c *Client) Ping() error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/ping")
	if err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status %d", resp.StatusCode)
	}
	return nil
}

// ListAgents returns a list of available agents.
func (c *Client) ListAgents(limit, offset int) ([]AgentManifest, error) {
	url := fmt.Sprintf("%s/agents?limit=%d&offset=%d", c.BaseURL, limit, offset)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result AgentsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Agents, nil
}

// GetAgent returns a specific agent's manifest.
func (c *Client) GetAgent(name string) (*AgentManifest, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/agents/" + name)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var agent AgentManifest
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &agent, nil
}

// CreateRun creates and starts a new run for the specified agent.
// For sync mode, this blocks until the run completes.
// For async mode, returns immediately with the run status.
func (c *Client) CreateRun(req RunCreateRequest) (*Run, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/runs",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, c.parseError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &run, nil
}

// GetRun returns the current status of a run.
func (c *Client) GetRun(runID string) (*Run, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/runs/" + runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &run, nil
}

// CancelRun cancels a running run.
func (c *Client) CancelRun(runID string) (*Run, error) {
	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/runs/"+runID+"/cancel",
		"application/json",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, c.parseError(resp)
	}

	var run Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &run, nil
}

// RunSync creates a run in sync mode and waits for completion.
// This is a convenience method for simple request-response patterns.
func (c *Client) RunSync(agentName string, prompt string) (*Run, error) {
	return c.CreateRun(RunCreateRequest{
		AgentName: agentName,
		Input:     []Message{NewUserMessage(prompt)},
		Mode:      RunModeSync,
	})
}

// RunAsync creates a run in async mode and returns immediately.
// Use GetRun to poll for status updates.
func (c *Client) RunAsync(agentName string, prompt string) (*Run, error) {
	return c.CreateRun(RunCreateRequest{
		AgentName: agentName,
		Input:     []Message{NewUserMessage(prompt)},
		Mode:      RunModeAsync,
	})
}

// WaitForCompletion polls a run until it reaches a terminal state.
func (c *Client) WaitForCompletion(runID string, pollInterval time.Duration, timeout time.Duration) (*Run, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		run, err := c.GetRun(runID)
		if err != nil {
			return nil, err
		}

		switch run.Status {
		case RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
			return run, nil
		case RunStatusAwaiting:
			return run, fmt.Errorf("run is awaiting input, cannot auto-complete")
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("timeout waiting for run to complete")
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var acpErr Error
	if err := json.Unmarshal(body, &acpErr); err == nil && acpErr.Code != "" {
		return &acpErr
	}

	return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
}
