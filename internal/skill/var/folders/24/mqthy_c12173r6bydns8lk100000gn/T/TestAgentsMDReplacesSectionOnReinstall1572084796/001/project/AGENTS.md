<!-- BEGIN SYNAPSE SKILL v2.0.0 -->
# Synapse — Task Tracking & Memory for AI Agents

> Version: 2.0.0

Synapse provides persistent task tracking and breadcrumb memory across sessions.
Data lives in `.synapse/memory.jsonl` (Git-tracked).

## Available Commands

```bash
syn ready --json          # Get actionable tasks (unblocked, open)
syn add "Title"           # Create task (--blocks N, --parent N, --assignee X)
syn claim <id>            # Mark as in-progress
syn done <id>             # Mark as complete
syn list --json           # List all tasks
syn get <id> --json       # Full task details
syn delete <id>           # Delete task
syn bc set <key> <value>  # Store breadcrumb (cross-session memory)
syn bc get <key>          # Retrieve breadcrumb
syn bc list [prefix]      # List breadcrumbs
```

## Workflow

1. Run `syn ready --json` to find actionable tasks
2. `syn claim <id>` before starting work
3. Discover bugs/issues? `syn add "New issue" --parent <current_id>`
4. `syn done <id>` when finished
5. Store context: `syn bc set session.last_file src/auth.go`

## Status Values

`open` → `in-progress` → `review` → `done` | `blocked` (waiting on deps)

## Breadcrumb Namespaces

- `session.*` — Current session context
- `arch.*` — Architecture decisions
- `bug.*` — Bug investigation notes
- `decision.*` — Design decisions with rationale

If the Synapse MCP server is available, prefer MCP tools over CLI for richer
integration (pagination, multi-agent claims, breadcrumb filtering).

<!-- END SYNAPSE SKILL -->
