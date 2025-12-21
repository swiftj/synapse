# Breadcrumb Storage Schema Design

**Status**: Draft
**Version**: 1.0
**Created**: 2025-12-21

## Overview

Breadcrumbs provide persistent, cross-session knowledge storage for AI agents working with Synapse. They solve the "context loss" problem by allowing agents to store and retrieve arbitrary key-value pairs that represent discoveries, configurations, and solutions found during task execution.

## Core Data Structure

### Go Type Definition

```go
type Breadcrumb struct {
    Key        string    `json:"key"`               // Namespaced key (e.g., "auth.method")
    Value      string    `json:"value"`             // The stored value
    TaskID     int       `json:"task_id,omitempty"` // Optional: task that created this
    CreatedAt  time.Time `json:"created_at"`        // Initial creation timestamp
    UpdatedAt  time.Time `json:"updated_at"`        // Last modification timestamp
}
```

### Field Specifications

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Key` | string | Yes | Unique identifier with namespace support (dot-separated) |
| `Value` | string | Yes | Arbitrary string value (agents can store JSON, URLs, etc.) |
| `TaskID` | int | No | References the Synapse task that discovered/created this breadcrumb |
| `CreatedAt` | time.Time | Yes | RFC3339 timestamp of initial creation |
| `UpdatedAt` | time.Time | Yes | RFC3339 timestamp of last update (initially equals CreatedAt) |

### Design Rationale

**String-based values**: While this limits type safety, it provides maximum flexibility. Agents can store:
- Simple strings: `"JWT"`
- JSON objects: `{"host": "localhost", "port": 5432}`
- URLs: `"https://docs.example.com/auth"`
- Multi-line content: error messages, stack traces, code snippets

**TaskID provenance**: Optional linking to tasks enables:
- Discovery tracking: "Which task found this solution?"
- Cleanup: Delete breadcrumbs when parent task is removed
- Audit trail: Understand the context of stored knowledge

**Separate timestamps**: Distinguishing creation from updates allows:
- Recency queries: "Show me breadcrumbs modified in the last 7 days"
- Staleness detection: "Flag breadcrumbs older than 30 days"
- Version awareness: Track when knowledge was last validated

## Storage Implementation

### File Location

```
.synapse/breadcrumbs.jsonl
```

**Separation rationale**: Keeping breadcrumbs in a separate file from `memory.jsonl` (tasks) provides:
- Clean domain separation
- Simpler parsing logic
- Independent Git history
- Reduced merge conflicts

### File Format (JSONL)

One JSON object per line, sorted by key for deterministic diffs:

```jsonl
{"key":"api.base_url","value":"https://api.example.com","task_id":42,"created_at":"2025-12-21T10:00:00Z","updated_at":"2025-12-21T10:00:00Z"}
{"key":"auth.method","value":"JWT","task_id":15,"created_at":"2025-12-20T14:30:00Z","updated_at":"2025-12-21T09:15:00Z"}
{"key":"db.connection","value":"{\"host\":\"localhost\",\"port\":5432}","task_id":8,"created_at":"2025-12-19T08:00:00Z","updated_at":"2025-12-19T08:00:00Z"}
```

### Git Considerations

- **Track in Git**: `.synapse/breadcrumbs.jsonl` should be committed
- **Merge safety**: Alphabetical key sorting ensures consistent diffs
- **Conflict resolution**: Custom merge driver could auto-resolve by timestamp (future enhancement)

## Key Namespacing Convention

### Standard Namespaces

| Prefix | Purpose | Examples |
|--------|---------|----------|
| `auth.*` | Authentication/authorization | `auth.method`, `auth.token_expiry`, `auth.provider` |
| `db.*` | Database configuration | `db.connection`, `db.migration_status`, `db.schema_version` |
| `api.*` | API endpoints and behavior | `api.base_url`, `api.rate_limit`, `api.version` |
| `config.*` | Application configuration | `config.environment`, `config.log_level`, `config.feature_flags` |
| `error.*` | Known errors and solutions | `error.cors_fix`, `error.timeout_workaround` |
| `build.*` | Build system discoveries | `build.flags`, `build.dependencies`, `build.target` |
| `test.*` | Testing-related knowledge | `test.fixtures_location`, `test.coverage_threshold` |
| `deploy.*` | Deployment information | `deploy.platform`, `deploy.credentials_path` |

### Naming Conventions

- **Use dot-separated namespaces**: `category.subcategory.name`
- **Lowercase keys**: `auth.method` not `Auth.Method`
- **Underscores for multi-word**: `auth.token_expiry` not `auth.tokenExpiry`
- **Descriptive names**: `db.primary_connection` not `db.conn1`

### Custom Namespaces

Projects can define custom namespaces in `.synapse/config.json`:

```json
{
  "breadcrumb_namespaces": {
    "ml": "Machine learning model configurations",
    "cache": "Caching strategies and configurations",
    "queue": "Message queue settings"
  }
}
```

## CLI Commands

### Set Breadcrumb

```bash
syn breadcrumb set <key> <value> [--task-id N]
```

**Behavior**:
- Creates new breadcrumb if key doesn't exist
- Updates existing breadcrumb (preserves `CreatedAt`, updates `UpdatedAt`)
- Optional `--task-id` links breadcrumb to specific task
- Returns confirmation with key and value

**Examples**:
```bash
# Simple set
syn breadcrumb set auth.method JWT

