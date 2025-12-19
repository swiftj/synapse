# SQLite Cache Implementation

## Overview

High-performance SQLite cache layer for the Synapse project using `modernc.org/sqlite` (pure Go, no CGO dependency).

**Location**: `/Users/johnswift/Projects/synapse/internal/storage/sqlite.go`

## Architecture

### Design Principles

1. **Source of Truth**: JSONL files remain the authoritative data source
2. **Rebuildable Cache**: Cache can be completely rebuilt from JSONL at any time
3. **Performance Target**: All operations complete in <50ms
4. **Thread Safety**: Uses RWMutex for concurrent access protection
5. **Pure Go**: No CGO dependencies via `modernc.org/sqlite`

### Database Schema

```sql
-- Main synapses table
CREATE TABLE synapses (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    parent_id INTEGER,
    assignee TEXT,
    discovered_from TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Many-to-many blocking relationship
CREATE TABLE blockers (
    synapse_id INTEGER NOT NULL,
    blocker_id INTEGER NOT NULL,
    PRIMARY KEY (synapse_id, blocker_id),
    FOREIGN KEY (synapse_id) REFERENCES synapses(id) ON DELETE CASCADE,
    FOREIGN KEY (blocker_id) REFERENCES synapses(id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX idx_synapses_status ON synapses(status);
CREATE INDEX idx_synapses_assignee ON synapses(assignee);
CREATE INDEX idx_synapses_parent ON synapses(parent_id);
CREATE INDEX idx_blockers_synapse ON blockers(synapse_id);
CREATE INDEX idx_blockers_blocker ON blockers(blocker_id);
```

## API Reference

### Initialization

```go
cache := storage.NewSQLiteCache(dbPath)
if err := cache.Init(); err != nil {
    log.Fatal(err)
}
defer cache.Close()
```

### Sync Operations

```go
// Rebuild cache from JSONL data
synapses := jsonlStore.All()
if err := cache.Rebuild(synapses); err != nil {
    log.Fatal(err)
}
```

### CRUD Operations

```go
// Insert
if err := cache.Insert(synapse); err != nil {
    log.Fatal(err)
}

// Update
synapse.Status = types.StatusInProgress
if err := cache.Update(synapse); err != nil {
    log.Fatal(err)
}

// Delete
if err := cache.Delete(id); err != nil {
    log.Fatal(err)
}

// Get single
synapse, err := cache.Get(id)
if err != nil {
    log.Fatal(err)
}
```

### Query Operations

```go
// All synapses (sorted by ID)
all, err := cache.All()

// Ready to work (efficient SQL query)
ready, err := cache.Ready()

// Filter by status
open, err := cache.ByStatus(types.StatusOpen)

// Filter by assignee
tasks, err := cache.ByAssignee("backend-dev")
```

### Utility Operations

```go
// Statistics
stats, err := cache.GetStats()
fmt.Printf("Synapses: %d, Ready: %d, Size: %d bytes\n",
    stats.SynapseCount, stats.ReadyCount, stats.DatabaseSizeB)

// Optimize database
cache.Vacuum()
cache.Analyze()

// Query plan analysis (debugging)
plan, err := cache.ExplainQuery("SELECT * FROM synapses WHERE status = ?", "open")
```

## Performance Characteristics

### Query Optimization

The `Ready()` query uses an efficient SQL approach:

```sql
SELECT s.* FROM synapses s
WHERE s.status IN ('open', 'blocked')
AND NOT EXISTS (
    SELECT 1 FROM blockers b
    JOIN synapses blocker ON b.blocker_id = blocker.id
    WHERE b.synapse_id = s.id
    AND blocker.status != 'done'
)
ORDER BY s.id
```

**Benefits**:
- Single SQL query with joins
- Index-optimized subquery
- No application-level filtering
- Handles complex blocking relationships efficiently

### Performance Targets

All operations designed for <50ms completion:

| Operation | Expected Time | Tested With |
|-----------|--------------|-------------|
| `Rebuild()` | <50ms | 5-1000 synapses |
| `Get()` | <10ms | Single lookup |
| `All()` | <50ms | 1000 synapses |
| `Ready()` | <50ms | Complex query |
| `ByStatus()` | <30ms | Indexed query |
| `ByAssignee()` | <30ms | Indexed query |
| `Insert()` | <20ms | With blockers |
| `Update()` | <30ms | With blocker update |
| `Delete()` | <10ms | Cascade delete |

### Connection Configuration

```go
db.SetMaxOpenConns(1)    // SQLite serializes writes
db.SetMaxIdleConns(1)     // Single persistent connection
db.SetConnMaxLifetime(0)  // No connection expiry
```

## Usage Patterns

### Pattern 1: Dual-Store Architecture

```go
// JSONL is source of truth
jsonlStore := storage.NewJSONLStore(".synapse")
jsonlStore.Load()

// SQLite provides fast queries
cache := storage.NewSQLiteCache(".synapse/cache.db")
cache.Init()
cache.Rebuild(jsonlStore.All())

// Write to both
synapse := types.NewSynapse(1, "Task")
jsonlStore.Create(synapse.Title)
jsonlStore.Save()
cache.Rebuild(jsonlStore.All())  // or cache.Insert(synapse)

// Read from cache
ready := cache.Ready()
```

### Pattern 2: Periodic Sync

```go
// Initial load
cache.Rebuild(jsonlStore.All())

// Fast operations use cache
ready := cache.Ready()

// Periodic full rebuild (ensure consistency)
ticker := time.NewTicker(5 * time.Minute)
go func() {
    for range ticker.C {
        jsonlStore.Load()
        cache.Rebuild(jsonlStore.All())
    }
}()
```

