package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// EmergentACPSessionStore implements ACPSessionStore against the Emergent graph API.
//
// Mapping:
//
//	ACPSession:
//	  - Graph object type: "acp_session"
//	  - SessionID     -> properties.session_id + label "session_id:{id}"
//	  - AgentName     -> properties.agent_name + label "agent:{name}"
//	  - AgentKey      -> properties.agent_key
//	  - WorkDir       -> properties.workdir
//	  - Status        -> properties.status + label "status:{status}"
//	  - TurnCount     -> properties.turn_count
//	  - CreatedAt     -> properties.created_at (RFC3339Nano)
//	  - LastActiveAt  -> properties.last_active_at (RFC3339Nano)
//	  - ModelID       -> properties.model_id
//	  - ModeID        -> properties.mode_id
//	  - Title         -> properties.title
//	  - Summary       -> properties.summary
//
//	ACPSessionMessage:
//	  - Graph object type: "acp_session_message"
//	  - MessageID     -> properties.message_id + label "message_id:{id}"
//	  - SessionID     -> properties.session_id + label "session_id:{id}"
//	  - TurnNumber    -> properties.turn_number
//	  - Prompt        -> properties.prompt
//	  - Response      -> properties.response
//	  - StopReason    -> properties.stop_reason
//	  - ToolCalls     -> properties.tool_calls (JSON array)
//	  - Error         -> properties.error
//	  - DurationMs    -> properties.duration_ms
//	  - CreatedAt     -> properties.created_at (RFC3339Nano)
type EmergentACPSessionStore struct {
	client *sdk.Client
}

const (
	acpSessionType        = "acp_session"
	acpSessionMessageType = "acp_session_message"
)

// NewEmergentACPSessionStore creates a new Emergent-backed ACPSessionStore.
func NewEmergentACPSessionStore(client *sdk.Client) *EmergentACPSessionStore {
	return &EmergentACPSessionStore{client: client}
}

// ---------------------------------------------------------------------------
// Label helpers
// ---------------------------------------------------------------------------

func acpSessionIDLabel(sessionID string) string  { return fmt.Sprintf("session_id:%s", sessionID) }
func acpAgentNameLabel(agentName string) string  { return fmt.Sprintf("agent:%s", agentName) }
func acpSessionStatusLabel(status string) string { return fmt.Sprintf("status:%s", status) }

// acpSessionLabels builds the full label set for a session.
func acpSessionLabels(s *ACPSession) []string {
	return []string{
		acpSessionIDLabel(s.SessionID),
		acpAgentNameLabel(s.AgentName),
		acpSessionStatusLabel(s.Status),
	}
}

// Message label helpers.
// Note: message objects use type "acp_session_message" so session_id labels
// don't collide with session objects (type "acp_session").
func acpMessageIDLabel(messageID string) string { return fmt.Sprintf("message_id:%s", messageID) }

