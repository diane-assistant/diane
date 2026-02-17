package slavetypes

import (
	"encoding/json"
	"time"
)

// Message types for WebSocket protocol
const (
	MessageTypeRegister   = "register"
	MessageTypeHeartbeat  = "heartbeat"
	MessageTypeToolUpdate = "tool_update"
	MessageTypeToolCall   = "tool_call"
	MessageTypeResponse   = "response"
	MessageTypeError      = "error"
	MessageTypeRestart    = "restart"
	MessageTypeUpgrade    = "upgrade"
)

// Message represents a WebSocket message
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"` // For request/response correlation
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// RegisterMessage is sent by slave on connection
type RegisterMessage struct {
	Hostname string                   `json:"hostname"`
	Version  string                   `json:"version"`
	Tools    []map[string]interface{} `json:"tools"`
}

// ToolCallMessage is sent by master to slave
type ToolCallMessage struct {
	Tool      string                 `json:"tool"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse is sent by slave back to master
type ToolCallResponse struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}
