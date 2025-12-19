// Package mcp implements a Model Context Protocol server for Synapse.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/johnswift/synapse/internal/storage"
	"github.com/johnswift/synapse/pkg/types"
)

// Server implements an MCP server over stdio using JSON-RPC 2.0.
type Server struct {
	store  *storage.JSONLStore
	reader *bufio.Reader
	writer io.Writer
}

// NewServer creates a new MCP server.
func NewServer(store *storage.JSONLStore) *Server {
	return &Server{
		store:  store,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

// JSON-RPC 2.0 structures
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP-specific structures
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	Tools struct{} `json:"tools"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Capabilities    serverCapabilities `json:"capabilities"`
}

type tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Run starts the MCP server main loop.
func (s *Server) Run() error {
	log.SetOutput(os.Stderr) // Log to stderr, not stdout
	log.Println("MCP server starting...")

	scanner := bufio.NewScanner(s.reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		log.Printf("Received: %s", line)

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		s.handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (s *Server) handleRequest(req *jsonRPCRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req *jsonRPCRequest) {
	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: serverInfo{
			Name:    "synapse-mcp-server",
			Version: "0.1.0",
		},
		Capabilities: serverCapabilities{},
	}

	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req *jsonRPCRequest) {
	tools := []tool{
		{
			Name:        "create_task",
			Description: "Create a new synapse task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Task title (required)",
					},
					"blocked_by": map[string]interface{}{
						"type":        "array",
						"description": "Array of task IDs this task is blocked by",
						"items": map[string]interface{}{
							"type": "number",
						},
					},
					"parent_id": map[string]interface{}{
						"type":        "number",
						"description": "Parent task ID",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Assignee role/name",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "update_task",
			Description: "Update an existing synapse task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID (required)",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status (open, in-progress, blocked, review, done)",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "New assignee",
					},
					"blocked_by": map[string]interface{}{
						"type":        "array",
						"description": "Updated list of blocking task IDs",
						"items": map[string]interface{}{
							"type": "number",
						},
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "get_task",
			Description: "Get a single task by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID (required)",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "list_tasks",
			Description: "List tasks with optional filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Filter by assignee",
					},
				},
			},
		},
		{
			Name:        "get_next_task",
			Description: "Get the highest priority ready task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "Filter by assignee role",
					},
				},
			},
		},
		{
			Name:        "complete_task",
			Description: "Mark a task as done",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID (required)",
					},
				},
				"required": []string{"id"},
			},
		},
	}

	s.sendResult(req.ID, toolsListResult{Tools: tools})
}

func (s *Server) handleToolsCall(req *jsonRPCRequest) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	var result toolCallResult
	var err error

	switch params.Name {
	case "create_task":
		result, err = s.createTask(params.Arguments)
	case "update_task":
		result, err = s.updateTask(params.Arguments)
	case "get_task":
		result, err = s.getTask(params.Arguments)
	case "list_tasks":
		result, err = s.listTasks(params.Arguments)
	case "get_next_task":
		result, err = s.getNextTask(params.Arguments)
	case "complete_task":
		result, err = s.completeTask(params.Arguments)
	default:
		s.sendError(req.ID, -32602, "Invalid params", fmt.Sprintf("unknown tool: %s", params.Name))
		return
	}

	if err != nil {
		result = toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: fmt.Sprintf("Error: %v", err),
			}},
			IsError: true,
		}
	}

	s.sendResult(req.ID, result)
}

func (s *Server) createTask(args map[string]interface{}) (toolCallResult, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return toolCallResult{}, fmt.Errorf("title is required")
	}

	syn, err := s.store.Create(title)
	if err != nil {
		return toolCallResult{}, err
	}

	// Set optional fields
	if blockedByRaw, ok := args["blocked_by"].([]interface{}); ok {
		blockedBy := make([]int, 0, len(blockedByRaw))
		for _, v := range blockedByRaw {
			if id, ok := v.(float64); ok {
				blockedBy = append(blockedBy, int(id))
			}
		}
		syn.BlockedBy = blockedBy
	}

	if parentID, ok := args["parent_id"].(float64); ok {
		syn.ParentID = int(parentID)
	}

	if assignee, ok := args["assignee"].(string); ok {
		syn.Assignee = assignee
	}

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after create: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) updateTask(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	if status, ok := args["status"].(string); ok {
		newStatus := types.Status(status)
		if !newStatus.IsValid() {
			return toolCallResult{}, fmt.Errorf("invalid status: %s", status)
		}
		syn.Status = newStatus
	}

	if assignee, ok := args["assignee"].(string); ok {
		syn.Assignee = assignee
	}

	if blockedByRaw, ok := args["blocked_by"].([]interface{}); ok {
		blockedBy := make([]int, 0, len(blockedByRaw))
		for _, v := range blockedByRaw {
			if bid, ok := v.(float64); ok {
				blockedBy = append(blockedBy, int(bid))
			}
		}
		syn.BlockedBy = blockedBy
	}

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after update: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) getTask(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) listTasks(args map[string]interface{}) (toolCallResult, error) {
	var tasks []*types.Synapse

	if status, ok := args["status"].(string); ok {
		tasks = s.store.ByStatus(types.Status(status))
	} else if assignee, ok := args["assignee"].(string); ok {
		tasks = s.store.ByAssignee(assignee)
	} else {
		tasks = s.store.All()
	}

	data, _ := json.MarshalIndent(tasks, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) getNextTask(args map[string]interface{}) (toolCallResult, error) {
	ready := s.store.Ready()

	if assignee, ok := args["assignee"].(string); ok {
		// Filter by assignee
		for _, task := range ready {
			if task.Assignee == assignee {
				data, _ := json.MarshalIndent(task, "", "  ")
				return toolCallResult{
					Content: []toolContent{{
						Type: "text",
						Text: string(data),
					}},
				}, nil
			}
		}
		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: "null",
			}},
		}, nil
	}

	if len(ready) > 0 {
		data, _ := json.MarshalIndent(ready[0], "", "  ")
		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: string(data),
			}},
		}, nil
	}

	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: "null",
		}},
	}, nil
}

func (s *Server) completeTask(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	syn.MarkDone()

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after complete: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	s.writeResponse(resp)
}

func (s *Server) sendError(id interface{}, code int, message string, data interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}

	s.writeResponse(resp)
}

func (s *Server) writeResponse(resp jsonRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}

	log.Printf("Sending: %s", data)

	if _, err := fmt.Fprintf(s.writer, "%s\n", data); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