# Set from task context
syn breadcrumb set error.cors_fix "Added Access-Control-Allow-Origin header" --task-id 42

# Set complex value (JSON)
syn breadcrumb set db.connection '{"host":"localhost","port":5432,"database":"synapse_dev"}'
```

### Get Breadcrumb

```bash
syn breadcrumb get <key>
```

**Behavior**:
- Returns value for exact key match
- Exit code 0 if found, 1 if not found
- Outputs just the value (for shell scripting)

**Examples**:
```bash
# Get value
syn breadcrumb get auth.method
# Output: JWT

# Use in script
DB_CONFIG=$(syn breadcrumb get db.connection)
```

### List Breadcrumbs

```bash
syn breadcrumb list [prefix] [--format json|table]
```

**Behavior**:
- Lists all breadcrumbs if no prefix provided
- Filters by prefix if provided (e.g., `auth.` shows all auth breadcrumbs)
- Default format: human-readable table
- `--format json` for machine parsing

**Examples**:
```bash
# List all breadcrumbs
syn breadcrumb list

# List auth-related breadcrumbs
syn breadcrumb list auth.

# List in JSON format
syn breadcrumb list --format json

# List with prefix in JSON
syn breadcrumb list api. --format json
```

**Output formats**:

Table format:
```
KEY                 VALUE                           TASK    CREATED              UPDATED
auth.method         JWT                             15      2025-12-20 14:30     2025-12-21 09:15
auth.token_expiry   3600                            15      2025-12-20 14:30     2025-12-20 14:30
db.connection       {"host":"localhost",...}        8       2025-12-19 08:00     2025-12-19 08:00
```

JSON format:
```json
[
  {
    "key": "auth.method",
    "value": "JWT",
    "task_id": 15,
    "created_at": "2025-12-20T14:30:00Z",
    "updated_at": "2025-12-21T09:15:00Z"
  }
]
```

### Delete Breadcrumb

```bash
syn breadcrumb delete <key> [--confirm]
```

**Behavior**:
- Removes breadcrumb with exact key match
- Requires confirmation unless `--confirm` flag provided
- Returns error if key doesn't exist

**Examples**:
```bash
# Delete with confirmation prompt
syn breadcrumb delete auth.old_method

# Delete without prompt
syn breadcrumb delete error.temp_fix --confirm
```

## MCP Tools

### set_breadcrumb

**Purpose**: Store a key-value breadcrumb for cross-session persistence

**Parameters**:
```typescript
{
  key: string;        // Required: namespaced key
  value: string;      // Required: value to store
  task_id?: number;   // Optional: link to task
}
```

**Returns**:
```typescript
{
  success: boolean;
  key: string;
  created: boolean;   // true if new, false if updated
  updated_at: string; // RFC3339 timestamp
}
```

**Example**:
```json
{
  "key": "auth.method",
  "value": "JWT",
  "task_id": 15
}
```

### get_breadcrumb

**Purpose**: Retrieve a single breadcrumb by exact key

**Parameters**:
```typescript
{
  key: string;  // Required: exact key to retrieve
}
```

**Returns**:
```typescript
{
  found: boolean;
  breadcrumb?: {
    key: string;
    value: string;
    task_id?: number;
    created_at: string;
    updated_at: string;
  }
}
```

**Example**:
```json
{
  "key": "auth.method"
}
```

### list_breadcrumbs

**Purpose**: Query breadcrumbs with filtering options

**Parameters**:
```typescript
{
  prefix?: string;           // Optional: filter by key prefix
  limit?: number;            // Optional: max results (default: 100)
  sort?: "key" | "created" | "updated";  // Optional: sort order (default: "key")
  order?: "asc" | "desc";    // Optional: sort direction (default: "asc")
}
```

**Returns**:
```typescript
{
  breadcrumbs: Array<{
    key: string;
    value: string;
    task_id?: number;
    created_at: string;
    updated_at: string;
  }>;
  total: number;  // Total matching breadcrumbs
}
```

**Examples**:
```json
// List all auth breadcrumbs
{
  "prefix": "auth."
}

