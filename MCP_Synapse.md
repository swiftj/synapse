# Synapse MCP Server

**Purpose**: Persistent task tracking and multi-agent coordination for AI workflows

## Triggers
- Task management and progress tracking needs
- Multi-step operations requiring persistent memory
- Multi-agent workflows needing coordination
- Cross-session context persistence requirements
- Discovered issues that need logging immediately
- Priority-based work queue management

## Choose When
- **For task persistence**: When work spans multiple sessions or agents
- **Over TodoWrite**: For permanent, Git-tracked task history with dependencies
- **For multi-agent work**: When multiple agents need to coordinate without collisions
- **For context preservation**: Breadcrumbs persist important decisions across sessions
- **For discovery logging**: Immediately capture bugs/issues found during other work
- **Not for ephemeral tasks**: Simple in-session checklists don't need Synapse

## Works Best With
- **Sequential**: Sequential analyzes complex problems → Synapse tracks resulting tasks
- **Serena**: Serena provides project memory → Synapse tracks actionable work items
- **Task agents**: Parent agent delegates → sub-agents claim and complete via Synapse

## Tools Reference

**Task Management:**
- `create_task` - Create tasks with priority, labels, dependencies
- `update_task` - Modify status, assignee, or metadata
- `get_task` / `list_tasks` - Query task state
- `get_next_task` - Get highest priority ready task
- `complete_task` - Mark task done
- `delete_task` - Delete a single task by ID or all tasks

**Multi-Agent Coordination:**
- `claim_task` - Lock task for your agent (30-min timeout)
- `release_claim` - Release lock if reassigning
- `complete_task_as` - Mark done with agent attribution
- `my_tasks` - List tasks claimed by your agent
- `get_context_window` - Tasks modified in time window

**Breadcrumbs (Cross-Session Context):**
- `set_breadcrumb` - Store key-value context
- `get_breadcrumb` - Retrieve stored context
- `list_breadcrumbs` - List with optional prefix filter
- `delete_breadcrumb` - Remove outdated context

## Examples
```
"track this bug for later" → Synapse create_task (persistent logging)
"what should I work on next" → Synapse get_next_task (priority queue)
"I'm starting on task 5" → Synapse claim_task (collision prevention)
"remember we chose JWT auth" → Synapse set_breadcrumb (context persistence)
"what did we decide about auth" → Synapse get_breadcrumb (context retrieval)
"show recent activity" → Synapse get_context_window (session context)
"simple checklist for this task" → TodoWrite (ephemeral, in-session only)
```
