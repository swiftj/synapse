// Package mcp implements a Model Context Protocol server for Synapse.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/swiftj/synapse/internal/storage"
	"github.com/swiftj/synapse/pkg/types"
)

// Server implements an MCP server over stdio using JSON-RPC 2.0.
type Server struct {
	store   *storage.JSONLStore
	bcStore *storage.BreadcrumbStore
	reader  *bufio.Reader
	writer  io.Writer
}

// NewServer creates a new MCP server.
func NewServer(store *storage.JSONLStore, bcStore *storage.BreadcrumbStore) *Server {
	return &Server{
		store:   store,
		bcStore: bcStore,
		reader:  bufio.NewReader(os.Stdin),
		writer:  os.Stdout,
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
					"priority": map[string]interface{}{
						"type":        "number",
						"description": "Priority level (higher = more important, default 0)",
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
					"discovered_from": map[string]interface{}{
						"type":        "number",
						"description": "ID of the task from which this task was discovered (provenance tracking)",
					},
					"labels": map[string]interface{}{
						"type":        "array",
						"description": "Labels/tags for categorization (e.g., bug, feature, security)",
						"items": map[string]interface{}{
							"type": "string",
						},
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
					"priority": map[string]interface{}{
						"type":        "number",
						"description": "Priority level (higher = more important)",
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
					"labels": map[string]interface{}{
						"type":        "array",
						"description": "Labels/tags for categorization (e.g., bug, feature, security)",
						"items": map[string]interface{}{
							"type": "string",
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
					"label": map[string]interface{}{
						"type":        "string",
						"description": "Filter by label",
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
		{
			Name:        "spawn_task",
			Description: "Create a subtask discovered while working on another task (auto-links provenance)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"parent_task_id": map[string]interface{}{
						"type":        "number",
						"description": "ID of the task being worked on when this was discovered",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Title of the new discovered task",
					},
					"blocked_by_parent": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this task should be blocked by the parent (default false)",
					},
				},
				"required": []string{"parent_task_id", "title"},
			},
		},
		{
			Name:        "add_note",
			Description: "Add a note to a task for context persistence",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID (required)",
					},
					"note": map[string]interface{}{
						"type":        "string",
						"description": "Note content to add",
					},
				},
				"required": []string{"id", "note"},
			},
		},
		{
			Name:        "set_breadcrumb",
			Description: "Store a key-value breadcrumb for cross-session persistence",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Namespaced key (e.g., 'auth.method', 'db.connection')",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Value to store",
					},
					"task_id": map[string]interface{}{
						"type":        "number",
						"description": "Optional: link to task that discovered this",
					},
				},
				"required": []string{"key", "value"},
			},
		},
		{
			Name:        "get_breadcrumb",
			Description: "Retrieve a single breadcrumb by exact key",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Exact key to retrieve",
					},
				},
				"required": []string{"key"},
			},
		},
		{
			Name:        "list_breadcrumbs",
			Description: "Query breadcrumbs with optional prefix filter",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prefix": map[string]interface{}{
						"type":        "string",
						"description": "Filter by key prefix (e.g., 'auth.' returns all auth breadcrumbs)",
					},
					"task_id": map[string]interface{}{
						"type":        "number",
						"description": "Filter by task ID",
					},
				},
			},
		},
		{
			Name:        "delete_breadcrumb",
			Description: "Remove a breadcrumb by key",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Exact key to delete",
					},
				},
				"required": []string{"key"},
			},
		},
		{
			Name:        "claim_task",
			Description: "Claim a task with locking (prevents other agents from claiming it)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID to claim",
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Your agent identifier (e.g., 'claude-1', 'coder-agent')",
					},
					"timeout_minutes": map[string]interface{}{
						"type":        "number",
						"description": "Claim timeout in minutes (default: 30)",
					},
				},
				"required": []string{"id", "agent_id"},
			},
		},
		{
			Name:        "release_claim",
			Description: "Release your claim on a task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID to release",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "complete_task_as",
			Description: "Mark a task as done with agent attribution",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "number",
						"description": "Task ID to complete",
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Your agent identifier",
					},
				},
				"required": []string{"id", "agent_id"},
			},
		},
		{
			Name:        "get_context_window",
			Description: "Get tasks modified within a time window (for session context)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"minutes": map[string]interface{}{
						"type":        "number",
						"description": "Look back N minutes (default: 60)",
					},
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by agent ID (optional)",
					},
				},
			},
		},
		{
			Name:        "my_tasks",
			Description: "Get all tasks claimed by a specific agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "Your agent identifier",
					},
				},
				"required": []string{"agent_id"},
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
	case "spawn_task":
		result, err = s.spawnTask(params.Arguments)
	case "add_note":
		result, err = s.addNote(params.Arguments)
	case "set_breadcrumb":
		result, err = s.setBreadcrumb(params.Arguments)
	case "get_breadcrumb":
		result, err = s.getBreadcrumb(params.Arguments)
	case "list_breadcrumbs":
		result, err = s.listBreadcrumbs(params.Arguments)
	case "delete_breadcrumb":
		result, err = s.deleteBreadcrumb(params.Arguments)
	case "claim_task":
		result, err = s.claimTask(params.Arguments)
	case "release_claim":
		result, err = s.releaseClaim(params.Arguments)
	case "complete_task_as":
		result, err = s.completeTaskAs(params.Arguments)
	case "get_context_window":
		result, err = s.getContextWindow(params.Arguments)
	case "my_tasks":
		result, err = s.myTasks(params.Arguments)
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
	if priority, ok := args["priority"].(float64); ok {
		syn.Priority = int(priority)
	}

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

	if discoveredFrom, ok := args["discovered_from"].(float64); ok {
		syn.DiscoveredFrom = fmt.Sprintf("#%d", int(discoveredFrom))
	}

	if labelsRaw, ok := args["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsRaw))
		for _, v := range labelsRaw {
			if label, ok := v.(string); ok {
				labels = append(labels, label)
			}
		}
		syn.Labels = labels
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

	if priority, ok := args["priority"].(float64); ok {
		syn.Priority = int(priority)
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

	if labelsRaw, ok := args["labels"].([]interface{}); ok {
		labels := make([]string, 0, len(labelsRaw))
		for _, v := range labelsRaw {
			if label, ok := v.(string); ok {
				labels = append(labels, label)
			}
		}
		syn.Labels = labels
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

	if label, ok := args["label"].(string); ok {
		tasks = s.store.ByLabel(label)
	} else if status, ok := args["status"].(string); ok {
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

func (s *Server) spawnTask(args map[string]interface{}) (toolCallResult, error) {
	parentID, ok := args["parent_task_id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("parent_task_id is required")
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		return toolCallResult{}, fmt.Errorf("title is required")
	}

	// Verify parent exists
	_, err := s.store.Get(int(parentID))
	if err != nil {
		return toolCallResult{}, fmt.Errorf("parent task not found: %w", err)
	}

	syn, err := s.store.Create(title)
	if err != nil {
		return toolCallResult{}, err
	}

	syn.DiscoveredFrom = fmt.Sprintf("#%d", int(parentID))
	syn.ParentID = int(parentID)

	if blockedByParent, ok := args["blocked_by_parent"].(bool); ok && blockedByParent {
		syn.BlockedBy = []int{int(parentID)}
		syn.Status = types.StatusBlocked
	}

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after spawn: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) addNote(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	note, ok := args["note"].(string)
	if !ok || note == "" {
		return toolCallResult{}, fmt.Errorf("note is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	syn.AddNote(note)

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after add_note: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) setBreadcrumb(args map[string]interface{}) (toolCallResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return toolCallResult{}, fmt.Errorf("key is required")
	}

	value, ok := args["value"].(string)
	if !ok {
		return toolCallResult{}, fmt.Errorf("value is required")
	}

	var taskID int
	if tid, ok := args["task_id"].(float64); ok {
		taskID = int(tid)
	}

	created, err := s.bcStore.Set(key, value, taskID)
	if err != nil {
		return toolCallResult{}, err
	}

	if err := s.bcStore.Save(); err != nil {
		log.Printf("Warning: failed to save breadcrumb: %v", err)
	}

	result := map[string]interface{}{
		"success": true,
		"key":     key,
		"created": created,
	}

	if b, found := s.bcStore.Get(key); found {
		result["updated_at"] = b.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) getBreadcrumb(args map[string]interface{}) (toolCallResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return toolCallResult{}, fmt.Errorf("key is required")
	}

	b, found := s.bcStore.Get(key)
	if !found {
		result := map[string]interface{}{
			"found": false,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: string(data),
			}},
		}, nil
	}

	result := map[string]interface{}{
		"found":      true,
		"breadcrumb": b,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) listBreadcrumbs(args map[string]interface{}) (toolCallResult, error) {
	var breadcrumbs []*types.Breadcrumb

	if taskID, ok := args["task_id"].(float64); ok {
		breadcrumbs = s.bcStore.ListByTask(int(taskID))
	} else if prefix, ok := args["prefix"].(string); ok {
		breadcrumbs = s.bcStore.List(prefix)
	} else {
		breadcrumbs = s.bcStore.List("")
	}

	result := map[string]interface{}{
		"breadcrumbs": breadcrumbs,
		"total":       len(breadcrumbs),
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) deleteBreadcrumb(args map[string]interface{}) (toolCallResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return toolCallResult{}, fmt.Errorf("key is required")
	}

	deleted := s.bcStore.Delete(key)
	if deleted {
		if err := s.bcStore.Save(); err != nil {
			log.Printf("Warning: failed to save after delete: %v", err)
		}
	}

	result := map[string]interface{}{
		"success": true,
		"deleted": deleted,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) claimTask(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return toolCallResult{}, fmt.Errorf("agent_id is required")
	}

	timeout := types.DefaultClaimTimeout
	if minutes, ok := args["timeout_minutes"].(float64); ok {
		timeout = time.Duration(minutes) * time.Minute
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	claimed := syn.Claim(agentID, timeout)
	if !claimed {
		result := map[string]interface{}{
			"success":       false,
			"claimed":       false,
			"claimed_by":    syn.ClaimedBy,
			"claimed_at":    syn.ClaimedAt,
			"error_message": "Task is already claimed by another agent",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: string(data),
			}},
		}, nil
	}

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after claim: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) releaseClaim(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	syn.ReleaseClaim()

	if err := s.store.Update(syn); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after release: %v", err)
	}

	data, _ := json.MarshalIndent(syn, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) completeTaskAs(args map[string]interface{}) (toolCallResult, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required")
	}

	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return toolCallResult{}, fmt.Errorf("agent_id is required")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	syn.MarkDoneBy(agentID)

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

func (s *Server) getContextWindow(args map[string]interface{}) (toolCallResult, error) {
	minutes := 60.0
	if m, ok := args["minutes"].(float64); ok {
		minutes = m
	}

	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	tasks := s.store.ModifiedSince(since)

	// Filter by agent if specified
	if agentID, ok := args["agent_id"].(string); ok && agentID != "" {
		var filtered []*types.Synapse
		for _, t := range tasks {
			if t.ClaimedBy == agentID || t.CompletedBy == agentID {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	result := map[string]interface{}{
		"tasks":        tasks,
		"total":        len(tasks),
		"since":        since.Format(time.RFC3339),
		"minutes_back": minutes,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func (s *Server) myTasks(args map[string]interface{}) (toolCallResult, error) {
	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return toolCallResult{}, fmt.Errorf("agent_id is required")
	}

	tasks := s.store.ClaimedBy(agentID)

	result := map[string]interface{}{
		"tasks":    tasks,
		"total":    len(tasks),
		"agent_id": agentID,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
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
