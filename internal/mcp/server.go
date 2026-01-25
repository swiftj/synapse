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
	ID      any             `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP-specific structures
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MaxResponseSize is the maximum allowed response size in bytes.
// MCP clients typically have token limits; 50KB is a safe threshold.
const MaxResponseSize = 50000

type serverCapabilities struct {
	Tools struct{} `json:"tools"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Capabilities    serverCapabilities `json:"capabilities"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "Task title (required)",
					},
					"priority": map[string]any{
						"type":        "number",
						"description": "Priority level (higher = more important, default 0)",
					},
					"blocked_by": map[string]any{
						"type":        "array",
						"description": "Array of task IDs this task is blocked by",
						"items": map[string]any{
							"type": "number",
						},
					},
					"parent_id": map[string]any{
						"type":        "number",
						"description": "Parent task ID",
					},
					"assignee": map[string]any{
						"type":        "string",
						"description": "Assignee role/name",
					},
					"discovered_from": map[string]any{
						"type":        "number",
						"description": "ID of the task from which this task was discovered (provenance tracking)",
					},
					"labels": map[string]any{
						"type":        "array",
						"description": "Labels/tags for categorization (e.g., bug, feature, security)",
						"items": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID (required)",
					},
					"status": map[string]any{
						"type":        "string",
						"description": "New status (open, in-progress, blocked, review, done)",
					},
					"priority": map[string]any{
						"type":        "number",
						"description": "Priority level (higher = more important)",
					},
					"assignee": map[string]any{
						"type":        "string",
						"description": "New assignee",
					},
					"blocked_by": map[string]any{
						"type":        "array",
						"description": "Updated list of blocking task IDs",
						"items": map[string]any{
							"type": "number",
						},
					},
					"labels": map[string]any{
						"type":        "array",
						"description": "Labels/tags for categorization (e.g., bug, feature, security)",
						"items": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID (required)",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "list_tasks",
			Description: "List tasks with optional filters and pagination. Returns summary by default to prevent response size issues. Use get_task(id) for full task details.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"status": map[string]any{
						"type":        "string",
						"description": "Filter by status",
					},
					"assignee": map[string]any{
						"type":        "string",
						"description": "Filter by assignee",
					},
					"label": map[string]any{
						"type":        "string",
						"description": "Filter by label",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "Maximum number of tasks to return (default: 20)",
					},
					"offset": map[string]any{
						"type":        "number",
						"description": "Number of tasks to skip for pagination",
					},
					"summary": map[string]any{
						"type":        "boolean",
						"description": "If true, return only id, title, status, priority (default: true). If false and response exceeds size limit, auto-falls back to summary with truncation notice.",
					},
					"fields": map[string]any{
						"type":        "array",
						"description": "Optional specific fields to include in the response. Recommended over summary:false for large datasets.",
						"items": map[string]any{
							"type": "string",
						},
					},
					"max_chars": map[string]any{
						"type":        "number",
						"description": "Maximum response size in characters (default: 50000). Responses exceeding this auto-truncate to summary mode.",
					},
				},
			},
		},
		{
			Name:        "get_next_task",
			Description: "Get the highest priority ready task",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"assignee": map[string]any{
						"type":        "string",
						"description": "Filter by assignee role",
					},
				},
			},
		},
		{
			Name:        "complete_task",
			Description: "Mark a task as done",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"parent_task_id": map[string]any{
						"type":        "number",
						"description": "ID of the task being worked on when this was discovered",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Title of the new discovered task",
					},
					"blocked_by_parent": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID (required)",
					},
					"note": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "Namespaced key (e.g., 'auth.method', 'db.connection')",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "Value to store",
					},
					"task_id": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prefix": map[string]any{
						"type":        "string",
						"description": "Filter by key prefix (e.g., 'auth.' returns all auth breadcrumbs)",
					},
					"task_id": map[string]any{
						"type":        "number",
						"description": "Filter by task ID",
					},
				},
			},
		},
		{
			Name:        "delete_breadcrumb",
			Description: "Remove a breadcrumb by key",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID to claim",
					},
					"agent_id": map[string]any{
						"type":        "string",
						"description": "Your agent identifier (e.g., 'claude-1', 'coder-agent')",
					},
					"timeout_minutes": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID to complete",
					},
					"agent_id": map[string]any{
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
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"minutes": map[string]any{
						"type":        "number",
						"description": "Look back N minutes (default: 60)",
					},
					"agent_id": map[string]any{
						"type":        "string",
						"description": "Filter by agent ID (optional)",
					},
				},
			},
		},
		{
			Name:        "my_tasks",
			Description: "Get all tasks claimed by a specific agent",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "Your agent identifier",
					},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			Name:        "delete_task",
			Description: "Delete a task by ID, delete all tasks, or delete all completed tasks",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "number",
						"description": "Task ID to delete (omit when using delete_all or delete_completed)",
					},
					"delete_all": map[string]any{
						"type":        "boolean",
						"description": "If true, delete all tasks (id is ignored)",
					},
					"delete_completed": map[string]any{
						"type":        "boolean",
						"description": "If true, delete all tasks with status 'done' (cleanup completed tasks)",
					},
				},
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
	case "delete_task":
		result, err = s.deleteTask(params.Arguments)
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

func (s *Server) createTask(args map[string]any) (toolCallResult, error) {
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

	if blockedByRaw, ok := args["blocked_by"].([]any); ok {
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

	if labelsRaw, ok := args["labels"].([]any); ok {
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

func (s *Server) updateTask(args map[string]any) (toolCallResult, error) {
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

	if blockedByRaw, ok := args["blocked_by"].([]any); ok {
		blockedBy := make([]int, 0, len(blockedByRaw))
		for _, v := range blockedByRaw {
			if bid, ok := v.(float64); ok {
				blockedBy = append(blockedBy, int(bid))
			}
		}
		syn.BlockedBy = blockedBy
	}

	if labelsRaw, ok := args["labels"].([]any); ok {
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

func (s *Server) getTask(args map[string]any) (toolCallResult, error) {
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

func (s *Server) listTasks(args map[string]any) (toolCallResult, error) {
	var tasks []*types.Synapse

	// Apply filters
	if label, ok := args["label"].(string); ok {
		tasks = s.store.ByLabel(label)
	} else if status, ok := args["status"].(string); ok {
		tasks = s.store.ByStatus(types.Status(status))
	} else if assignee, ok := args["assignee"].(string); ok {
		tasks = s.store.ByAssignee(assignee)
	} else {
		tasks = s.store.All()
	}

	totalCount := len(tasks)

	// Apply pagination
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	offset := 0
	if o, ok := args["offset"].(float64); ok && o >= 0 {
		offset = int(o)
	}

	// Response size limit (caller can override)
	maxChars := MaxResponseSize
	if mc, ok := args["max_chars"].(float64); ok && mc > 0 {
		maxChars = int(mc)
	}

	// Apply offset
	if offset >= len(tasks) {
		tasks = []*types.Synapse{}
	} else {
		tasks = tasks[offset:]
	}

	// Apply limit
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}

	// Check for summary mode (default true) and fields selection
	summary := true
	if s, ok := args["summary"].(bool); ok {
		summary = s
	}

	var fieldsSet map[string]bool
	if fieldsRaw, ok := args["fields"].([]any); ok && len(fieldsRaw) > 0 {
		fieldsSet = make(map[string]bool, len(fieldsRaw))
		for _, f := range fieldsRaw {
			if fieldName, ok := f.(string); ok {
				fieldsSet[fieldName] = true
			}
		}
		// If fields are explicitly specified, disable summary mode
		summary = false
	}

	// Build response data
	var resultTasks []map[string]any

	if fieldsSet != nil {
		// Return only specified fields
		resultTasks = make([]map[string]any, 0, len(tasks))
		for _, t := range tasks {
			taskMap := s.synapseToFieldMap(t, fieldsSet)
			resultTasks = append(resultTasks, taskMap)
		}
	} else if summary {
		// Summary mode: return only id, title, status, priority
		resultTasks = make([]map[string]any, 0, len(tasks))
		for _, t := range tasks {
			taskMap := map[string]any{
				"id":       t.ID,
				"title":    t.Title,
				"status":   t.Status,
				"priority": t.Priority,
			}
			resultTasks = append(resultTasks, taskMap)
		}
	}

	// Build final response with pagination metadata
	var data []byte
	var response map[string]any

	if resultTasks != nil {
		// Summary or field-selected mode
		response = map[string]any{
			"tasks":  resultTasks,
			"total":  totalCount,
			"limit":  limit,
			"offset": offset,
		}
		data, _ = json.Marshal(response)
	} else {
		// Full mode: return complete task objects
		response = map[string]any{
			"tasks":  tasks,
			"total":  totalCount,
			"limit":  limit,
			"offset": offset,
		}
		data, _ = json.Marshal(response)

		// Check if response exceeds size limit - auto-fallback to summary mode
		if len(data) > maxChars {
			log.Printf("Response size %d exceeds limit %d, falling back to summary mode", len(data), maxChars)

			// Rebuild as summary with truncated notes indicator
			summaryTasks := make([]map[string]any, 0, len(tasks))
			for _, t := range tasks {
				taskMap := map[string]any{
					"id":       t.ID,
					"title":    t.Title,
					"status":   t.Status,
					"priority": t.Priority,
				}
				// Include note count so caller knows there's more data
				if len(t.Notes) > 0 {
					taskMap["notes_count"] = len(t.Notes)
				}
				if t.Description != "" {
					// Truncate long descriptions
					desc := t.Description
					if len(desc) > 100 {
						desc = desc[:97] + "..."
					}
					taskMap["description"] = desc
				}
				summaryTasks = append(summaryTasks, taskMap)
			}

			response = map[string]any{
				"tasks":            summaryTasks,
				"total":            totalCount,
				"limit":            limit,
				"offset":           offset,
				"truncated":        true,
				"truncation_reason": "response_size_exceeded",
				"hint":             "Use get_task(id) to retrieve full task details, or use fields parameter to select specific fields",
			}
			data, _ = json.Marshal(response)
		}
	}

	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

// synapseToFieldMap converts a Synapse to a map with only the specified fields.
func (s *Server) synapseToFieldMap(t *types.Synapse, fields map[string]bool) map[string]any {
	result := make(map[string]any)

	if fields["id"] {
		result["id"] = t.ID
	}
	if fields["title"] {
		result["title"] = t.Title
	}
	if fields["description"] {
		result["description"] = t.Description
	}
	if fields["status"] {
		result["status"] = t.Status
	}
	if fields["priority"] {
		result["priority"] = t.Priority
	}
	if fields["blocked_by"] {
		result["blocked_by"] = t.BlockedBy
	}
	if fields["parent_id"] {
		result["parent_id"] = t.ParentID
	}
	if fields["assignee"] {
		result["assignee"] = t.Assignee
	}
	if fields["discovered_from"] {
		result["discovered_from"] = t.DiscoveredFrom
	}
	if fields["labels"] {
		result["labels"] = t.Labels
	}
	if fields["notes"] {
		result["notes"] = t.Notes
	}
	if fields["claimed_by"] {
		result["claimed_by"] = t.ClaimedBy
	}
	if fields["claimed_at"] {
		result["claimed_at"] = t.ClaimedAt
	}
	if fields["completed_by"] {
		result["completed_by"] = t.CompletedBy
	}
	if fields["created_at"] {
		result["created_at"] = t.CreatedAt
	}
	if fields["updated_at"] {
		result["updated_at"] = t.UpdatedAt
	}

	return result
}

func (s *Server) getNextTask(args map[string]any) (toolCallResult, error) {
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

func (s *Server) completeTask(args map[string]any) (toolCallResult, error) {
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

func (s *Server) spawnTask(args map[string]any) (toolCallResult, error) {
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

func (s *Server) addNote(args map[string]any) (toolCallResult, error) {
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

func (s *Server) setBreadcrumb(args map[string]any) (toolCallResult, error) {
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

	result := map[string]any{
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

func (s *Server) getBreadcrumb(args map[string]any) (toolCallResult, error) {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return toolCallResult{}, fmt.Errorf("key is required")
	}

	b, found := s.bcStore.Get(key)
	if !found {
		result := map[string]any{
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

	result := map[string]any{
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

func (s *Server) listBreadcrumbs(args map[string]any) (toolCallResult, error) {
	var breadcrumbs []*types.Breadcrumb

	if taskID, ok := args["task_id"].(float64); ok {
		breadcrumbs = s.bcStore.ListByTask(int(taskID))
	} else if prefix, ok := args["prefix"].(string); ok {
		breadcrumbs = s.bcStore.List(prefix)
	} else {
		breadcrumbs = s.bcStore.List("")
	}

	result := map[string]any{
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

func (s *Server) deleteBreadcrumb(args map[string]any) (toolCallResult, error) {
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

	result := map[string]any{
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

func (s *Server) claimTask(args map[string]any) (toolCallResult, error) {
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
		result := map[string]any{
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

func (s *Server) releaseClaim(args map[string]any) (toolCallResult, error) {
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

func (s *Server) completeTaskAs(args map[string]any) (toolCallResult, error) {
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

func (s *Server) getContextWindow(args map[string]any) (toolCallResult, error) {
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

	result := map[string]any{
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

func (s *Server) myTasks(args map[string]any) (toolCallResult, error) {
	agentID, ok := args["agent_id"].(string)
	if !ok || agentID == "" {
		return toolCallResult{}, fmt.Errorf("agent_id is required")
	}

	tasks := s.store.ClaimedBy(agentID)

	result := map[string]any{
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

func (s *Server) deleteTask(args map[string]any) (toolCallResult, error) {
	// Check if delete_all is specified
	if deleteAll, ok := args["delete_all"].(bool); ok && deleteAll {
		all := s.store.All()
		count := len(all)
		if count == 0 {
			return toolCallResult{
				Content: []toolContent{{
					Type: "text",
					Text: "No tasks to delete",
				}},
			}, nil
		}

		if err := s.store.DeleteAll(); err != nil {
			return toolCallResult{}, err
		}

		if err := s.store.Save(); err != nil {
			log.Printf("Warning: failed to save after delete all: %v", err)
		}

		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: fmt.Sprintf("Deleted all %d task(s)", count),
			}},
		}, nil
	}

	// Check if delete_completed is specified (cleanup done tasks)
	if deleteCompleted, ok := args["delete_completed"].(bool); ok && deleteCompleted {
		count, err := s.store.DeleteByStatus(types.StatusDone)
		if err != nil {
			return toolCallResult{}, err
		}

		if count == 0 {
			return toolCallResult{
				Content: []toolContent{{
					Type: "text",
					Text: "No completed tasks to delete",
				}},
			}, nil
		}

		if err := s.store.Save(); err != nil {
			log.Printf("Warning: failed to save after delete completed: %v", err)
		}

		return toolCallResult{
			Content: []toolContent{{
				Type: "text",
				Text: fmt.Sprintf("Deleted %d completed task(s)", count),
			}},
		}, nil
	}

	// Delete single task by ID
	id, ok := args["id"].(float64)
	if !ok {
		return toolCallResult{}, fmt.Errorf("id is required (or set delete_all or delete_completed to true)")
	}

	syn, err := s.store.Get(int(id))
	if err != nil {
		return toolCallResult{}, err
	}

	title := syn.Title
	if err := s.store.Delete(int(id)); err != nil {
		return toolCallResult{}, err
	}

	if err := s.store.Save(); err != nil {
		log.Printf("Warning: failed to save after delete: %v", err)
	}

	return toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: fmt.Sprintf("Deleted task #%d: %s", int(id), title),
		}},
	}, nil
}

func (s *Server) sendResult(id any, result any) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}

	s.writeResponse(resp)
}

func (s *Server) sendError(id any, code int, message string, data any) {
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
