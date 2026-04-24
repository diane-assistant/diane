package emergent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AgentQuestionOption is a predefined answer option for an agent question.
type AgentQuestionOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// AgentQuestion represents a question asked by an emergent agent during a run.
type AgentQuestion struct {
	ID          string                `json:"id"`
	RunID       string                `json:"runId"`
	AgentID     string                `json:"agentId"`
	Question    string                `json:"question"`
	Options     []AgentQuestionOption `json:"options"`
	Status      string                `json:"status"`
	CreatedAt   time.Time             `json:"createdAt"`
	RespondedAt *time.Time            `json:"respondedAt,omitempty"`
}

// QuestionsService makes direct HTTP calls to the Emergent questions REST API.
// It is separate from the graph SDK client because the questions endpoints are
// not part of the Emergent graph API.
type QuestionsService struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	projectID  string
}

// tokenInfoResponse matches the relevant fields of GET /api/auth/me.
type tokenInfoResponse struct {
	ProjectID string `json:"project_id"`
}

// emergentAPIResponse is the generic envelope used by Emergent list endpoints.
type emergentAPIResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewQuestionsService creates a QuestionsService from the given Config.
// If cfg.ProjectID is empty it calls GET /api/auth/me to resolve it.
func NewQuestionsService(cfg *Config) (*QuestionsService, error) {
	s := &QuestionsService{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		projectID:  cfg.ProjectID,
	}

	if s.projectID == "" {
		if err := s.resolveProjectID(); err != nil {
			return nil, fmt.Errorf("failed to resolve Emergent project ID: %w", err)
		}
		// Cache resolved ID back into cfg so callers don't resolve it again.
		cfg.ProjectID = s.projectID
	}

	return s, nil
}

// resolveProjectID fetches the project ID from GET /api/auth/me.
func (s *QuestionsService) resolveProjectID() error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.baseURL+"/api/auth/me", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /api/auth/me returned status %d", resp.StatusCode)
	}

	var info tokenInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return err
	}
	if info.ProjectID == "" {
		return fmt.Errorf("GET /api/auth/me returned empty project_id")
	}
	s.projectID = info.ProjectID
	return nil
}

// ListQuestions returns agent questions for the project, filtered by status.
// Pass status="" to return all questions. Typical value is "pending".
func (s *QuestionsService) ListQuestions(ctx context.Context, status string) ([]AgentQuestion, error) {
	u := fmt.Sprintf("%s/api/projects/%s/agent-questions", s.baseURL, s.projectID)
	if status != "" {
		u += "?status=" + status
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list questions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list questions returned status %d", resp.StatusCode)
	}

	var envelope emergentAPIResponse[[]AgentQuestion]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode questions response: %w", err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("list questions failed: %s", envelope.Error)
	}
	if envelope.Data == nil {
		return []AgentQuestion{}, nil
	}
	return envelope.Data, nil
}

// RespondToQuestion submits an answer for the given question ID.
// Returns nil on HTTP 202. Maps HTTP 404 and 409 to descriptive errors.
func (s *QuestionsService) RespondToQuestion(ctx context.Context, questionID, response string) error {
	u := fmt.Sprintf("%s/api/projects/%s/agent-questions/%s/respond", s.baseURL, s.projectID, questionID)

	body, err := json.Marshal(map[string]string{"response": response})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to respond to question: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("question %s not found", questionID)
	case http.StatusConflict:
		return fmt.Errorf("question %s is already answered or the run is not paused", questionID)
	default:
		return fmt.Errorf("respond to question returned status %d", resp.StatusCode)
	}
}
