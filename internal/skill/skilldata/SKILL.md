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
  version: "1.0.6"
  repository: "https://github.com/swiftj/synapse"
---

# Synapse Skill

Local-first, Git-backed task tracker and breadcrumb memory system.
Data: `.synapse/memory.jsonl` (Git-tracked). Prefer MCP tools when available; fall back to `syn` CLI otherwise (see `references/tool-reference.md`).

## Core Workflow

**Session Start**: `get_context_window` → `list_breadcrumbs` prefix `session.` → `get_next_task` → `claim_task`

**During Work**:
- `claim_task` before code, `complete_task` when done
- `spawn_task` immediately on discovering new work
- `add_note` for progress milestones
- `set_breadcrumb` for cross-session knowledge (e.g., `arch.auth = JWT`)

**Session End**: `complete_task_as` → breadcrumb `session.current_task`, `session.progress`, `session.next_step`

## Status Discipline

Synapse is the single source of truth. `list_tasks` + `list_breadcrumbs` must always reflect current state.

1. **Update immediately** — claim before starting, mark done instantly, set `blocked` on discovery
2. **Break plans into tasks upfront** — create all tasks *before* starting work
3. **Add progress notes** on long-running tasks via `add_note`
4. **Record discoveries immediately** via `spawn_task`
5. **Breadcrumb session state** at pause points for seamless session resumption

## Task Creation

- **Priority**: Higher = more important. 0 default, 10+ critical
- **Labels**: `bug`, `feature`, `security`, `refactor`, `docs`, `test`
- **Statuses**: `open` → `in-progress` → `blocked` | `review` → `done`
- **Dependencies**: `blocked_by` task IDs; blocked tasks hidden from `get_next_task`
- **Subtasks**: `parent_id` or `spawn_task` for auto-linked provenance

## Breadcrumb Patterns

Namespaced keys: `session.*` (context), `arch.*` (architecture), `env.*` (environment), `bug.*` (investigations), `decision.*` (rationale). Breadcrumbs persist across sessions.

## Multi-Agent Coordination

Each agent uses unique `agent_id`. `claim_task` before work (prevents double-work, expires 30min). `my_tasks` to check claims. `complete_task_as` for attribution.

## Reference

- CLI fallback commands: `references/tool-reference.md`
- Detailed workflow patterns: `references/workflows.md`
- Storage: `.synapse/memory.jsonl`, `.synapse/breadcrumbs.jsonl`, `.synapse/config.json`
- Errors: "task not found" → `list_tasks`; "already claimed" → `get_next_task`; "store not initialized" → `syn init`
