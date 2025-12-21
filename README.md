# Synapse

**The shared nervous system for Vibe Coders and their Agents.**

Synapse is a lightweight, local-first, Git-backed issue tracker designed to serve as persistent "long-term memory" for AI agents. It enables multiple AI agents to coordinate work through a shared task graph with dependency tracking.

## Features

- **Git-Friendly Storage**: JSONL format for clean diffs and easy merging
- **Dependency Tracking**: Block tasks on other tasks, automatic ready-state detection
- **Sub-Agent Support**: Assign tasks to roles (`@qa`, `@coder`, `@architect`)
- **MCP Integration**: JSON-RPC 2.0 server for Claude Code and other AI tools
- **DAG Visualization**: Web-based Mermaid.js task graph with auto-refresh
- **Pure Go**: No CGO dependencies, single binary deployment
- **SQLite Cache**: Optional high-performance cache layer using `modernc.org/sqlite`

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
synapse add "Design API endpoints"
synapse add "Implement handlers" --blocks 1
synapse add "Write tests" --blocks 2 --assignee @qa

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

| Command | Description |
|---------|-------------|
| `init` | Initialize `.synapse` directory in current project |
| `add <title>` | Create a new task with optional `--blocks N`, `--parent N`, `--assignee X` |
| `list` | List all tasks (filter with `--status`, output with `--json`) |
| `ready` | List tasks ready to work on (unblocked, open status) |
| `get <id>` | Get details of a specific task |
| `claim <id>` | Mark task as in-progress |
| `done <id>` | Mark task as done |
| `all-done` | Mark all tasks as done (cleanup/reset command) |
| `serve` | Start MCP server (JSON-RPC over stdio) |
| `view` | Start visualization server (`--port N`, default 8080) |

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

**Available Tools:**
- `create_task` - Create new tasks with dependencies
- `update_task` - Modify task status, assignee, or blockers
- `get_task` - Retrieve task details
- `list_tasks` - List tasks with optional filters
- `get_next_task` - Get highest priority ready task
- `complete_task` - Mark task as done

## Visualization

Start the web-based DAG viewer:

```bash
synapse view --port 8080
```

Open http://localhost:8080 to see your task graph with:
- Color-coded status (white=open, yellow=in-progress, gray=blocked, green=done)
- Solid arrows for blocking dependencies
- Dotted arrows for parent-child relationships
- Auto-refresh every 5 seconds

## Data Storage

Synapse stores data in `.synapse/memory.jsonl`:

```jsonl
{"id":1,"title":"Design API","status":"done","created_at":"...","updated_at":"..."}
{"id":2,"title":"Implement handlers","status":"open","blocked_by":[1],"created_at":"..."}
```

**Best Practices:**
- Commit `.synapse/memory.jsonl` to Git
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

## Sub-Agent Coordination

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

## Development

```bash
# Run tests
go test ./...

# Build binary
go build -o synapse ./cmd/synapse/

# Run examples
go run ./examples/cache_demo/
go run ./examples/viz_server/
```

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please read the existing code style and add tests for new features.
