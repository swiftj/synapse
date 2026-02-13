# Synapse

**The shared nervous system for Vibe Coders and their Agents.**

<!-- Badges -->
<p align="center">
  <img src="https://img.shields.io/badge/VERSION-1.0.4-blue?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/-macOS-000000?style=flat-square&logo=apple&logoColor=white" alt="macOS">
  <img src="https://img.shields.io/badge/-Linux-FCC624?style=flat-square&logo=linux&logoColor=black" alt="Linux">
  <img src="https://img.shields.io/badge/LICENSE-MIT-green?style=flat-square" alt="License">
</p>
<p align="center">
  <img src="https://img.shields.io/badge/CGO-FREE-success?style=flat-square" alt="CGO Free">
  <img src="https://img.shields.io/badge/MCP-COMPATIBLE-8A2BE2?style=flat-square" alt="MCP Compatible">
  <img src="https://img.shields.io/badge/Claude_Code-INTEGRATED-orange?style=flat-square" alt="Claude Code">
</p>

---

Synapse is a lightweight, local-first, Git-backed issue tracker designed to serve as persistent "long-term memory" for AI agents. It enables multiple AI agents to coordinate work through a shared task graph with dependency tracking.

## Features

- **Git-Friendly Storage**: JSONL format for clean diffs and easy merging
- **Dependency Tracking**: Block tasks on other tasks, automatic ready-state detection
- **Sub-Agent Support**: Assign tasks to roles (`@qa`, `@coder`, `@architect`)
- **MCP Integration**: JSON-RPC 2.0 server for Claude Code and other AI tools
- **DAG Visualization**: Web-based Mermaid.js task graph with auto-refresh
- **Pure Go**: No CGO dependencies, single binary deployment
- **Breadcrumb System**: Key-value storage for cross-session context persistence
- **Claim Locking**: Multi-agent coordination with automatic timeout release
- **Agent Identity**: Track which agent claimed and completed each task
- **Rich Metadata**: Notes, labels, priority, and discovery tracking

## Installation

```bash
go install github.com/swiftj/synapse/cmd/synapse@latest
```

Or build from source:

```bash
git clone https://github.com/swiftj/synapse.git
cd synapse
go build -o synapse ./cmd/synapse/
```

## Quick Start

```bash
# Initialize in your project
synapse init

# Manually add tasks
synapse add "Design API endpoints" --priority 3
synapse add "Implement handlers" --blocks 1 --label backend
synapse add "Write tests" --blocks 2 --assignee @qa --label testing

# View ready tasks
synapse ready

# Claim and complete work
synapse claim 1
synapse done 1

# Check what's ready now
synapse ready  # Task 2 is now unblocked!

# Optionally instruct Claude Code to use Synapse (CLAUDE.md or AGENTS.md)
echo "Use 'synapse' for task tracking" >> CLAUDE.md
```

## Commands

### Task Management

| Command | Description |
|---------|-------------|
| `init` | Initialize `.synapse` directory in current project |
| `add <title>` | Create a new task with optional flags (see below) |
| `list` | List all tasks (filter with `--status`, `--label`, output with `--json`) |
| `ready` | List tasks ready to work on (unblocked, open status) |
| `get <id>` | Get details of a specific task |
| `claim <id>` | Mark task as in-progress |
| `done <id>` | Mark task as done |
| `all-done` | Mark all tasks as done (cleanup/reset command) |
| `serve` | Start MCP server (JSON-RPC over stdio) |
| `view` | Start visualization server (`--port N`, default 8080) |

**Add command flags:**
- `--blocks N` - Task is blocked by task N
- `--parent N` - Task is a subtask of task N
- `--assignee X` - Assign to role (e.g., `@qa`, `@coder`)
- `--priority N` - Set priority (higher = more important)
- `--label X` - Add a label (can be used multiple times)
- `--note "text"` - Add a note (can be used multiple times)
- `--discovered-from N` - Link to task where this was discovered

### Breadcrumb Commands

Breadcrumbs are key-value pairs for storing cross-session context:

| Command | Description |
|---------|-------------|
| `breadcrumb set <key> <value>` | Store a breadcrumb |
| `breadcrumb get <key>` | Retrieve a breadcrumb |
| `breadcrumb list [prefix]` | List breadcrumbs (optionally filter by prefix) |
| `breadcrumb delete <key>` | Delete a breadcrumb |
| `bc` | Alias for `breadcrumb` |

