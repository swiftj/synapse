# Synapse - Claude Code Project Instructions

## Project Overview

**Synapse** is a lightweight, local-first, Git-backed issue tracker serving as persistent "long-term memory" for AI agents. It solves the "dementia problem" (context loss between sessions) and the "lost work problem" (unlogged discoveries).

**Tagline:** *The shared nervous system for Vibe Coders and their Agents.*

## Key Differentiators

- **CGO-free architecture** - Pure Go, runs anywhere without compilation issues
- **Built-in MCP Server** - Native Claude Code integration, no external plugins
- **Local visualization engine** - DAG view for human "vibe coders"
- **Sub-agent awareness** - Role-based task assignment for parallel workflows

## Technical Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go 1.23+ | Performance, static binaries |
| Database | `modernc.org/sqlite` | Pure Go SQLite, no CGO dependency |
| Protocol | MCP via JSON-RPC | Native Claude Code integration |
| Storage | JSONL in `.synapse/memory.jsonl` | Git-friendly, merge-safe |
| Visualization | Mermaid.js (embedded) | Live DAG rendering |

## Architecture

```
project_root/
├── .synapse/
│   ├── memory.jsonl      # Source of Truth (Git tracked)
│   ├── index.db          # SQLite Cache (Git ignored)
│   └── config.json       # Agent roles and custom states
├── cmd/
│   └── synapse/          # CLI entry point
├── internal/
│   ├── storage/          # JSONL + SQLite operations
│   ├── engine/           # Next-action topological sort
│   ├── mcp/              # MCP server implementation
│   └── view/             # Visualization server
├── pkg/
│   └── types/            # Shared types (Synapse struct)
└── CLAUDE.md
```

## Core Data Model

```go
type Synapse struct {
    ID             int      `json:"id"`
    Title          string   `json:"title"`
    Description    string   `json:"description,omitempty"`
    Status         string   `json:"status"` // open, in-progress, blocked, review, done
    BlockedBy      []int    `json:"blocked_by,omitempty"`
    ParentID       int      `json:"parent_id,omitempty"`
    Assignee       string   `json:"assignee,omitempty"` // @qa, @architect, @coder
    DiscoveredFrom string   `json:"discovered_from,omitempty"`
    CreatedAt      string   `json:"created_at"`
    UpdatedAt      string   `json:"updated_at"`
}
```

## Development Guidelines

### Code Style
- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `internal/` for non-exported packages
- Use `pkg/` only for types shared across commands
- Prefer composition over inheritance
- Error wrapping with `fmt.Errorf("context: %w", err)`

### Performance Requirements
- All operations must complete in <50ms
- SQLite cache must be rebuildable from JSONL
- Append-only writes preferred for JSONL

### Testing
- Unit tests for all business logic in `*_test.go`
- Integration tests for CLI commands
- Table-driven tests preferred
- Run tests: `go test ./...`

### Building
- Build: `go build -o synapse ./cmd/synapse`
- Static binary: `CGO_ENABLED=0 go build -ldflags="-s -w" -o synapse ./cmd/synapse`

## CLI Commands

| Command | Description |
|---------|-------------|
| `syn init` | Initialize .synapse directory |
| `syn add "Title" [--blocks N] [--parent N]` | Create new synapse |
| `syn ready [--json]` | List unblocked, actionable tasks |
| `syn claim N` | Mark task as in-progress |
| `syn done N` | Mark task as complete |
| `syn view` | Start visualization server on :8080 |
| `syn serve` | Start MCP server on stdio |

## MCP Tools to Implement

1. **create_task** - Create new synapse with title, optional blockers/parent
2. **update_task** - Modify status, assignee, blockers
3. **get_next_task** - Return highest priority unblocked task
4. **list_tasks** - Query tasks by status/assignee
5. **read_memory** - Get full project state as context

## Critical Implementation Notes

### Ready Filter Logic
A task is `Ready` when:
- `Status == "open"`
- All tasks in `BlockedBy` have `Status == "done"`

### Topological Sort
The "Next-Action" engine must:
1. Build dependency graph from `BlockedBy` relationships
2. Return only unblocked tasks
3. Sort by priority (implied by ID or explicit field)
4. Output strictly typed JSON for agents, rich text for humans

### Git Merge Safety
- JSONL format: one JSON object per line
- Deterministic sorting by ID for consistent diffs
- Include custom merge driver in `.gitattributes`

## Dependencies to Add

```
go get modernc.org/sqlite
```

Do NOT use:
- `mattn/go-sqlite3` (requires CGO)
- Any C-binding SQLite drivers

## Workflow for AI Agents

When working on this codebase:
1. Run `syn ready --json` before starting work
2. Log new issues with `syn add` for any bugs discovered
3. Update task status with `syn claim` and `syn done`
4. Keep the visualization running for human oversight

## Status Values

| Status | Meaning |
|--------|---------|
| `open` | Not started, available for work |
| `in-progress` | Currently being worked on |
| `blocked` | Waiting on other tasks |
| `review` | Complete, awaiting verification |
| `done` | Finished and verified |

## File Patterns

- `.synapse/index.db` - Add to `.gitignore`
- `.synapse/memory.jsonl` - Track in Git
- `.synapse/config.json` - Track in Git