### Pattern 3: Transaction Wrapper

```go
func UpdateSynapse(id int, updates func(*types.Synapse)) error {
    // Load from JSONL
    syn, err := jsonlStore.Get(id)
    if err != nil {
        return err
    }

    // Apply updates
    updates(syn)

    // Save to JSONL (source of truth)
    if err := jsonlStore.Update(syn); err != nil {
        return err
    }
    if err := jsonlStore.Save(); err != nil {
        return err
    }

    // Update cache
    return cache.Update(syn)
}
```

## Thread Safety

### Lock Strategy

- **RWMutex**: Separates readers from writers
- **Read operations**: `RLock()` allows concurrent reads
- **Write operations**: `Lock()` ensures exclusive access
- **Internal queries**: Inherit parent lock (loadBlockers)

### Concurrent Access

```go
// Multiple concurrent readers - safe
go cache.All()
go cache.Ready()
go cache.Get(1)

// Writers are serialized
go cache.Insert(syn1)   // Blocks
go cache.Update(syn2)   // Waits
go cache.Delete(3)      // Waits
```

## Testing

Comprehensive test suite in `/Users/johnswift/Projects/synapse/internal/storage/sqlite_test.go`:

```bash
# Run all SQLite tests
go test ./internal/storage -run TestSQLiteCache -v

# Run benchmarks
go test ./internal/storage -bench BenchmarkSQLiteCache -benchmem

# Run demo
go run ./examples/cache_demo.go
```

### Test Coverage

- ✅ Initialization and schema creation
- ✅ Rebuild from JSONL data
- ✅ Insert/Update/Delete operations
- ✅ Get single synapse with blockers
- ✅ All synapses query
- ✅ Ready tasks (complex blocker logic)
- ✅ Filter by status
- ✅ Filter by assignee
- ✅ Statistics and monitoring
- ✅ Empty blocker lists
- ✅ Nullable field handling
- ✅ Performance with 1000 synapses
- ✅ Concurrent read safety
- ✅ Benchmarks for all operations

## Migration from JSONL

No migration required - cache is ephemeral and can be rebuilt:

```go
// Delete old cache
os.Remove(".synapse/cache.db")

// Rebuild from JSONL
jsonlStore.Load()
cache.Init()
cache.Rebuild(jsonlStore.All())
```

## Monitoring

### Statistics Tracking

```go
stats, err := cache.GetStats()
if err != nil {
    log.Fatal(err)
}

log.Printf("Cache health:")
log.Printf("  Synapses: %d", stats.SynapseCount)
log.Printf("  Blockers: %d", stats.BlockerCount)
log.Printf("  Ready:    %d", stats.ReadyCount)
log.Printf("  Size:     %d bytes", stats.DatabaseSizeB)
```

### Query Analysis

```go
// Explain query plan for optimization
plan, err := cache.ExplainQuery(`
    SELECT * FROM synapses WHERE status = ?
`, "open")
fmt.Println(plan)
```

### Maintenance Operations

```go
// Reclaim space
cache.Vacuum()

// Update query optimizer statistics
cache.Analyze()
```

## Implementation Details

### Blocker Handling

Blockers are stored in a separate table for normalization:

```go
// On Insert/Update: delete old + insert new
tx.Exec("DELETE FROM blockers WHERE synapse_id = ?", id)
for _, blockerID := range syn.BlockedBy {
    tx.Exec("INSERT INTO blockers VALUES (?, ?)", id, blockerID)
}
```

### Timestamp Format

Uses RFC3339Nano for precision and timezone awareness:

```go
createdAt := syn.CreatedAt.Format(time.RFC3339Nano)
parsed, _ := time.Parse(time.RFC3339Nano, createdAt)
```

### NULL Handling

Helper functions for optional fields:

```go
func nullString(s string) sql.NullString {
    if s == "" {
        return sql.NullString{Valid: false}
    }
    return sql.NullString{String: s, Valid: true}
}

func nullInt(i int) sql.NullInt64 {
    if i == 0 {
        return sql.NullInt64{Valid: false}
    }
    return sql.NullInt64{Int64: int64(i), Valid: true}
}
```

## Future Enhancements

Potential optimizations for larger datasets:

1. **Prepared Statement Pool**: Reuse prepared statements
2. **Batch Operations**: Bulk insert/update methods
3. **Write-Ahead Logging**: Enable WAL mode for better concurrency
4. **Memory-Mapped I/O**: For read-heavy workloads
5. **Partial Indexes**: For specific query patterns
6. **Materialized Views**: For complex aggregations

## Dependencies

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"  // Pure Go SQLite driver

    "github.com/johnswift/synapse/pkg/types"
)
```

**Why modernc.org/sqlite?**
- Pure Go implementation (no CGO)
- Cross-platform compatibility
- Easier builds and deployment
- Full SQLite feature support
- Active maintenance

## Files Created

1. `/Users/johnswift/Projects/synapse/internal/storage/sqlite.go` - Implementation (650 lines)
2. `/Users/johnswift/Projects/synapse/internal/storage/sqlite_test.go` - Tests (600 lines)
3. `/Users/johnswift/Projects/synapse/examples/cache_demo.go` - Demo program (150 lines)
4. This documentation file

## Summary

The SQLite cache layer provides:

- ✅ Pure Go implementation (no CGO)
- ✅ <50ms query performance
- ✅ Efficient blocking relationship queries
- ✅ Thread-safe concurrent access
- ✅ Rebuildable from JSONL source of truth
- ✅ Comprehensive test coverage
- ✅ Monitoring and statistics
- ✅ Production-ready error handling

The implementation follows Go best practices and integrates seamlessly with the existing JSONL-based storage system.