**Example usage:**
```bash
# Store context for later sessions
synapse bc set "auth.strategy" "JWT with refresh tokens"
synapse bc set "db.migration.pending" "add_user_roles"
synapse bc set "blocked.reason.3" "Waiting for API spec from team"

# Retrieve context
synapse bc get "auth.strategy"

# List all breadcrumbs
synapse bc list

# List breadcrumbs with prefix
synapse bc list "auth."
```

## Task Lifecycle

```
open -> in-progress -> done
  |          |
  v          v
blocked -> review
```

Tasks automatically transition from `blocked` to ready when all their blockers are marked `done`.

## MCP Server Integration

Synapse includes a Model Context Protocol server for AI agent integration:

```json
// Add to Claude Code MCP config
{
  "mcpServers": {
    "synapse": {
      "command": "synapse",
      "args": ["serve"]
    }
  }
}
```

**Task Management Tools:**
- `create_task` - Create new tasks with dependencies, priority, labels, notes
- `update_task` - Modify task status, assignee, blockers, or metadata
- `get_task` - Retrieve task details
- `list_tasks` - List tasks with optional filters
- `get_next_task` - Get highest priority ready task
- `complete_task` - Mark task as done

**Multi-Agent Coordination Tools:**
- `claim_task` - Claim a task with your agent ID (30-min timeout)
- `release_claim` - Release your claim on a task
- `complete_task_as` - Mark task done and record completing agent
- `my_tasks` - List all tasks claimed by your agent
- `get_context_window` - Get tasks modified within a time window

**Breadcrumb Tools:**
- `set_breadcrumb` - Store a key-value pair (optionally linked to a task)
- `get_breadcrumb` - Retrieve a breadcrumb by key
- `list_breadcrumbs` - List breadcrumbs with optional prefix filter
- `delete_breadcrumb` - Remove a breadcrumb

## Visualization

Start the web-based DAG viewer:

```bash
synapse view --port 8080
```

Open http://localhost:8080 to see your task graph with:
- Color-coded status (white=open, yellow=in-progress, gray=blocked, blue=review, green=done)
- Solid arrows for blocking dependencies
- Dotted arrows for parent-child relationships
- Priority indicators (`P3` = priority 3)
- Claimed-by indicators (`@agent-name`)
- Label badges (`[backend,api]`)
- Auto-refresh every 5 seconds

## Data Storage

Synapse stores data in `.synapse/`:

| File | Description | Git |
|------|-------------|-----|
| `memory.jsonl` | Task data (source of truth) | ✅ Track |
| `breadcrumbs.jsonl` | Key-value context storage | ✅ Track |
| `index.db` | SQLite cache (auto-rebuilt) | ❌ Ignore |

**Task format example:**
```jsonl
{"id":1,"title":"Design API","status":"done","priority":3,"labels":["backend"],"created_at":"..."}
{"id":2,"title":"Implement handlers","status":"in-progress","blocked_by":[1],"claimed_by":"agent-1","claimed_at":"..."}
```

**Breadcrumb format example:**
```jsonl
{"key":"auth.strategy","value":"JWT with refresh tokens","created_at":"...","updated_at":"..."}
{"key":"blocked.reason.3","value":"Waiting for API spec","task_id":3,"created_at":"..."}
```

**Best Practices:**
- Commit `.synapse/memory.jsonl` and `.synapse/breadcrumbs.jsonl` to Git
- Add `.synapse/index.db` to `.gitignore` (SQLite cache, auto-rebuilt)

## Architecture

```
cmd/synapse/          # CLI entry point
internal/
  storage/            # JSONL + SQLite persistence
  mcp/                # MCP JSON-RPC server
  view/               # Web visualization server
pkg/types/            # Core Synapse struct and status types
```

## Multi-Agent Coordination

### Role-Based Assignment

Assign tasks to roles for multi-agent workflows:

```bash
synapse add "Review security" --assignee @security
synapse add "Fix vulnerabilities" --blocks 3 --assignee @coder
synapse add "Deploy to staging" --blocks 4 --assignee @devops
```

Query by assignee:
```bash
synapse list --assignee @coder --json
```

