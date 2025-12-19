This is a comprehensive engineering development plan for **"Synapse"**—a next-generation agent memory system inspired by Beads but architected specifically for the Claude Code ecosystem with improvements in portability, visualization, and human-agent "vibe" interaction.

---

# Product Name: Synapse

**Tagline:** *The shared nervous system for Vibe Coders and their Agents.*

## 1. Executive Summary

**Synapse** is a lightweight, local-first, Git-backed issue tracker designed to serve as the persistent "long-term memory" for AI agents. It solves the "dementia problem" (forgetting context between sessions) and the "lost work problem" (noticing bugs but failing to log them).

**Key Differentiator vs. Beads:**
While Beads is a general-purpose tool, **Synapse** is purpose-built for **Claude Code**. It features a **CGO-free architecture** (runs on any OS without compilation errors), a **built-in MCP Server** (no external plugins required), and a **local visualization engine** to help the human "vibe coder" see the project state at a glance.

---

## 2. Engineering Requirements (PRD)

### Core Functional Requirements

1. **Atomic Memory Units:**
* Store tasks as "Synapses" (Issues) with: `ID`, `Title`, `Description`, `Status`, `ParentID`, `Blockers`, and `DiscoveredFrom`.
* **Status States:** `Open`, `In-Progress`, `Blocked`, `Review`, `Done`.


2. **The "Next-Action" Engine:**
* Must implement a topological sort to return *only* unblocked, high-priority work when the agent asks "What's next?".
* **Output:** Strictly typed JSON for the agent; rich text for the human.


3. **Git-Native Persistence:**
* All data stored in `.synapse/memory.jsonl` (JSON Lines).
* Updates must be merge-friendly.
* **Merge Driver:** Include a custom Git merge driver to handle concurrent agent updates without conflicts.


4. **Native MCP Support:**
* Command `synapse serve` starts an MCP (Model Context Protocol) server immediately, exposing tools like `read_memory`, `log_memory`, and `claim_task` directly to Claude Desktop or Claude Code.


5. **Sub-Agent Awareness:**
* Support an `Assignee` field that recognizes generic roles (e.g., "QA-Agent", "Refactor-Agent") to support Claude Code's sub-agent architecture.



### Non-Functional Requirements

* **Zero-Dependency Binary:** Written in **Go** using a *pure-Go* SQLite driver (removes Beads' dependency on CGO/glibc, making it work on Alpine Linux, old distros, and Windows without hassle).
* **Speed:** Operations must complete in <50ms.
* **Context Safety:** The database must be rebuildable entirely from the JSONL file if the SQLite cache is corrupted.

---

## 3. Architecture & Dependencies

### Tech Stack

* **Language:** Go (Golang) 1.23+
* **Database:** `modernc.org/sqlite` (Pure Go port of SQLite - **Critical improvement** for portability).
* **Protocol:** Model Context Protocol (MCP) via JSON-RPC.
* **Visualization:** HTML template with **Mermaid.js** (embedded in the binary).

### Directory Structure

```text
project_root/
├── .synapse/
│   ├── memory.jsonl      # The Source of Truth (Git tracked)
│   ├── index.db          # SQLite Cache (Git ignored)
│   └── config.json       # Agent roles and custom states
├── CLAUDE.md             # Auto-managed instructions

```

---

## 4. Development Instructions (Step-by-Step)

### Phase 1: The Core (Day 1-2)

1. **Initialize:** `go mod init github.com/yourname/synapse`
2. **Define Structs:**
```go
type Synapse struct {
    ID             int      `json:"id"`
    Title          string   `json:"title"`
    Status         string   `json:"status"` // open, claimed, done
    BlockedBy      []int    `json:"blocked_by"`
    ParentID       int      `json:"parent_id,omitempty"`
    Assignee       string   `json:"assignee,omitempty"` // For sub-agents
}

```


3. **Implement Storage:**
* Write the `Load()` function: Read `memory.jsonl` line-by-line into the SQLite cache.
* Write the `Save()` function: Dump new issues to `memory.jsonl` as append-only or rewrite (ensure deterministic sorting for Git diffs).



### Phase 2: The Agent Interface (Day 3)

1. **Build CLI Commands:**
* `syn add "Fix login bug" --blocks 4 --parent 2`
* `syn ready --json` (The most critical command for the agent).


2. **Logic:** Implement the "Ready" filter.
* *Rule:* A task is `Ready` if `Status == Open` AND `len(UnresolvedBlockers) == 0`.



### Phase 3: The Integrations (Day 4)

1. **MCP Server:**
* Implement `syn serve`. It should listen on Stdio.
* Expose tools: `create_task`, `update_task`, `get_next_task`.


2. **Claude Code Hook:**
* Create a `syn init` command that appends a strict prompt to `CLAUDE.md` or `AGENTS.md`:
> "Before starting work, ALWAYS run `syn ready --json`. Create new issues for any bugs you find using `syn add`."





---

## 5. Improvements Over Beads for "Vibe Coding"

Here are specific ways Synapse will outperform the original Beads implementation for your specific goal:

### 1. Pure Portability (The "It Just Works" Factor)

* **Beads Limitation:** Uses CGO (links against C libraries). This often breaks on different Linux versions (glibc errors) or requires complex Windows setups.
* **Synapse Solution:** Use a **transpiled SQLite (pure Go)**. The resulting binary is static. You can drop it into a container, a remote SSH session, or a Windows laptop, and it runs immediately.

### 2. The "Vibe" Visualizer

* **Beads Limitation:** Text/JSON only. Hard for humans to visualize complex dependency trees.
* **Synapse Solution:** Add a command `syn view`.
* This spins up a localhost server on port 8080.
* Renders a **Live DAG (Directed Acyclic Graph)** using Mermaid.js or D3.
* *Benefit:* You can keep this open on a second monitor while vibe coding to see "the plan" evolve in real-time as the agent works.



### 3. Sub-Agent Specialization

* **Beads Limitation:** "Assignees" are loose text fields.
* **Synapse Solution:** Strongly typed **Agent Roles**.
* You can assign an issue to `@qa`, `@architect`, or `@coder`.
* When you spin up a specific sub-agent in Claude Code, it only queries for tasks assigned to its role.
* *Benefit:* Enables parallel vibe coding (one agent fixing bugs, one agent building features) without them colliding.



### 4. "Ghost Mode" (Context Injection)

* **Beads Limitation:** Passive. The agent must remember to query it.
* **Synapse Solution:** **Active MCP Context.**
* As an MCP server, Synapse can *push* the "Current Objective" into Claude's context window as a "Resource".
* This means Claude *always* knows what issue it is working on without even asking.
