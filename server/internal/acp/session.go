package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/diane-assistant/diane/internal/store"
)

// SessionStatus represents the lifecycle state of an ACP session.
type SessionStatus string

const (
	SessionActive       SessionStatus = "active"
	SessionIdle         SessionStatus = "idle"
	SessionDisconnected SessionStatus = "disconnected"
	SessionClosed       SessionStatus = "closed"
)

// DefaultIdleTimeout is the duration after which an idle session is reaped.
const DefaultIdleTimeout = 30 * time.Minute

// SessionState holds the runtime state for an active ACP session.
// This is the in-memory representation; the persistent counterpart is store.ACPSession.
type SessionState struct {
	SessionID    string
	AgentName    string
	AgentKey     string
	WorkDir      string
	Client       *StdioClient
	Status       SessionStatus
	TurnCount    int
	CreatedAt    time.Time
	LastActiveAt time.Time
	ModelID      string
	ModeID       string
	Title        string
	Models       *ModelsInfo
	Modes        *ModesInfo
}

// SessionInfo is the JSON-serializable snapshot of a session returned by the API.
type SessionInfo struct {
	SessionID    string        `json:"session_id"`
	AgentName    string        `json:"agent_name"`
	WorkDir      string        `json:"workdir"`
	Status       SessionStatus `json:"status"`
	TurnCount    int           `json:"turn_count"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActiveAt time.Time     `json:"last_active_at"`
	ModelID      string        `json:"model_id,omitempty"`
	ModeID       string        `json:"mode_id,omitempty"`
	Title        string        `json:"title,omitempty"`
	Models       *ModelsInfo   `json:"models,omitempty"`
	Modes        *ModesInfo    `json:"modes,omitempty"`
}

// Info returns a JSON-serializable snapshot of the session.
func (s *SessionState) Info() *SessionInfo {
	return &SessionInfo{
		SessionID:    s.SessionID,
		AgentName:    s.AgentName,
		WorkDir:      s.WorkDir,
		Status:       s.Status,
		TurnCount:    s.TurnCount,
		CreatedAt:    s.CreatedAt,
		LastActiveAt: s.LastActiveAt,
		ModelID:      s.ModelID,
		ModeID:       s.ModeID,
		Title:        s.Title,
		Models:       s.Models,
		Modes:        s.Modes,
	}
}

// ---------------------------------------------------------------------------
// Manager session wiring
// ---------------------------------------------------------------------------

// SetSessionStore sets the session persistence backend.
// Called after NewManager in the API server setup.
func (m *Manager) SetSessionStore(s store.ACPSessionStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionStore = s
}

// InitSessions marks previously-live sessions as disconnected (subprocess is gone)
// and starts the idle reaper. Call once after SetSessionStore on startup.
func (m *Manager) InitSessions(ctx context.Context) {
	if m.sessionStore == nil {
		return
	}
	if err := m.sessionStore.MarkDisconnected(ctx); err != nil {
		slog.Warn("failed to mark sessions disconnected on startup", "error", err)
	}
	m.StartIdleReaper()
}

// ---------------------------------------------------------------------------
// Session lifecycle
// ---------------------------------------------------------------------------

// StartSession creates a new multi-turn session with an ACP agent.
// It spawns (or reuses) the agent subprocess, creates an ACP session,
// and persists the metadata.
func (m *Manager) StartSession(agentName string, workDir string, title string) (*SessionInfo, error) {
	if m.sessionStore == nil {
		return nil, fmt.Errorf("session store not configured")
	}

	agent, err := m.GetAgent(agentName)
	if err != nil {
		return nil, err
	}
	if !agent.Enabled {
		return nil, fmt.Errorf("agent '%s' is disabled", agentName)
	}

	// Resolve working directory.
	cwd := workDir
	if cwd == "" {
		cwd = agent.WorkDir
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	agentKey := agent.UniqueKey()

	// Get or spawn a live StdioClient.
	client, err := m.getOrCreateStdioClient(agent)
	if err != nil {
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	// Create the ACP session on the agent subprocess.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessionResult, err := client.NewSessionWithInfo(ctx, cwd)
	if err != nil {
		return nil, fmt.Errorf("session/new failed: %w", err)
	}

	now := time.Now().UTC()
	state := &SessionState{
		SessionID:    sessionResult.SessionID,
		AgentName:    agentName,
		AgentKey:     agentKey,
		WorkDir:      cwd,
		Client:       client,
		Status:       SessionActive,
		TurnCount:    0,
		CreatedAt:    now,
		LastActiveAt: now,
		Title:        title,
		Models:       sessionResult.Models,
		Modes:        sessionResult.Modes,
	}
	if sessionResult.Models != nil {
		state.ModelID = sessionResult.Models.CurrentModelID
	}
	if sessionResult.Modes != nil {
		state.ModeID = sessionResult.Modes.CurrentModeID
	}

	// Store in memory.
	m.sessionsMu.Lock()
	m.sessions[sessionResult.SessionID] = state
	m.sessionsMu.Unlock()

	// Persist to Emergent (best-effort).
	bgCtx := context.Background()
	if err := m.sessionStore.CreateSession(bgCtx, &store.ACPSession{
		SessionID:    state.SessionID,
		AgentName:    state.AgentName,
		AgentKey:     state.AgentKey,
		WorkDir:      state.WorkDir,
		Status:       string(state.Status),
		TurnCount:    state.TurnCount,
		CreatedAt:    state.CreatedAt,
		LastActiveAt: state.LastActiveAt,
		ModelID:      state.ModelID,
		ModeID:       state.ModeID,
		Title:        state.Title,
	}); err != nil {
		slog.Warn("failed to persist new session", "session_id", state.SessionID, "error", err)
	}

	slog.Info("session started",
		"session_id", state.SessionID,
		"agent", agentName,
		"workdir", cwd,
	)

	return state.Info(), nil
}

// PromptSession sends a prompt to an existing session and returns the response.
// The full prompt+response (including tool calls and timing) is persisted as an
// ACPSessionMessage.
func (m *Manager) PromptSession(sessionID string, prompt string) (*Run, error) {
	if m.sessionStore == nil {
		return nil, fmt.Errorf("session store not configured")
	}

	// Look up in-memory session.
	m.sessionsMu.RLock()
	state, ok := m.sessions[sessionID]
	m.sessionsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session '%s' not found (may have been closed or expired)", sessionID)
	}

	if state.Status == SessionClosed {
		return nil, fmt.Errorf("session '%s' is closed", sessionID)
	}
	if state.Status == SessionDisconnected {
		return nil, fmt.Errorf("session '%s' is disconnected (agent process died)", sessionID)
	}

	// Mark active and bump turn.
	state.Status = SessionActive
	state.TurnCount++
	turnNumber := state.TurnCount
	startTime := time.Now()

	// Collect streamed output.
	var outputText string
	var toolCalls []store.ACPToolCall

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, promptErr := state.Client.Prompt(ctx, sessionID, prompt, func(update *SessionUpdateParams) {
		switch update.Update.SessionUpdate {
		case "agent_message_chunk":
			if update.Update.Content != nil && update.Update.Content.Type == "text" {
				outputText += update.Update.Content.Text
			}
		case "tool_call":
			toolCalls = append(toolCalls, store.ACPToolCall{
				ToolCallID: update.Update.ToolCallID,
				Title:      update.Update.Title,
				Kind:       update.Update.Kind,
				Status:     update.Update.Status,
			})
		}
	})

	durationMs := int(time.Since(startTime).Milliseconds())
	state.LastActiveAt = time.Now().UTC()

	// Build the Run response.
	runID := make([]byte, 16)
	rand.Read(runID)
	run := &Run{
		AgentName:  state.AgentName,
		SessionID:  sessionID,
		RunID:      hex.EncodeToString(runID),
		TurnNumber: turnNumber,
		CreatedAt:  startTime,
	}

	// Build the persistent message record.
	msgID := make([]byte, 16)
	rand.Read(msgID)
	msg := &store.ACPSessionMessage{
		MessageID:  hex.EncodeToString(msgID),
		SessionID:  sessionID,
		TurnNumber: turnNumber,
		Prompt:     prompt,
		Response:   outputText,
		ToolCalls:  toolCalls,
		DurationMs: durationMs,
		CreatedAt:  startTime.UTC(),
	}

	if promptErr != nil {
		finishedAt := time.Now()
		run.FinishedAt = &finishedAt
		run.Status = RunStatusFailed
		run.Error = &Error{
			Code:    "prompt_error",
			Message: fmt.Sprintf("prompt failed: %v", promptErr),
		}
		msg.Error = promptErr.Error()
	} else {
		finishedAt := time.Now()
		run.FinishedAt = &finishedAt
		switch result.StopReason {
		case "end_turn":
			run.Status = RunStatusCompleted
		case "cancelled":
			run.Status = RunStatusCancelled
		default:
			run.Status = RunStatusCompleted
		}
		run.Output = []Message{NewTextMessage("agent", outputText)}
		msg.StopReason = result.StopReason
	}

	// Mark idle after prompt completes.
	state.Status = SessionIdle

	// Persist message and update session metadata (best-effort).
	bgCtx := context.Background()
	if err := m.sessionStore.AddMessage(bgCtx, msg); err != nil {
		slog.Warn("failed to persist session message",
			"session_id", sessionID, "message_id", msg.MessageID, "error", err)
	}
	if err := m.sessionStore.UpdateSession(bgCtx, sessionID, map[string]interface{}{
		"turn_count":     state.TurnCount,
		"last_active_at": state.LastActiveAt.Format(time.RFC3339Nano),
		"status":         string(state.Status),
	}); err != nil {
		slog.Warn("failed to update session metadata",
			"session_id", sessionID, "error", err)
	}

	return run, nil
}

// GetSessionInfo returns a session's current state.
// Checks in-memory first, falls back to persistent store.
func (m *Manager) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	m.sessionsMu.RLock()
	state, ok := m.sessions[sessionID]
	m.sessionsMu.RUnlock()
	if ok {
		return state.Info(), nil
	}

	// Fall back to persistent store for historical/disconnected sessions.
	if m.sessionStore != nil {
		ctx := context.Background()
		stored, err := m.sessionStore.GetSession(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up session: %w", err)
		}
		if stored == nil {
			return nil, fmt.Errorf("session '%s' not found", sessionID)
		}
		return &SessionInfo{
			SessionID:    stored.SessionID,
			AgentName:    stored.AgentName,
			WorkDir:      stored.WorkDir,
			Status:       SessionStatus(stored.Status),
			TurnCount:    stored.TurnCount,
			CreatedAt:    stored.CreatedAt,
			LastActiveAt: stored.LastActiveAt,
			ModelID:      stored.ModelID,
			ModeID:       stored.ModeID,
			Title:        stored.Title,
		}, nil
	}

	return nil, fmt.Errorf("session '%s' not found", sessionID)
}

// ListSessionsFromStore returns sessions from persistent storage.
func (m *Manager) ListSessionsFromStore(agentName string, status string) ([]*SessionInfo, error) {
	if m.sessionStore == nil {
		return nil, fmt.Errorf("session store not configured")
	}

	ctx := context.Background()
	sessions, err := m.sessionStore.ListSessions(ctx, agentName, status)
	if err != nil {
		return nil, err
	}

	result := make([]*SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, &SessionInfo{
			SessionID:    s.SessionID,
			AgentName:    s.AgentName,
			WorkDir:      s.WorkDir,
			Status:       SessionStatus(s.Status),
			TurnCount:    s.TurnCount,
			CreatedAt:    s.CreatedAt,
			LastActiveAt: s.LastActiveAt,
			ModelID:      s.ModelID,
			ModeID:       s.ModeID,
			Title:        s.Title,
		})
	}
	return result, nil
}

// CloseSession gracefully closes a session.
// If this was the last session on the agent subprocess, the subprocess is killed.
func (m *Manager) CloseSession(sessionID string) error {
	m.sessionsMu.Lock()
	state, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.sessionsMu.Unlock()

	if !ok {
		// Still update the store.
		if m.sessionStore != nil {
			ctx := context.Background()
			return m.sessionStore.UpdateSession(ctx, sessionID, map[string]interface{}{
				"status": string(SessionClosed),
			})
		}
		return fmt.Errorf("session '%s' not found", sessionID)
	}

	state.Status = SessionClosed

	// Check if any other in-memory sessions share this client.
	m.sessionsMu.RLock()
	clientInUse := false
	for _, s := range m.sessions {
		if s.AgentKey == state.AgentKey && s.Client == state.Client {
			clientInUse = true
			break
		}
	}
	m.sessionsMu.RUnlock()

	// If no other sessions use this client, tear down the subprocess.
	if !clientInUse && state.Client != nil {
		slog.Info("closing agent subprocess (last session closed)",
			"agent_key", state.AgentKey,
			"session_id", sessionID,
		)
		state.Client.Close()

		m.mu.Lock()
		if existing, ok := m.stdioClients[state.AgentKey]; ok && existing == state.Client {
			delete(m.stdioClients, state.AgentKey)
		}
		m.mu.Unlock()
	}

	// Update persistent store.
	if m.sessionStore != nil {
		ctx := context.Background()
		if err := m.sessionStore.UpdateSession(ctx, sessionID, map[string]interface{}{
			"status": string(SessionClosed),
		}); err != nil {
			slog.Warn("failed to update session status in store",
				"session_id", sessionID, "error", err)
		}
	}

	slog.Info("session closed", "session_id", sessionID, "agent", state.AgentName)
	return nil
}

// SetSessionConfig changes a configuration option (model, mode, etc.) on a live session.
func (m *Manager) SetSessionConfig(sessionID string, configID string, value string) error {
	m.sessionsMu.RLock()
	state, ok := m.sessions[sessionID]
	m.sessionsMu.RUnlock()
	if !ok {
		return fmt.Errorf("session '%s' not found", sessionID)
	}
	if state.Client == nil {
		return fmt.Errorf("session '%s' has no active client", sessionID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := state.Client.SetConfigOption(ctx, sessionID, configID, value); err != nil {
		return fmt.Errorf("set config option failed: %w", err)
	}

	// Update local state.
	switch configID {
	case "model":
		state.ModelID = value
	case "mode":
		state.ModeID = value
	}

	// Update persistent store.
	if m.sessionStore != nil {
		bgCtx := context.Background()
		updates := map[string]interface{}{}
		switch configID {
		case "model":
			updates["model_id"] = value
		case "mode":
			updates["mode_id"] = value
		}
		if len(updates) > 0 {
			_ = m.sessionStore.UpdateSession(bgCtx, sessionID, updates)
		}
	}

	return nil
}

// GetSessionMessages returns the full message history for a session.
func (m *Manager) GetSessionMessages(sessionID string) ([]*store.ACPSessionMessage, error) {
	if m.sessionStore == nil {
		return nil, fmt.Errorf("session store not configured")
	}
	ctx := context.Background()
	return m.sessionStore.GetMessages(ctx, sessionID)
}

// ---------------------------------------------------------------------------
// Idle reaper
// ---------------------------------------------------------------------------

// CleanupIdleSessions closes sessions that have exceeded the idle timeout.
func (m *Manager) CleanupIdleSessions() {
	m.sessionsMu.RLock()
	var toClose []string
	for id, s := range m.sessions {
		if s.Status != SessionClosed && time.Since(s.LastActiveAt) > m.idleTimeout {
			toClose = append(toClose, id)
		}
	}
	m.sessionsMu.RUnlock()

	for _, id := range toClose {
		slog.Info("closing idle session", "session_id", id)
		m.CloseSession(id)
	}
}

// StartIdleReaper starts a background goroutine that cleans up idle sessions every 5 minutes.
func (m *Manager) StartIdleReaper() {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.reaperCancel = cancel
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.CleanupIdleSessions()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopIdleReaper stops the background idle cleanup goroutine.
func (m *Manager) StopIdleReaper() {
	m.mu.Lock()
	if m.reaperCancel != nil {
		m.reaperCancel()
		m.reaperCancel = nil
	}
	m.mu.Unlock()
}

// CloseAllSessions closes all active sessions. Called on shutdown.
func (m *Manager) CloseAllSessions() {
	m.sessionsMu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.sessionsMu.Unlock()

	for _, id := range ids {
		m.CloseSession(id)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// getOrCreateStdioClient returns an existing StdioClient for the agent or spawns a new one.
// The client is stored in m.stdioClients for reuse across sessions.
func (m *Manager) getOrCreateStdioClient(agent *AgentConfig) (*StdioClient, error) {
	key := agent.UniqueKey()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing live client.
	if client, ok := m.stdioClients[key]; ok {
		if err := client.Ping(context.Background()); err == nil {
			return client, nil
		}
		slog.Info("existing agent client is dead, respawning", "agent_key", key)
		client.Close()
		delete(m.stdioClients, key)
	}

	// Determine command and args.
	command := agent.Command
	args := agent.Args
	if command == "" {
		command = agent.Name
		args = getACPArgsForAgent(agent.Name)
	}

	cmdPath, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("command not found: %s", command)
	}

	client, err := NewStdioClient(cmdPath, args, agent.WorkDir, agent.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to start agent process: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	m.stdioClients[key] = client
	return client, nil
}
