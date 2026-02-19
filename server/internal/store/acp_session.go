package store

import (
	"context"
	"time"
)

// ACPSession represents a persistent ACP agent session stored in Emergent.
type ACPSession struct {
	SessionID    string    `json:"session_id"`
	AgentName    string    `json:"agent_name"`
	AgentKey     string    `json:"agent_key"` // Unique key: name@workdir or just name
	WorkDir      string    `json:"workdir"`
	Status       string    `json:"status"` // "active", "idle", "disconnected", "closed"
	TurnCount    int       `json:"turn_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	ModelID      string    `json:"model_id,omitempty"`
	ModeID       string    `json:"mode_id,omitempty"`
	Title        string    `json:"title,omitempty"`
	Summary      string    `json:"summary,omitempty"`
}

// ACPSessionMessage represents a single turn (prompt + response) in a session.
// Each turn is stored as a separate graph object linked to the session.
type ACPSessionMessage struct {
	// Identity
	MessageID  string `json:"message_id"`  // Unique ID for this message
	SessionID  string `json:"session_id"`  // Parent session
	TurnNumber int    `json:"turn_number"` // 1-based turn index within session

	// Prompt (user input)
	Prompt string `json:"prompt"` // The text sent to the agent

	// Response (agent output)
	Response   string `json:"response"`              // Full text response from the agent
	StopReason string `json:"stop_reason,omitempty"` // "end_turn", "max_tokens", "cancelled"

	// Tool calls observed during this turn
	ToolCalls []ACPToolCall `json:"tool_calls,omitempty"`

	// Error if the turn failed
	Error string `json:"error,omitempty"`

	// Timing
	DurationMs int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// ACPToolCall represents a tool invocation observed during an agent turn.
// Captured from session/update notifications with sessionUpdate="tool_call".
type ACPToolCall struct {
	ToolCallID string `json:"tool_call_id,omitempty"`
	Title      string `json:"title"`            // Human-readable title (e.g., "Reading auth.go")
	Kind       string `json:"kind,omitempty"`   // Tool kind/category
	Status     string `json:"status,omitempty"` // "running", "completed", "failed"
}

// ACPSessionStore defines the interface for persisting ACP sessions and their
// full conversation history.
//
// Sessions track multi-turn conversations with ACP agents. Every prompt sent
// and response received is stored as an ACPSessionMessage, providing a complete
// audit trail. Session metadata tracks the overall state (active/idle/closed),
// while messages capture the turn-by-turn details including tool calls and timing.
type ACPSessionStore interface {
	// --- Session CRUD ---

	// CreateSession persists a new session.
	CreateSession(ctx context.Context, session *ACPSession) error

	// GetSession retrieves a session by its ACP session ID.
	GetSession(ctx context.Context, sessionID string) (*ACPSession, error)

	// ListSessions returns sessions, optionally filtered by agent name and/or status.
	// Pass empty strings to skip filtering.
	ListSessions(ctx context.Context, agentName string, status string) ([]*ACPSession, error)

	// ListAllSessions returns all sessions, optionally filtered by status.
	ListAllSessions(ctx context.Context, status string) ([]*ACPSession, error)

	// UpdateSession applies partial updates to a session.
	// Supported keys: status, turn_count, last_active_at, model_id, mode_id, title, summary.
	UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error

	// DeleteSession removes a session and all its messages permanently.
	DeleteSession(ctx context.Context, sessionID string) error

	// MarkDisconnected sets all non-closed sessions to "disconnected".
	// Called on startup when all agent subprocesses are gone.
	MarkDisconnected(ctx context.Context) error

	// --- Message (turn) history ---

	// AddMessage stores a completed turn (prompt + response) for a session.
	AddMessage(ctx context.Context, msg *ACPSessionMessage) error

	// GetMessages returns all messages for a session, ordered by turn number ascending.
	GetMessages(ctx context.Context, sessionID string) ([]*ACPSessionMessage, error)

	// GetRecentMessages returns the last N messages for a session.
	GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]*ACPSessionMessage, error)

	// GetMessage retrieves a single message by its ID.
	GetMessage(ctx context.Context, messageID string) (*ACPSessionMessage, error)

	// DeleteMessages removes all messages for a session.
	// Called by DeleteSession, but also available independently for cleanup.
	DeleteMessages(ctx context.Context, sessionID string) error
}
