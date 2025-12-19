// Package storage provides persistence for Synapse data.
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/swiftj/synapse/pkg/types"
)

const (
	// SQLiteCacheFile is the default cache database filename.
	SQLiteCacheFile = "cache.db"
)

// SQLiteCache provides a fast query layer over JSONL source of truth.
// It can be rebuilt from JSONL data and serves as a performance optimization.
type SQLiteCache struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// NewSQLiteCache creates a new SQLite cache at the given path.
func NewSQLiteCache(dbPath string) *SQLiteCache {
	return &SQLiteCache{
		path: dbPath,
	}
}

// Init creates the database schema if it doesn't exist.
func (c *SQLiteCache) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	db, err := sql.Open("sqlite", c.path)
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}

	// Configure connection pool for performance
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	c.db = db

	// Create schema with optimized indexes
	schema := `
	CREATE TABLE IF NOT EXISTS synapses (
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

	CREATE INDEX IF NOT EXISTS idx_synapses_status ON synapses(status);
	CREATE INDEX IF NOT EXISTS idx_synapses_assignee ON synapses(assignee);
	CREATE INDEX IF NOT EXISTS idx_synapses_parent ON synapses(parent_id);

	CREATE TABLE IF NOT EXISTS blockers (
		synapse_id INTEGER NOT NULL,
		blocker_id INTEGER NOT NULL,
		PRIMARY KEY (synapse_id, blocker_id),
		FOREIGN KEY (synapse_id) REFERENCES synapses(id) ON DELETE CASCADE,
		FOREIGN KEY (blocker_id) REFERENCES synapses(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_blockers_synapse ON blockers(synapse_id);
	CREATE INDEX IF NOT EXISTS idx_blockers_blocker ON blockers(blocker_id);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}

// Rebuild clears the cache and rebuilds it from JSONL data.
// This is the primary sync mechanism to ensure cache matches source of truth.
func (c *SQLiteCache) Rebuild(synapses []*types.Synapse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.Exec("DELETE FROM blockers"); err != nil {
		return fmt.Errorf("clear blockers: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM synapses"); err != nil {
		return fmt.Errorf("clear synapses: %w", err)
	}

	// Prepare statements for batch insert
	synStmt, err := tx.Prepare(`
		INSERT INTO synapses (id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare synapse insert: %w", err)
	}
	defer synStmt.Close()

	blockStmt, err := tx.Prepare(`
		INSERT INTO blockers (synapse_id, blocker_id)
		VALUES (?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare blocker insert: %w", err)
	}
	defer blockStmt.Close()

	// Insert all synapses
	for _, syn := range synapses {
		_, err := synStmt.Exec(
			syn.ID,
			syn.Title,
			nullString(syn.Description),
			string(syn.Status),
			nullInt(syn.ParentID),
			nullString(syn.Assignee),
			nullString(syn.DiscoveredFrom),
			syn.CreatedAt.Format(time.RFC3339Nano),
			syn.UpdatedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("insert synapse %d: %w", syn.ID, err)
		}

		// Insert blockers
		for _, blockerID := range syn.BlockedBy {
			if _, err := blockStmt.Exec(syn.ID, blockerID); err != nil {
				return fmt.Errorf("insert blocker %d->%d: %w", syn.ID, blockerID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Insert adds a new synapse to the cache.
func (c *SQLiteCache) Insert(syn *types.Synapse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO synapses (id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		syn.ID,
		syn.Title,
		nullString(syn.Description),
		string(syn.Status),
		nullInt(syn.ParentID),
		nullString(syn.Assignee),
		nullString(syn.DiscoveredFrom),
		syn.CreatedAt.Format(time.RFC3339Nano),
		syn.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert synapse: %w", err)
	}

	// Insert blockers
	for _, blockerID := range syn.BlockedBy {
		_, err := tx.Exec(`
			INSERT INTO blockers (synapse_id, blocker_id)
			VALUES (?, ?)
		`, syn.ID, blockerID)
		if err != nil {
			return fmt.Errorf("insert blocker: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Update modifies an existing synapse in the cache.
func (c *SQLiteCache) Update(syn *types.Synapse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE synapses
		SET title = ?, description = ?, status = ?, parent_id = ?, assignee = ?,
		    discovered_from = ?, updated_at = ?
		WHERE id = ?
	`,
		syn.Title,
		nullString(syn.Description),
		string(syn.Status),
		nullInt(syn.ParentID),
		nullString(syn.Assignee),
		nullString(syn.DiscoveredFrom),
		syn.UpdatedAt.Format(time.RFC3339Nano),
		syn.ID,
	)
	if err != nil {
		return fmt.Errorf("update synapse: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("synapse %d not found", syn.ID)
	}

	// Update blockers - delete and re-insert for simplicity
	if _, err := tx.Exec("DELETE FROM blockers WHERE synapse_id = ?", syn.ID); err != nil {
		return fmt.Errorf("delete old blockers: %w", err)
	}

	for _, blockerID := range syn.BlockedBy {
		_, err := tx.Exec(`
			INSERT INTO blockers (synapse_id, blocker_id)
			VALUES (?, ?)
		`, syn.ID, blockerID)
		if err != nil {
			return fmt.Errorf("insert blocker: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Delete removes a synapse from the cache.
func (c *SQLiteCache) Delete(id int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result, err := c.db.Exec("DELETE FROM synapses WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete synapse: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("synapse %d not found", id)
	}

	return nil
}

// Get retrieves a single synapse by ID.
func (c *SQLiteCache) Get(id int) (*types.Synapse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var syn types.Synapse
	var description, assignee, discoveredFrom sql.NullString
	var parentID sql.NullInt64
	var createdAt, updatedAt string

	err := c.db.QueryRow(`
		SELECT id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at
		FROM synapses
		WHERE id = ?
	`, id).Scan(
		&syn.ID,
		&syn.Title,
		&description,
		&syn.Status,
		&parentID,
		&assignee,
		&discoveredFrom,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("synapse %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query synapse: %w", err)
	}

	// Parse nullable fields
	if description.Valid {
		syn.Description = description.String
	}
	if parentID.Valid {
		syn.ParentID = int(parentID.Int64)
	}
	if assignee.Valid {
		syn.Assignee = assignee.String
	}
	if discoveredFrom.Valid {
		syn.DiscoveredFrom = discoveredFrom.String
	}

	// Parse timestamps
	syn.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	syn.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	// Load blockers
	syn.BlockedBy, err = c.loadBlockers(syn.ID)
	if err != nil {
		return nil, fmt.Errorf("load blockers: %w", err)
	}

	return &syn, nil
}

// All retrieves all synapses ordered by ID.
func (c *SQLiteCache) All() ([]*types.Synapse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := c.db.Query(`
		SELECT id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at
		FROM synapses
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query synapses: %w", err)
	}
	defer rows.Close()

	return c.scanSynapses(rows)
}

// Ready retrieves all synapses that are ready to work on.
// A synapse is ready when:
// - Status is "open" or "blocked"
// - All blocking synapses have status "done"
func (c *SQLiteCache) Ready() ([]*types.Synapse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Efficient SQL query to find unblocked tasks
	// Join with blockers and filter out any with non-done blockers
	query := `
		SELECT s.id, s.title, s.description, s.status, s.parent_id, s.assignee,
		       s.discovered_from, s.created_at, s.updated_at
		FROM synapses s
		WHERE s.status IN ('open', 'blocked')
		AND NOT EXISTS (
			SELECT 1 FROM blockers b
			JOIN synapses blocker ON b.blocker_id = blocker.id
			WHERE b.synapse_id = s.id
			AND blocker.status != 'done'
		)
		ORDER BY s.id
	`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query ready synapses: %w", err)
	}
	defer rows.Close()

	return c.scanSynapses(rows)
}

// ByStatus retrieves all synapses with the given status.
func (c *SQLiteCache) ByStatus(status types.Status) ([]*types.Synapse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := c.db.Query(`
		SELECT id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at
		FROM synapses
		WHERE status = ?
		ORDER BY id
	`, string(status))
	if err != nil {
		return nil, fmt.Errorf("query synapses by status: %w", err)
	}
	defer rows.Close()

	return c.scanSynapses(rows)
}

// ByAssignee retrieves all synapses assigned to the given role.
func (c *SQLiteCache) ByAssignee(assignee string) ([]*types.Synapse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := c.db.Query(`
		SELECT id, title, description, status, parent_id, assignee, discovered_from, created_at, updated_at
		FROM synapses
		WHERE assignee = ?
		ORDER BY id
	`, assignee)
	if err != nil {
		return nil, fmt.Errorf("query synapses by assignee: %w", err)
	}
	defer rows.Close()

	return c.scanSynapses(rows)
}

// Close closes the database connection.
func (c *SQLiteCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("close database: %w", err)
		}
		c.db = nil
	}
	return nil
}

// scanSynapses is a helper to scan multiple rows into Synapse structs.
func (c *SQLiteCache) scanSynapses(rows *sql.Rows) ([]*types.Synapse, error) {
	var synapses []*types.Synapse

	for rows.Next() {
		var syn types.Synapse
		var description, assignee, discoveredFrom sql.NullString
		var parentID sql.NullInt64
		var createdAt, updatedAt string

		err := rows.Scan(
			&syn.ID,
			&syn.Title,
			&description,
			&syn.Status,
			&parentID,
			&assignee,
			&discoveredFrom,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan synapse: %w", err)
		}

		// Parse nullable fields
		if description.Valid {
			syn.Description = description.String
		}
		if parentID.Valid {
			syn.ParentID = int(parentID.Int64)
		}
		if assignee.Valid {
			syn.Assignee = assignee.String
		}
		if discoveredFrom.Valid {
			syn.DiscoveredFrom = discoveredFrom.String
		}

		// Parse timestamps
		syn.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		syn.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at: %w", err)
		}

		// Initialize empty blockers slice (will be populated after rows are closed)
		syn.BlockedBy = []int{}

		synapses = append(synapses, &syn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	// Close rows before loading blockers to avoid nested queries
	rows.Close()

	// Load blockers for all synapses in a single batch query
	if len(synapses) > 0 {
		blockerMap, err := c.loadAllBlockers()
		if err != nil {
			return nil, fmt.Errorf("load blockers: %w", err)
		}
		for _, syn := range synapses {
			if blockers, ok := blockerMap[syn.ID]; ok {
				syn.BlockedBy = blockers
			}
		}
	}

	return synapses, nil
}

// loadAllBlockers loads all blockers from the database and returns a map of synapse ID to blocker IDs.
// This is more efficient than N+1 queries and avoids nested query issues.
func (c *SQLiteCache) loadAllBlockers() (map[int][]int, error) {
	rows, err := c.db.Query(`
		SELECT synapse_id, blocker_id
		FROM blockers
		ORDER BY synapse_id, blocker_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blockerMap := make(map[int][]int)
	for rows.Next() {
		var synapseID, blockerID int
		if err := rows.Scan(&synapseID, &blockerID); err != nil {
			return nil, err
		}
		blockerMap[synapseID] = append(blockerMap[synapseID], blockerID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return blockerMap, nil
}

// loadBlockers is a helper to load the blocker IDs for a single synapse.
// Used for single-synapse queries like Get().
func (c *SQLiteCache) loadBlockers(synapseID int) ([]int, error) {
	rows, err := c.db.Query(`
		SELECT blocker_id
		FROM blockers
		WHERE synapse_id = ?
		ORDER BY blocker_id
	`, synapseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blockers []int
	for rows.Next() {
		var blockerID int
		if err := rows.Scan(&blockerID); err != nil {
			return nil, err
		}
		blockers = append(blockers, blockerID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil for consistency with JSONL store
	if blockers == nil {
		blockers = []int{}
	}

	return blockers, nil
}

// nullString converts a string to sql.NullString, treating empty string as NULL.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullInt converts an int to sql.NullInt64, treating 0 as NULL.
func nullInt(i int) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(i), Valid: true}
}

// Path returns the database file path.
func (c *SQLiteCache) Path() string {
	return c.path
}

// Vacuum optimizes the database file size.
func (c *SQLiteCache) Vacuum() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if _, err := c.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum database: %w", err)
	}

	return nil
}

// Stats returns database statistics for monitoring.
type Stats struct {
	SynapseCount  int
	BlockerCount  int
	ReadyCount    int
	DatabaseSizeB int64
}

// GetStats returns current cache statistics.
func (c *SQLiteCache) GetStats() (*Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := &Stats{}

	// Count synapses
	err := c.db.QueryRow("SELECT COUNT(*) FROM synapses").Scan(&stats.SynapseCount)
	if err != nil {
		return nil, fmt.Errorf("count synapses: %w", err)
	}

	// Count blockers
	err = c.db.QueryRow("SELECT COUNT(*) FROM blockers").Scan(&stats.BlockerCount)
	if err != nil {
		return nil, fmt.Errorf("count blockers: %w", err)
	}

	// Count ready tasks (using same logic as Ready())
	query := `
		SELECT COUNT(*)
		FROM synapses s
		WHERE s.status IN ('open', 'blocked')
		AND NOT EXISTS (
			SELECT 1 FROM blockers b
			JOIN synapses blocker ON b.blocker_id = blocker.id
			WHERE b.synapse_id = s.id
			AND blocker.status != 'done'
		)
	`
	err = c.db.QueryRow(query).Scan(&stats.ReadyCount)
	if err != nil {
		return nil, fmt.Errorf("count ready: %w", err)
	}

	// Get database file size
	var pageCount, pageSize int64
	err = c.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return nil, fmt.Errorf("get page count: %w", err)
	}
	err = c.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return nil, fmt.Errorf("get page size: %w", err)
	}
	stats.DatabaseSizeB = pageCount * pageSize

	return stats, nil
}

// Analyze updates query optimizer statistics for better performance.
func (c *SQLiteCache) Analyze() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if _, err := c.db.Exec("ANALYZE"); err != nil {
		return fmt.Errorf("analyze database: %w", err)
	}

	return nil
}

// Debug helper to explain query plans.
func (c *SQLiteCache) ExplainQuery(query string, args ...interface{}) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := c.db.Query(explainQuery, args...)
	if err != nil {
		return "", fmt.Errorf("explain query: %w", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			return "", err
		}
		plan.WriteString(fmt.Sprintf("%d|%d: %s\n", id, parent, detail))
	}

	return plan.String(), rows.Err()
}