// acpMessageLabels builds the label set for a session message.
func acpMessageLabels(msg *ACPSessionMessage) []string {
	return []string{
		acpMessageIDLabel(msg.MessageID),
		acpSessionIDLabel(msg.SessionID),
	}
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

// acpSessionToProperties converts an ACPSession to Emergent properties.
func acpSessionToProperties(s *ACPSession) map[string]any {
	props := map[string]any{
		"session_id":     s.SessionID,
		"agent_name":     s.AgentName,
		"agent_key":      s.AgentKey,
		"workdir":        s.WorkDir,
		"status":         s.Status,
		"turn_count":     s.TurnCount,
		"created_at":     s.CreatedAt.UTC().Format(time.RFC3339Nano),
		"last_active_at": s.LastActiveAt.UTC().Format(time.RFC3339Nano),
	}
	if s.ModelID != "" {
		props["model_id"] = s.ModelID
	}
	if s.ModeID != "" {
		props["mode_id"] = s.ModeID
	}
	if s.Title != "" {
		props["title"] = s.Title
	}
	if s.Summary != "" {
		props["summary"] = s.Summary
	}
	return props
}

// acpSessionFromObject converts an Emergent GraphObject to an ACPSession.
func acpSessionFromObject(obj *graph.GraphObject) (*ACPSession, error) {
	s := &ACPSession{}

	if v, ok := obj.Properties["session_id"].(string); ok {
		s.SessionID = v
	}
	if v, ok := obj.Properties["agent_name"].(string); ok {
		s.AgentName = v
	}
	if v, ok := obj.Properties["agent_key"].(string); ok {
		s.AgentKey = v
	}
	if v, ok := obj.Properties["workdir"].(string); ok {
		s.WorkDir = v
	}
	if v, ok := obj.Properties["status"].(string); ok {
		s.Status = v
	}

	// turn_count can come as float64 or json.Number from JSON
	if v, ok := obj.Properties["turn_count"]; ok {
		switch n := v.(type) {
		case float64:
			s.TurnCount = int(n)
		case json.Number:
			i, _ := n.Int64()
			s.TurnCount = int(i)
		case int:
			s.TurnCount = n
		}
	}

	if v, ok := obj.Properties["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.CreatedAt = t
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = obj.CreatedAt
	}

	if v, ok := obj.Properties["last_active_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			s.LastActiveAt = t
		}
	}
	if s.LastActiveAt.IsZero() {
		s.LastActiveAt = s.CreatedAt
	}

	if v, ok := obj.Properties["model_id"].(string); ok {
		s.ModelID = v
	}
	if v, ok := obj.Properties["mode_id"].(string); ok {
		s.ModeID = v
	}
	if v, ok := obj.Properties["title"].(string); ok {
		s.Title = v
	}
	if v, ok := obj.Properties["summary"].(string); ok {
		s.Summary = v
	}

	return s, nil
}

// acpSessionMessageToProperties converts an ACPSessionMessage to Emergent properties.
func acpSessionMessageToProperties(msg *ACPSessionMessage) map[string]any {
	props := map[string]any{
		"message_id":  msg.MessageID,
		"session_id":  msg.SessionID,
		"turn_number": msg.TurnNumber,
		"prompt":      msg.Prompt,
		"response":    msg.Response,
		"duration_ms": msg.DurationMs,
		"created_at":  msg.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if msg.StopReason != "" {
		props["stop_reason"] = msg.StopReason
	}
	if msg.Error != "" {
		props["error"] = msg.Error
	}
	if len(msg.ToolCalls) > 0 {
		// Store tool calls as a JSON string to preserve structure.
		tc, err := json.Marshal(msg.ToolCalls)
		if err == nil {
			props["tool_calls"] = string(tc)
		}
	}
	return props
}

// acpSessionMessageFromObject converts an Emergent GraphObject to an ACPSessionMessage.
func acpSessionMessageFromObject(obj *graph.GraphObject) (*ACPSessionMessage, error) {
	msg := &ACPSessionMessage{}

	if v, ok := obj.Properties["message_id"].(string); ok {
		msg.MessageID = v
	}
	if v, ok := obj.Properties["session_id"].(string); ok {
		msg.SessionID = v
	}
	if v, ok := obj.Properties["prompt"].(string); ok {
		msg.Prompt = v
	}
	if v, ok := obj.Properties["response"].(string); ok {
		msg.Response = v
	}
	if v, ok := obj.Properties["stop_reason"].(string); ok {
		msg.StopReason = v
	}
	if v, ok := obj.Properties["error"].(string); ok {
		msg.Error = v
	}

	// turn_number
	if v, ok := obj.Properties["turn_number"]; ok {
		switch n := v.(type) {
		case float64:
			msg.TurnNumber = int(n)
		case json.Number:
			i, _ := n.Int64()
			msg.TurnNumber = int(i)
		case int:
			msg.TurnNumber = n
		}
	}

	// duration_ms
	if v, ok := obj.Properties["duration_ms"]; ok {
		switch n := v.(type) {
		case float64:
			msg.DurationMs = int(n)
		case json.Number:
			i, _ := n.Int64()
			msg.DurationMs = int(i)
		case int:
			msg.DurationMs = n
		}
	}

	// created_at
	if v, ok := obj.Properties["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			msg.CreatedAt = t
		}
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = obj.CreatedAt
	}

	// tool_calls — stored as JSON string
	if v, ok := obj.Properties["tool_calls"].(string); ok && v != "" {
		var toolCalls []ACPToolCall
		if err := json.Unmarshal([]byte(v), &toolCalls); err == nil {
			msg.ToolCalls = toolCalls
		}
	}

	return msg, nil
}