### Claim Locking

Prevent multiple agents from working on the same task:

```bash
# Agent claims task (30-minute timeout)
synapse claim 5

# Task is now "in-progress" and locked
# Other agents will see it as unavailable

# When done, mark complete
synapse done 5
```

**Via MCP Tools:**
- `claim_task` - Claims with agent ID and timeout
- `release_claim` - Releases if stuck or reassigning
- `complete_task_as` - Records completing agent
- `my_tasks` - Shows all tasks claimed by an agent

Claims automatically expire after 30 minutes if not completed or renewed, preventing deadlocks from crashed agents.

### Context Window Queries

Get tasks modified within a time window for session context:

```bash
# MCP tool: get_context_window
# Parameters: minutes (e.g., 60 for last hour)
```

This is useful for agents resuming work to understand recent activity.

## Claude Code Integration

To get Claude Code to automatically use Synapse MCP to its fullest, add the following to your project's `CLAUDE.md` file:

<details>
<summary>📋 Click to expand recommended CLAUDE.md content</summary>

```markdown
# Task Management with Synapse

This project uses Synapse for persistent task tracking and agent coordination.

## Required Behavior

**Always use Synapse MCP tools for task management:**
- Before starting work: Check `get_next_task` or `list_tasks` to understand current priorities
- When discovering issues: Use `create_task` to log them immediately (prevents lost work)
- When starting a task: Use `claim_task` with your agent ID to prevent collisions
- When completing work: Use `complete_task_as` to record completion
- For cross-session context: Use breadcrumbs (`set_breadcrumb`, `get_breadcrumb`)

## Task Workflow

1. **Session Start:**
   - Call `list_tasks` with status "in-progress" to see active work
   - Call `get_context_window` with minutes=60 to see recent activity
   - Check `my_tasks` if resuming previous work

2. **During Work:**
   - Log discovered bugs/issues immediately with `create_task` (use `discovered_from` to link to current task)
   - Add observations with `update_task` to append notes
   - Use `set_breadcrumb` for important context that should persist across sessions

3. **Task Completion:**
   - Use `complete_task_as` to mark done and record your agent ID
   - Check `get_next_task` for the next priority item

## Task Creation Guidelines

When creating tasks, always include:
- Clear, actionable title
- `priority` (1-5, higher = more important)
- `labels` for categorization (e.g., "bug", "feature", "refactor")
- `blocked_by` if dependent on other tasks
- `assignee` if role-specific (@qa, @coder, @architect)

Example:
\`\`\`
create_task({
  title: "Fix authentication token expiry",
  priority: 3,
  labels: ["bug", "security"],
  blocked_by: [5],
  assignee: "@coder"
})
\`\`\`

## Breadcrumb Usage

Use breadcrumbs to persist important context:
- `arch.*` - Architectural decisions (e.g., "arch.auth" = "JWT with refresh tokens")
- `blocked.*` - Blocking reasons (e.g., "blocked.task.5" = "Waiting for API spec")
- `context.*` - Session context (e.g., "context.current_focus" = "refactoring auth module")
- `decision.*` - Key decisions made (e.g., "decision.database" = "PostgreSQL for ACID compliance")

## Multi-Agent Coordination

- Claims timeout after 30 minutes automatically
- Always claim before starting work on a task
- Release claims if switching to different work
- Check `my_tasks` to see what you currently own
```

</details>

## Development

```bash
# Install git hooks (auto-version bumping)
./scripts/install-hooks.sh

# Run tests
go test ./...

# Build binary
go build -o synapse ./cmd/synapse/

# Run examples
go run ./examples/cache_demo/
go run ./examples/viz_server/
```

### Version Management

This project uses Git hooks for automatic SemVer versioning:

| Commit Type | Action | Example |
|-------------|--------|---------|
| Normal commit with `.go` files | Bump patch | `0.3.2` → `0.3.3` |
| `[minor]` in commit message | Bump minor | `0.3.2` → `0.4.0` |
| `[major]` in commit message | Bump major | `0.3.2` → `1.0.0` |
| `[skip-version]` in commit message | No change | `0.3.2` → `0.3.2` |

The hooks automatically update:
- `cmd/synapse/main.go` (version constant)
- `README.md` (version badge)

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please read the existing code style and add tests for new features.
