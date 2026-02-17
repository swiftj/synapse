# Synapse Workflow Patterns

## Multi-Session Workflow

### Starting a New Session

```
1. get_context_window(minutes: 120)     → See what happened recently
2. list_breadcrumbs(prefix: "session.") → Recover prior session state
3. get_next_task()                       → Find highest priority work
4. claim_task(id, agent_id)              → Lock it for yourself
```

### Ending a Session

```
1. add_note(id, "Progress: completed X, Y remains")
2. set_breadcrumb("session.last_task", "<id>")
3. set_breadcrumb("session.context", "Brief description of state")
4. complete_task(id) OR update_task(id, status: "review")
```

### Resuming After Interruption

```
1. my_tasks(agent_id)                    → Find your claimed tasks
2. get_task(id)                          → Read notes for context
3. get_breadcrumb("session.context")     → Recall session state
4. Continue working
```

## Parallel Agent Coordination

### Agent Specialization Pattern

Assign roles to agents using the `assignee` field:

- `@architect` — System design, dependency graphs, API contracts
- `@coder` — Implementation, bug fixes, feature work
- `@qa` — Testing, validation, edge case discovery
- `@reviewer` — Code review, documentation, standards compliance

### Coordination Protocol

```
Agent A:                          Agent B:
  get_next_task(assignee: @coder)   get_next_task(assignee: @qa)
  claim_task(5, "coder-1")          claim_task(8, "qa-1")
  ... work ...                       ... work ...
  spawn_task(5, "Edge case found")  complete_task_as(8, "qa-1")
  complete_task_as(5, "coder-1")
```

### Avoiding Conflicts

1. Always `claim_task` before starting work
2. Use unique `agent_id` values per agent instance
3. Claims auto-expire after 30 minutes (configurable)
4. Check `my_tasks` to see current assignments
5. Use `release_claim` if you need to switch tasks

## Dependency Graph Patterns

### Linear Pipeline

```
Task 1: "Design API" (open)
Task 2: "Implement API" (blocked_by: [1])
Task 3: "Write tests" (blocked_by: [2])
Task 4: "Deploy" (blocked_by: [3])
```

Only Task 1 appears in `get_next_task`. As each completes, the next unblocks.

### Fan-Out / Fan-In

```
Task 1: "Design schema" (open)
Task 2: "Build auth service" (blocked_by: [1])
Task 3: "Build user service" (blocked_by: [1])
Task 4: "Integration tests" (blocked_by: [2, 3])
```

After Task 1 completes, Tasks 2 and 3 can be worked in parallel.
Task 4 unblocks only when both are done.

### Discovery-Driven

```
Working on Task 5...
  → discover bug → spawn_task(5, "Fix null check in auth")
  → discover debt → spawn_task(5, "Refactor validation layer")

Spawned tasks auto-link via discovered_from for provenance.
```

## Breadcrumb Strategies

### Architecture Decision Records

```
set_breadcrumb("arch.auth", "JWT with refresh tokens, 15min access expiry")
set_breadcrumb("arch.db", "PostgreSQL with pgx driver, connection pooling")
set_breadcrumb("arch.cache", "Redis for session store, 30min TTL")
```

### Investigation Trail

```
set_breadcrumb("bug.42.symptom", "Login fails after token refresh")
set_breadcrumb("bug.42.root_cause", "Race condition in refresh endpoint")
set_breadcrumb("bug.42.fix", "Added mutex around token rotation")
```

### Environment Context

```
set_breadcrumb("env.go_version", "1.23")
set_breadcrumb("env.node_version", "20.11")
set_breadcrumb("env.db_url", "localhost:5432/myapp")
```

### Session Handoff

When handing off between agents or sessions:

```
set_breadcrumb("session.goal", "Complete auth module")
set_breadcrumb("session.blocked_on", "Waiting for API keys from team")
set_breadcrumb("session.next_step", "Implement refresh token rotation")
set_breadcrumb("session.files_modified", "internal/auth/handler.go, internal/auth/token.go")
```

## Keeping Synapse Accurate

Synapse should be the single source of truth for project progress. A new agent
or human should be able to query `list_tasks` and `list_breadcrumbs` and
immediately understand what's done, what's in flight, and what's next — without
reading source code.

### Plan Decomposition Pattern

When given a multi-step plan or task list:

```
1. Create a task for EACH step before starting any work
2. Set blocked_by relationships to encode the order
3. Claim and start the first unblocked task
4. Mark each task done IMMEDIATELY when finished
5. Claim the next task before starting it

Example:
  create_task("Design schema", priority: 5)           → #10
  create_task("Build API", blocked_by: [10])           → #11
  create_task("Write tests", blocked_by: [11])         → #12
  claim_task(10, "agent-1")
  ... work on schema ...
  complete_task(10)      ← do this NOW, not later
  claim_task(11, "agent-1")
  ... work on API ...
```

### Real-Time Status Updates

Update status as events happen, not in batches:

```
Discovered a blocker?
  → update_task(id, status: "blocked")
  → add_note(id, "Blocked: missing API credentials")

Made meaningful progress?
  → add_note(id, "Completed: endpoints for /users and /auth")

Need someone else to verify?
  → update_task(id, status: "review")

Found a new issue while working?
  → spawn_task(current_id, "Fix edge case in validation")
  ← Don't wait, log it immediately
```

### Progress Breadcrumbs

At natural pause points, capture session state:

```
set_breadcrumb("session.current_task", "11")
set_breadcrumb("session.progress", "API endpoints done, middleware next")
set_breadcrumb("session.next_step", "Add auth middleware to router")
set_breadcrumb("session.files_touched", "internal/api/handler.go, internal/api/routes.go")
```

This lets the next session resume instantly without re-reading code.

## Task Lifecycle Best Practices

1. **Be specific in titles**: "Fix null pointer in auth middleware" not "Fix bug"
2. **Use labels**: Categorize with `bug`, `feature`, `security`, `refactor`
3. **Set priority**: Critical issues get 10+, nice-to-haves get 0
4. **Add notes as you go**: Future sessions will thank you
5. **Use breadcrumbs for decisions**: Record WHY, not just WHAT
6. **Clean up**: `delete_task` completed items periodically, or use `syn delete --done`
