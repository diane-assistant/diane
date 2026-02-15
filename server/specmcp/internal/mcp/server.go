package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Server implements the MCP protocol over stdio.
type Server struct {
	registry *Registry
	info     ServerInfo
	logger   *slog.Logger
}

// NewServer creates an MCP server with the given registry and server info.
func NewServer(registry *Registry, info ServerInfo, logger *slog.Logger) *Server {
	return &Server{
		registry: registry,
		info:     info,
		logger:   logger,
	}
}

// Run reads JSON-RPC requests from stdin and writes responses to stdout.
// It blocks until stdin is closed or the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	// MCP messages can be large (e.g. sync results)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	encoder := json.NewEncoder(os.Stdout)

	s.logger.Info("specmcp server started", "name", s.info.Name, "version", s.info.Version)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := s.handleMessage(ctx, line)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				s.logger.Error("failed to write response", "error", err)
				return fmt.Errorf("writing response: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("reading stdin: %w", err)
	}

	s.logger.Info("specmcp server stopped (stdin closed)")
	return nil
}

// handleMessage parses a JSON-RPC request and dispatches to the appropriate handler.
func (s *Server) handleMessage(ctx context.Context, data []byte) *Response {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.logger.Error("failed to parse request", "error", err)
		return &Response{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    ErrCodeParse,
				Message: "Parse error",
				Data:    err.Error(),
			},
		}
	}

	// Notifications (no ID) don't get a response
	if req.ID == nil && req.Method == "notifications/initialized" {
		s.logger.Info("client initialized")
		return nil
	}
	if req.ID == nil {
		s.logger.Debug("received notification", "method", req.Method)
		return nil
	}

	s.logger.Debug("handling request", "method", req.Method, "id", string(req.ID))

	result, rpcErr := s.dispatch(ctx, &req)
	resp := &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	return resp
}

// dispatch routes a request to the appropriate handler method.
func (s *Server) dispatch(ctx context.Context, req *Request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "tools/list":
		return s.handleToolsList()
	case "tools/call":
		return s.handleToolsCall(ctx, req.Params)
	case "prompts/list":
		return s.handlePromptsList()
	case "prompts/get":
		return s.handlePromptsGet(req.Params)
	case "resources/list":
		return s.handleResourcesList()
	case "resources/read":
		return s.handleResourcesRead(req.Params)
	default:
		return nil, &RPCError{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
}

// handleInitialize responds to the MCP handshake.
func (s *Server) handleInitialize(params json.RawMessage) (any, *RPCError) {
	var initParams InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, &RPCError{
				Code:    ErrCodeInvalidParams,
				Message: "Invalid initialize params",
				Data:    err.Error(),
			}
		}
	}

	s.logger.Info("client connecting",
		"client", initParams.ClientInfo.Name,
		"client_version", initParams.ClientInfo.Version,
		"protocol_version", initParams.ProtocolVersion,
	)

	caps := ServerCapability{
		Tools: &ToolsCapability{},
	}
	if s.registry.HasPrompts() {
		caps.Prompts = &PromptsCapability{}
	}
	if s.registry.HasResources() {
		caps.Resources = &ResourcesCapability{}
	}

	return &InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    caps,
		ServerInfo:      s.info,
	}, nil
}

// handleToolsList returns all registered tools.
func (s *Server) handleToolsList() (any, *RPCError) {
	return &ToolsListResult{
		Tools: s.registry.List(),
	}, nil
}

// handleToolsCall dispatches a tool call to the registry.
func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *RPCError) {
	var callParams ToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid tools/call params",
			Data:    err.Error(),
		}
	}

	tool := s.registry.Get(callParams.Name)
	if tool == nil {
		return nil, &RPCError{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("tool not found: %s", callParams.Name),
		}
	}

	s.logger.Info("calling tool", "tool", callParams.Name)

	result, err := tool.Execute(ctx, callParams.Arguments)
	if err != nil {
		s.logger.Error("tool execution failed", "tool", callParams.Name, "error", err)
		return ErrorResult(fmt.Sprintf("tool execution failed: %v", err)), nil
	}

	return result, nil
}

// handlePromptsList returns all registered prompts.
func (s *Server) handlePromptsList() (any, *RPCError) {
	return &PromptsListResult{
		Prompts: s.registry.ListPrompts(),
	}, nil
}

// handlePromptsGet returns a specific prompt by name.
func (s *Server) handlePromptsGet(params json.RawMessage) (any, *RPCError) {
	var getParams PromptsGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, &RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid prompts/get params",
			Data:    err.Error(),
		}
	}

	prompt := s.registry.GetPrompt(getParams.Name)
	if prompt == nil {
		return nil, &RPCError{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("prompt not found: %s", getParams.Name),
		}
	}

	s.logger.Debug("getting prompt", "prompt", getParams.Name)

	result, err := prompt.Get(getParams.Arguments)
	if err != nil {
		return nil, &RPCError{
			Code:    ErrCodeInternal,
			Message: fmt.Sprintf("prompt error: %v", err),
		}
	}

	return result, nil
}

// handleResourcesList returns all registered resources.
func (s *Server) handleResourcesList() (any, *RPCError) {
	return &ResourcesListResult{
		Resources: s.registry.ListResources(),
	}, nil
}

// handleResourcesRead returns the content of a specific resource.
func (s *Server) handleResourcesRead(params json.RawMessage) (any, *RPCError) {
	var readParams ResourcesReadParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return nil, &RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid resources/read params",
			Data:    err.Error(),
		}
	}

	resource := s.registry.GetResource(readParams.URI)
	if resource == nil {
		return nil, &RPCError{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("resource not found: %s", readParams.URI),
		}
	}

	s.logger.Debug("reading resource", "uri", readParams.URI)

	result, err := resource.Read()
	if err != nil {
		return nil, &RPCError{
			Code:    ErrCodeInternal,
			Message: fmt.Sprintf("resource read error: %v", err),
		}
	}

	return result, nil
}