// ---------------------------------------------------------------------------
// ACPSessionStore implementation
// ---------------------------------------------------------------------------

func (s *EmergentACPSessionStore) CreateSession(ctx context.Context, session *ACPSession) error {
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if session.LastActiveAt.IsZero() {
		session.LastActiveAt = session.CreatedAt
	}
	if session.Status == "" {
		session.Status = "active"
	}

	props := acpSessionToProperties(session)
	labels := acpSessionLabels(session)
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       acpSessionType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent create acp session: %w", err)
	}

	slog.Info("emergent: created acp session",
		"session_id", session.SessionID,
		"agent", session.AgentName,
		"object_id", obj.ID,
	)
	return nil
}

func (s *EmergentACPSessionStore) GetSession(ctx context.Context, sessionID string) (*ACPSession, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Label: acpSessionIDLabel(sessionID),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup acp session %q: %w", sessionID, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return acpSessionFromObject(resp.Items[0])
}

func (s *EmergentACPSessionStore) ListSessions(ctx context.Context, agentName string, status string) ([]*ACPSession, error) {
	opts := &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Limit: 1000,
	}

	// Use labels for filtering. The Graph API supports a single Label filter,
	// so we pick the most selective one. If both are provided, we filter
	// client-side for the second criterion.
	if agentName != "" {
		opts.Label = acpAgentNameLabel(agentName)
	} else if status != "" {
		opts.Label = acpSessionStatusLabel(status)
	}

	resp, err := s.client.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("emergent list acp sessions: %w", err)
	}

	sessions := make([]*ACPSession, 0, len(resp.Items))
	for _, obj := range resp.Items {
		sess, err := acpSessionFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed acp session", "object_id", obj.ID, "error", err)
			continue
		}

		// Client-side filter for the second criterion
		if agentName != "" && status != "" && sess.Status != status {
			continue
		}

		sessions = append(sessions, sess)
	}

	// Sort by last_active_at descending (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
	})

	return sessions, nil
}

func (s *EmergentACPSessionStore) ListAllSessions(ctx context.Context, status string) ([]*ACPSession, error) {
	return s.ListSessions(ctx, "", status)
}

func (s *EmergentACPSessionStore) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	// Find the object
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Label: acpSessionIDLabel(sessionID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup acp session for update: %w", err)
	}
	if len(resp.Items) == 0 {
		return fmt.Errorf("acp session not found: %s", sessionID)
	}

	obj := resp.Items[0]

	// Build updated properties — merge with existing
	props := make(map[string]any)
	for k, v := range updates {
		props[k] = v
	}

	// Rebuild labels if status changed
	var newLabels []string
	rebuildLabels := false

	if newStatus, ok := updates["status"].(string); ok {
		rebuildLabels = true
		// Read current agent_name from properties for label rebuild
		agentName, _ := obj.Properties["agent_name"].(string)
		newLabels = []string{
			acpSessionIDLabel(sessionID),
			acpAgentNameLabel(agentName),
			acpSessionStatusLabel(newStatus),
		}
	}

	updateReq := &graph.UpdateObjectRequest{
		Properties: props,
	}
	if rebuildLabels {
		updateReq.Labels = newLabels
		updateReq.ReplaceLabels = true
	}

	_, err = s.client.Graph.UpdateObject(ctx, obj.ID, updateReq)
	if err != nil {
		return fmt.Errorf("emergent update acp session: %w", err)
	}

	slog.Info("emergent: updated acp session", "session_id", sessionID)
	return nil
}

func (s *EmergentACPSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	// Cascade: delete all messages for this session first.
	if err := s.DeleteMessages(ctx, sessionID); err != nil {
		return fmt.Errorf("emergent delete session messages during cascade: %w", err)
	}

	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Label: acpSessionIDLabel(sessionID),
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("emergent lookup acp session for delete: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil // Already deleted
	}

	err = s.client.Graph.DeleteObject(ctx, resp.Items[0].ID)
	if err != nil {
		return fmt.Errorf("emergent delete acp session: %w", err)
	}

	slog.Info("emergent: deleted acp session", "session_id", sessionID, "object_id", resp.Items[0].ID)
	return nil
}