// Get 10 most recently updated breadcrumbs
{
  "limit": 10,
  "sort": "updated",
  "order": "desc"
}
```

### delete_breadcrumb

**Purpose**: Remove a breadcrumb by key

**Parameters**:
```typescript
{
  key: string;  // Required: exact key to delete
}
```

**Returns**:
```typescript
{
  success: boolean;
  deleted: boolean;  // false if key didn't exist
}
```

**Example**:
```json
{
  "key": "error.temp_fix"
}
```

## Query Patterns

### Common Query Scenarios

#### 1. Get by Exact Key

**Use case**: Retrieve specific known information

```bash
# CLI
syn breadcrumb get db.connection

# MCP
get_breadcrumb({ key: "db.connection" })
```

#### 2. List by Prefix

**Use case**: Get all breadcrumbs in a namespace

```bash
# CLI - All auth breadcrumbs
syn breadcrumb list auth.

# MCP
list_breadcrumbs({ prefix: "auth." })
```

#### 3. Get Most Recent N Breadcrumbs

**Use case**: See what was discovered recently

```bash
# CLI
syn breadcrumb list --sort updated --order desc --limit 10

# MCP
list_breadcrumbs({
  limit: 10,
  sort: "updated",
  order: "desc"
})
```

#### 4. Find Breadcrumbs by Task

**Use case**: See all discoveries from a specific task

```bash
# This requires a filtered list operation
syn breadcrumb list --task-id 42 --format json

# MCP
list_breadcrumbs({ task_id: 42 })
```

#### 5. Find Stale Breadcrumbs

**Use case**: Identify outdated knowledge

```bash
# This requires date filtering (future enhancement)
syn breadcrumb list --older-than 30d

