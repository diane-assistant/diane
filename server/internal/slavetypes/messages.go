package slavetypes

import (
	"encoding/json"
	"time"
)

// Message types for WebSocket protocol
const (
	MessageTypeRegister       = "register"
	MessageTypeHeartbeat      = "heartbeat"
	MessageTypeToolUpdate     = "tool_update"
	MessageTypeToolCall       = "tool_call"
	MessageTypeResponse       = "response"
	MessageTypeError          = "error"
	MessageTypeRestart        = "restart"
	MessageTypeUpgrade        = "upgrade"
	MessageTypeMasterTools    = "master_tools"     // Master -> Slave: sends available master tools
	MessageTypeMasterToolCall = "master_tool_call" // Slave -> Master: requests execution of a master tool
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

// MasterToolsMessage is sent by master to slave with all available master tools
type MasterToolsMessage struct {
	// Servers maps server name -> list of tools for that server
	Servers map[string][]map[string]interface{} `json:"servers"`
	// ContextMappings maps context name -> list of enabled server names for that context.
	// This allows the slave to filter master-proxied tools by context, achieving parity
	// with the master's context filtering.
	ContextMappings map[string][]string `json:"context_mappings,omitempty"`
}

// MasterToolCallMessage is sent by slave to master to execute a tool on the master
type MasterToolCallMessage struct {
	Server    string                 `json:"server"`    // The MCP server name on the master
	Tool      string                 `json:"tool"`      // The tool name (without server prefix)
	Arguments map[string]interface{} `json:"arguments"` // Tool arguments
}