func (s *EmergentACPSessionStore) MarkDisconnected(ctx context.Context) error {
	// Find all sessions that are "active" or "idle"
	activeResp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Label: acpSessionStatusLabel("active"),
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("emergent list active sessions: %w", err)
	}

	idleResp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionType,
		Label: acpSessionStatusLabel("idle"),
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("emergent list idle sessions: %w", err)
	}

	// Merge both lists
	allItems := append(activeResp.Items, idleResp.Items...)

	count := 0
	for _, obj := range allItems {
		sessionID, _ := obj.Properties["session_id"].(string)
		agentName, _ := obj.Properties["agent_name"].(string)

		newLabels := []string{
			acpSessionIDLabel(sessionID),
			acpAgentNameLabel(agentName),
			acpSessionStatusLabel("disconnected"),
		}

		_, err := s.client.Graph.UpdateObject(ctx, obj.ID, &graph.UpdateObjectRequest{
			Properties: map[string]any{
				"status": "disconnected",
			},
			Labels:        newLabels,
			ReplaceLabels: true,
		})
		if err != nil {
			slog.Warn("failed to mark session disconnected",
				"session_id", sessionID,
				"object_id", obj.ID,
				"error", err,
			)
			continue
		}
		count++
	}

	if count > 0 {
		slog.Info("emergent: marked sessions as disconnected", "count", count)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Message (turn) history
// ---------------------------------------------------------------------------

func (s *EmergentACPSessionStore) AddMessage(ctx context.Context, msg *ACPSessionMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	props := acpSessionMessageToProperties(msg)
	labels := acpMessageLabels(msg)
	status := "active"

	obj, err := s.client.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       acpSessionMessageType,
		Status:     &status,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return fmt.Errorf("emergent create acp session message: %w", err)
	}

	slog.Info("emergent: created acp session message",
		"message_id", msg.MessageID,
		"session_id", msg.SessionID,
		"turn", msg.TurnNumber,
		"object_id", obj.ID,
	)
	return nil
}

func (s *EmergentACPSessionStore) GetMessages(ctx context.Context, sessionID string) ([]*ACPSessionMessage, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionMessageType,
		Label: acpSessionIDLabel(sessionID),
		Limit: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent list acp session messages: %w", err)
	}

	messages := make([]*ACPSessionMessage, 0, len(resp.Items))
	for _, obj := range resp.Items {
		msg, err := acpSessionMessageFromObject(obj)
		if err != nil {
			slog.Warn("skipping malformed acp session message", "object_id", obj.ID, "error", err)
			continue
		}
		messages = append(messages, msg)
	}

	// Sort by turn_number ascending.
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].TurnNumber < messages[j].TurnNumber
	})

	return messages, nil
}

func (s *EmergentACPSessionStore) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]*ACPSessionMessage, error) {
	all, err := s.GetMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 || limit >= len(all) {
		return all, nil
	}

	// Return the last `limit` messages (already sorted by turn_number ascending).
	return all[len(all)-limit:], nil
}

func (s *EmergentACPSessionStore) GetMessage(ctx context.Context, messageID string) (*ACPSessionMessage, error) {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionMessageType,
		Label: acpMessageIDLabel(messageID),
		Limit: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("emergent lookup acp session message %q: %w", messageID, err)
	}
	if len(resp.Items) == 0 {
		return nil, nil
	}

	return acpSessionMessageFromObject(resp.Items[0])
}

func (s *EmergentACPSessionStore) DeleteMessages(ctx context.Context, sessionID string) error {
	resp, err := s.client.Graph.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  acpSessionMessageType,
		Label: acpSessionIDLabel(sessionID),
		Limit: 10000,
	})
	if err != nil {
		return fmt.Errorf("emergent list messages for delete: %w", err)
	}

	if len(resp.Items) == 0 {
		return nil
	}

	var deleteErr error
	count := 0
	for _, obj := range resp.Items {
		if err := s.client.Graph.DeleteObject(ctx, obj.ID); err != nil {
			slog.Warn("failed to delete acp session message",
				"object_id", obj.ID,
				"session_id", sessionID,
				"error", err,
			)
			deleteErr = err
			continue
		}
		count++
	}

	if count > 0 {
		slog.Info("emergent: deleted acp session messages",
			"session_id", sessionID,
			"count", count,
		)
	}

	if deleteErr != nil {
		return fmt.Errorf("emergent delete messages (partial): %w", deleteErr)
	}
	return nil
}
