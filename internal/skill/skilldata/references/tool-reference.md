# Synapse MCP Tool Reference

Complete reference for all Synapse MCP tools with parameters, types, and examples.

## Task Management

### create_task

Create a new synapse task.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | yes | Task title |
| `priority` | number | no | Higher = more important (default: 0) |
| `blocked_by` | number[] | no | Task IDs this is blocked by |
| `parent_id` | number | no | Parent task ID |
| `assignee` | string | no | Role/name (e.g., `@coder`) |
| `discovered_from` | number | no | Task ID that led to discovery |
| `labels` | string[] | no | Tags: `bug`, `feature`, `security`, etc. |

**Example:**
```json
{
  "title": "Implement JWT refresh",
  "priority": 5,
  "blocked_by": [3],
  "labels": ["feature", "auth"]
}
```

### update_task

Update an existing synapse task.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |
| `status` | string | no | `open`, `in-progress`, `blocked`, `review`, `done` |
| `priority` | number | no | New priority |
| `assignee` | string | no | New assignee |
| `blocked_by` | number[] | no | Updated blocker list |
| `labels` | string[] | no | Updated labels |

### get_task

Get full task details by ID.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |

Returns all fields: title, status, priority, notes, labels, timestamps, claims.

### list_tasks

List tasks with optional filters and pagination. Returns summary by default.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `status` | string | no | | Filter by status |
| `assignee` | string | no | | Filter by assignee |
| `label` | string | no | | Filter by label |
| `limit` | number | no | 20 | Max tasks to return |
| `offset` | number | no | 0 | Skip N tasks (pagination) |
| `summary` | boolean | no | true | Summary mode (id, title, status, priority) |
| `fields` | string[] | no | | Specific fields to include |
| `max_chars` | number | no | 50000 | Max response size before auto-truncation |

**Pagination example:**
```json
{"status": "open", "limit": 10, "offset": 0}
{"status": "open", "limit": 10, "offset": 10}
```

### get_next_task

Get the highest priority unblocked task.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `assignee` | string | no | Filter by assignee role |

Returns the single highest-priority task with `status=open` and all blockers done.

### complete_task

Mark a task as done.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |

### delete_task

Delete task(s).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | no | Task ID (omit for bulk ops) |
| `delete_all` | boolean | no | Delete all tasks |
| `delete_completed` | boolean | no | Delete tasks with status `done` |

### spawn_task

Create a subtask discovered while working on another task. Auto-links provenance.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `parent_task_id` | number | yes | Task being worked on |
| `title` | string | yes | New task title |
| `blocked_by_parent` | boolean | no | Block on parent (default: false) |

### add_note

Append a note to a task for context persistence.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |
| `note` | string | yes | Note content |

## Breadcrumb Memory

### set_breadcrumb

Store a key-value breadcrumb for cross-session persistence.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Namespaced key (e.g., `auth.method`) |
| `value` | string | yes | Value to store |
| `task_id` | number | no | Link to originating task |

### get_breadcrumb

Retrieve a single breadcrumb by exact key.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Exact key |

### list_breadcrumbs

Query breadcrumbs with optional filters.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prefix` | string | no | Key prefix filter (e.g., `auth.`) |
| `task_id` | number | no | Filter by linked task |

### delete_breadcrumb

Remove a breadcrumb by key.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Exact key to delete |

## Multi-Agent Coordination

### claim_task

Claim a task with locking to prevent concurrent work.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |
| `agent_id` | string | yes | Your identifier (e.g., `claude-1`) |
| `timeout_minutes` | number | no | Claim expiry (default: 30) |

### release_claim

Release your lock on a task.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |

### complete_task_as

Mark a task as done with agent attribution.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | number | yes | Task ID |
| `agent_id` | string | yes | Your identifier |

### my_tasks

Get all tasks claimed by a specific agent.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `agent_id` | string | yes | Your identifier |

### get_context_window

Get tasks modified within a time window for session context recovery.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `minutes` | number | no | Look back N minutes (default: 60) |
| `agent_id` | string | no | Filter by agent |
