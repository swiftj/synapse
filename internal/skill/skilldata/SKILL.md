---
name: synapse
description: >-
  Persistent task tracking, breadcrumb memory, and multi-agent coordination
  for AI coding agents. Use when managing work across sessions, tracking bugs
  and issues, coordinating parallel agents, storing cross-session context,
  or when the user mentions tasks, issues, priorities, tracking, or memory.
license: MIT
compatibility: Requires synapse binary. Works on macOS and Linux.
metadata:
  author: swiftj
  version: "{{VERSION}}"
  repository: "https://github.com/swiftj/synapse"
---

# Synapse Skill

Synapse is a local-first, Git-backed task tracker and breadcrumb memory system.
Data lives in `.synapse/memory.jsonl` (Git-tracked) with an in-memory store.

## Tool Detection

Check if the Synapse MCP server is available by looking for these tools:
`create_task`, `list_tasks`, `get_next_task`, `set_breadcrumb`

If MCP tools are available, prefer them. Otherwise fall back to CLI commands
(see CLI Fallback section below).

## Quick Reference

### Task Management
| Tool | Purpose | Required Params |
|------|---------|-----------------|
| `create_task` | Create a new task | `title` |
| `update_task` | Modify task fields | `id` |
| `get_task` | Get full task details | `id` |
| `list_tasks` | Query with filters/pagination | (none) |
| `get_next_task` | Highest priority ready task | (none) |
| `complete_task` | Mark task done | `id` |
| `delete_task` | Delete task(s) | (none) |
| `spawn_task` | Create subtask from discovery | `parent_task_id`, `title` |
| `add_note` | Append note to task | `id`, `note` |

### Breadcrumb Memory
| Tool | Purpose | Required Params |
|------|---------|-----------------|
| `set_breadcrumb` | Store key-value pair | `key`, `value` |
| `get_breadcrumb` | Retrieve by exact key | `key` |
| `list_breadcrumbs` | Query with prefix filter | (none) |
| `delete_breadcrumb` | Remove by key | `key` |

### Multi-Agent Coordination
| Tool | Purpose | Required Params |
|------|---------|-----------------|
| `claim_task` | Lock task for yourself | `id`, `agent_id` |
| `release_claim` | Release your lock | `id` |
| `complete_task_as` | Complete with attribution | `id`, `agent_id` |
| `my_tasks` | Your claimed tasks | `agent_id` |
| `get_context_window` | Recent activity | (none) |

## Core Workflow

### Session Start
1. `get_context_window` — recent activity
2. `list_breadcrumbs` with prefix `session.` — prior session state
3. `get_next_task` — highest priority unblocked work
4. `claim_task` — lock it before starting

### During Work
- `claim_task` before touching code, `complete_task` the moment you finish
- `spawn_task` immediately when discovering new issues
- `add_note` to record progress milestones and decisions
- `set_breadcrumb` for cross-session knowledge (e.g., `arch.auth = JWT`)

### Session End
- `complete_task_as` for finished work
- `set_breadcrumb` for `session.current_task`, `session.progress`, `session.next_step`

## Status Discipline

**Synapse is the single source of truth for progress.** Any agent or human
should be able to query `list_tasks` and `list_breadcrumbs` at any time and
get an accurate picture of what's done, what's in flight, and what remains —
without scanning code.

**Rules:**
1. **Update immediately, not in batches.** Claim before starting, mark done the
   moment you finish, set `blocked` the instant you discover a blocker. Never
   defer status updates.
2. **Break plans into tasks upfront.** When given a multi-step plan, create a
   Synapse task for each step *before* starting work. This makes the full scope
   visible and queryable from the start.
3. **Add progress notes on long-running tasks.** Use `add_note` so the next
   session understands where things stand (e.g., "API endpoints done, starting
   tests") without re-reading code.
4. **Record discoveries immediately.** Use `spawn_task` the moment you find new
   work. Don't wait until the current task is done.
5. **Breadcrumb session state at pause points.** Set `session.current_task`,
   `session.progress`, and `session.next_step` so a new session can resume
   without re-analyzing the codebase.

## Task Creation Guidelines

**Priority**: Higher number = more important. Use 0 for default, 10+ for critical.

**Labels**: Use for categorization: `bug`, `feature`, `security`, `refactor`, `docs`, `test`.

**Assignee**: Role-based: `@qa`, `@coder`, `@architect`, `@reviewer`.

**Dependencies**: Set `blocked_by` to task IDs that must complete first.
A task with unfinished blockers won't appear in `get_next_task`.

**Subtasks**: Use `parent_id` for hierarchical organization, or `spawn_task`
to auto-link provenance when discovering issues during other work.

## Status Values

| Status | Meaning |
|--------|---------|
| `open` | Available for work |
| `in-progress` | Currently being worked on |
| `blocked` | Waiting on dependencies |
| `review` | Done, awaiting verification |
| `done` | Finished and verified |

## Breadcrumb Patterns

Use namespaced keys for organization:

- `session.*` — Session context (e.g., `session.goal`, `session.last_file`)
- `arch.*` — Architecture decisions (e.g., `arch.auth_method`)
- `env.*` — Environment details (e.g., `env.go_version`)
- `bug.*` — Bug investigation notes (e.g., `bug.root_cause`)
- `decision.*` — Design decisions with rationale

Breadcrumbs persist across sessions. Use `list_breadcrumbs` with a prefix
to retrieve related knowledge.

## Multi-Agent Coordination

When multiple agents work in parallel:

1. Each agent uses a unique `agent_id` (e.g., `claude-1`, `coder-qa`)
2. Call `claim_task` before starting — this prevents double-work
3. Claims expire after 30 minutes by default (configurable via `timeout_minutes`)
4. Use `my_tasks` to see what you've claimed
5. Call `complete_task_as` to record who finished what

## list_tasks Pagination

`list_tasks` returns summaries by default (id, title, status, priority).
For large projects, use pagination:

```json
{"status": "open", "limit": 10, "offset": 0}
```

Use `get_task` with a specific ID to get full details including notes,
labels, and timestamps.

## CLI Fallback

If MCP tools are not available, use CLI commands:

```bash
syn ready --json          # Get ready tasks
syn add "Title"           # Create task
syn claim 5               # Start working
syn done 5                # Complete task
syn list --status open    # Filter by status
syn bc set key value      # Set breadcrumb
syn bc get key            # Get breadcrumb
syn bc list prefix        # List breadcrumbs
```

## Error Recovery

- **"task not found"**: The task was deleted or the ID is wrong. Use `list_tasks`.
- **"already claimed"**: Another agent claimed it. Use `get_next_task` for alternatives.
- **"store not initialized"**: Run `syn init` first to create `.synapse/` directory.

## Data Storage

- **Source of truth**: `.synapse/memory.jsonl` (Git-tracked, one JSON per line)
- **Breadcrumbs**: `.synapse/breadcrumbs.jsonl` (Git-tracked, key-value pairs)
- **Config**: `.synapse/config.json` (agent roles, custom states)

For detailed workflow patterns, see `references/workflows.md`.
For complete tool documentation, see `references/tool-reference.md`.