# MCP
list_breadcrumbs({
  updated_before: "2025-11-21T00:00:00Z"
})
```

## Implementation Phases

### Phase 1: Core Storage (MVP)

- [ ] Define `Breadcrumb` struct in `pkg/types/breadcrumb.go`
- [ ] Implement JSONL read/write in `internal/storage/breadcrumb.go`
- [ ] Add basic CLI commands: `set`, `get`, `list`, `delete`
- [ ] Unit tests for storage layer

### Phase 2: MCP Integration

- [ ] Implement MCP tools in `internal/mcp/breadcrumb.go`
- [ ] Add breadcrumb operations to MCP server
- [ ] Integration tests for MCP tools

### Phase 3: Advanced Queries

- [ ] Add prefix filtering to `list`
- [ ] Implement sorting by created/updated timestamps
- [ ] Add task-based filtering
- [ ] Date-based filtering (stale detection)

### Phase 4: Enhancements

- [ ] SQLite caching for fast queries (like task index)
- [ ] Custom merge driver for Git conflicts
- [ ] Breadcrumb validation (namespace constraints)
- [ ] Import/export functionality

## Performance Considerations

### Target Performance

- **Set operation**: <5ms (append to JSONL)
- **Get operation**: <10ms (linear scan, <1000 breadcrumbs)
- **List operation**: <50ms (full file read + filter)

### Scaling Strategy

**Small projects (<100 breadcrumbs)**:
- JSONL file read on every operation (simple, no caching)

**Medium projects (100-1000 breadcrumbs)**:
- In-memory cache loaded on first access
- Write-through cache on updates

**Large projects (>1000 breadcrumbs)**:
- SQLite index (like task index)
- Rebuild from JSONL on startup
- Fast prefix queries with B-tree index

### Cache Invalidation

If implementing SQLite caching:
- `.synapse/breadcrumbs.db` (ignored in Git)
- Rebuild from JSONL if file modified timestamp changed
- Similar pattern to existing task index

## Security Considerations

### Sensitive Data

**Problem**: Breadcrumbs may contain secrets (API keys, passwords)

**Mitigations**:
1. **Documentation warning**: Clearly state breadcrumbs are Git-tracked
2. **Namespace convention**: Use `secret.*` namespace with .gitignore pattern
3. **Encryption option (future)**: Encrypt values with local key

Example `.gitignore` entry:
```
# Ignore sensitive breadcrumbs
.synapse/breadcrumbs-secret.jsonl
```

### Input Validation

- **Key validation**: Alphanumeric + dots + underscores only
- **Value size limits**: Max 10KB per value (prevent abuse)
- **Total file size monitoring**: Warn if `breadcrumbs.jsonl` exceeds 1MB

## Migration Strategy

Since this is a new feature, no migration needed. However, for future-proofing:

### Version 1.0 → 2.0 (Hypothetical)

If schema changes (e.g., adding `metadata` field):

```go
type Breadcrumb struct {
    Key        string                 `json:"key"`
    Value      string                 `json:"value"`
    TaskID     int                    `json:"task_id,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"` // NEW
    CreatedAt  time.Time              `json:"created_at"`
    UpdatedAt  time.Time              `json:"updated_at"`
}
```

Migration tool:
```bash
syn breadcrumb migrate --from 1.0 --to 2.0
```

Backward compatibility: Old files without `metadata` field should parse successfully (omitempty).

## Testing Strategy

### Unit Tests

```go
// internal/storage/breadcrumb_test.go
func TestBreadcrumbStorage(t *testing.T) {
    // Test JSONL write/read
    // Test update vs create
    // Test prefix filtering
    // Test sorting
}
```

### Integration Tests

```bash
# Test CLI workflow
syn init
syn breadcrumb set test.key "test value"
syn breadcrumb get test.key
# Should output: test value

syn breadcrumb list test.
# Should show test.key entry

syn breadcrumb delete test.key --confirm
syn breadcrumb get test.key
# Should exit with code 1 (not found)
```

### MCP Tests

```go
// internal/mcp/breadcrumb_test.go
func TestMCPBreadcrumbTools(t *testing.T) {
    // Test set_breadcrumb
    // Test get_breadcrumb
    // Test list_breadcrumbs
    // Test delete_breadcrumb
}
```

## Example Workflows

### Agent Discovery Workflow

```go
// Agent discovers authentication method during task execution
mcp.SetBreadcrumb(SetBreadcrumbParams{
    Key:    "auth.method",
    Value:  "JWT with RS256 signing",
    TaskID: currentTaskID,
})

// Later session, different agent needs auth info
breadcrumb := mcp.GetBreadcrumb(GetBreadcrumbParams{
    Key: "auth.method",
})
// Agent now knows auth method without re-discovery
```

### Human Oversight Workflow

```bash
# Human checks what agents have learned
syn breadcrumb list

# Reviews specific domain
syn breadcrumb list error.

# Deletes outdated solution
syn breadcrumb delete error.old_cors_fix --confirm
```

### Task Provenance Tracking

```bash
# Find all breadcrumbs from a specific investigation
syn breadcrumb list --task-id 42 --format json | jq '.[] | {key, value}'

# See what task discovered a solution
syn breadcrumb get error.timeout_fix --format json | jq '.task_id'
```

## Open Questions

1. **Value size limits**: Should we enforce max value size? (Suggested: 10KB)
2. **Key uniqueness**: Should keys be globally unique or scoped by project? (Suggested: globally unique within project)
3. **Expiration**: Should breadcrumbs support TTL/expiration? (Suggested: Phase 4 enhancement)
4. **Encryption**: Should we support encrypted values? (Suggested: Phase 4, optional namespace-based)
5. **Audit log**: Should we track history of value changes? (Suggested: Future, separate audit.jsonl)

## References

- **Task storage**: `.synapse/memory.jsonl` implementation pattern
- **MCP specification**: JSON-RPC tool definitions
- **JSONL format**: [JSON Lines specification](https://jsonlines.org/)
- **Git merge strategies**: Custom merge drivers for JSONL files
