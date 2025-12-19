package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/johnswift/synapse/pkg/types"
)

func setupTestCache(t *testing.T) (*SQLiteCache, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cache := NewSQLiteCache(dbPath)
	if err := cache.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	cleanup := func() {
		cache.Close()
	}

	return cache, cleanup
}

func createTestSynapses() []*types.Synapse {
	now := time.Now().UTC()

	return []*types.Synapse{
		{
			ID:          1,
			Title:       "Design API",
			Description: "Design REST API endpoints",
			Status:      types.StatusDone,
			Assignee:    "architect",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{},
		},
		{
			ID:          2,
			Title:       "Implement handlers",
			Description: "Implement HTTP handlers",
			Status:      types.StatusOpen,
			Assignee:    "backend",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{1}, // Blocked by completed task - should be ready
		},
		{
			ID:          3,
			Title:       "Write tests",
			Status:      types.StatusBlocked,
			Assignee:    "backend",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{2}, // Blocked by open task - not ready
		},
		{
			ID:        4,
			Title:     "Deploy to staging",
			Status:    types.StatusOpen,
			CreatedAt: now,
			UpdatedAt: now,
			BlockedBy: []int{1, 2}, // One done, one open - not ready
		},
		{
			ID:        5,
			Title:     "Update docs",
			Status:    types.StatusInProgress,
			Assignee:  "tech-writer",
			CreatedAt: now,
			UpdatedAt: now,
			BlockedBy: []int{},
		},
	}
}

func TestSQLiteCache_InitAndClose(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	if cache.db == nil {
		t.Fatal("database not initialized")
	}
}

func TestSQLiteCache_Rebuild(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()

	start := time.Now()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("Rebuild took %v, expected < 50ms", elapsed)
	}

	// Verify all synapses were inserted
	all, err := cache.All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}
	if len(all) != len(synapses) {
		t.Errorf("got %d synapses, want %d", len(all), len(synapses))
	}
}

func TestSQLiteCache_InsertUpdateDelete(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	syn := &types.Synapse{
		ID:          100,
		Title:       "Test task",
		Description: "Test description",
		Status:      types.StatusOpen,
		Assignee:    "tester",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		BlockedBy:   []int{1, 2},
	}

	// Test Insert
	start := time.Now()
	if err := cache.Insert(syn); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Errorf("Insert took %v, expected < 50ms", time.Since(start))
	}

	// Verify Insert
	retrieved, err := cache.Get(100)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Title != syn.Title {
		t.Errorf("got title %q, want %q", retrieved.Title, syn.Title)
	}
	if len(retrieved.BlockedBy) != 2 {
		t.Errorf("got %d blockers, want 2", len(retrieved.BlockedBy))
	}

	// Test Update
	syn.Title = "Updated title"
	syn.Status = types.StatusInProgress
	syn.BlockedBy = []int{3}

	start = time.Now()
	if err := cache.Update(syn); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Errorf("Update took %v, expected < 50ms", time.Since(start))
	}

	// Verify Update
	updated, err := cache.Get(100)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("got title %q, want %q", updated.Title, "Updated title")
	}
	if updated.Status != types.StatusInProgress {
		t.Errorf("got status %v, want %v", updated.Status, types.StatusInProgress)
	}
	if len(updated.BlockedBy) != 1 || updated.BlockedBy[0] != 3 {
		t.Errorf("got blockers %v, want [3]", updated.BlockedBy)
	}

	// Test Delete
	start = time.Now()
	if err := cache.Delete(100); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Errorf("Delete took %v, expected < 50ms", time.Since(start))
	}

	// Verify Delete
	_, err = cache.Get(100)
	if err == nil {
		t.Fatal("expected error for deleted synapse")
	}
}

func TestSQLiteCache_Get(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	start := time.Now()
	syn, err := cache.Get(2)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("Get took %v, expected < 50ms", elapsed)
	}
	if syn.Title != "Implement handlers" {
		t.Errorf("got title %q, want %q", syn.Title, "Implement handlers")
	}
	if len(syn.BlockedBy) != 1 || syn.BlockedBy[0] != 1 {
		t.Errorf("got blockers %v, want [1]", syn.BlockedBy)
	}
}

func TestSQLiteCache_All(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	start := time.Now()
	all, err := cache.All()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("All failed: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("All took %v, expected < 50ms", elapsed)
	}
	if len(all) != 5 {
		t.Errorf("got %d synapses, want 5", len(all))
	}

	// Verify ordering by ID
	for i := 0; i < len(all)-1; i++ {
		if all[i].ID >= all[i+1].ID {
			t.Errorf("synapses not sorted by ID: %d >= %d", all[i].ID, all[i+1].ID)
		}
	}
}

func TestSQLiteCache_Ready(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	start := time.Now()
	ready, err := cache.Ready()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Ready failed: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("Ready took %v, expected < 50ms", elapsed)
	}

	// Expected ready tasks:
	// - ID 2: status=open, blocked by [1] where 1 is done → READY
	// Should NOT include:
	// - ID 1: status=done
	// - ID 3: status=blocked, blocked by [2] where 2 is open → NOT READY
	// - ID 4: status=open, blocked by [1,2] where 2 is open → NOT READY
	// - ID 5: status=in-progress

	if len(ready) != 1 {
		t.Errorf("got %d ready tasks, want 1", len(ready))
		for _, r := range ready {
			t.Logf("  Ready: ID=%d Title=%q Status=%s BlockedBy=%v",
				r.ID, r.Title, r.Status, r.BlockedBy)
		}
	}

	if len(ready) > 0 && ready[0].ID != 2 {
		t.Errorf("got ready task ID %d, want 2", ready[0].ID)
	}
}

func TestSQLiteCache_ByStatus(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	start := time.Now()
	openTasks, err := cache.ByStatus(types.StatusOpen)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ByStatus failed: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("ByStatus took %v, expected < 50ms", elapsed)
	}

	// Should have tasks 2 and 4 with status "open"
	if len(openTasks) != 2 {
		t.Errorf("got %d open tasks, want 2", len(openTasks))
	}

	for _, task := range openTasks {
		if task.Status != types.StatusOpen {
			t.Errorf("got status %v, want %v", task.Status, types.StatusOpen)
		}
	}
}

func TestSQLiteCache_ByAssignee(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	start := time.Now()
	backendTasks, err := cache.ByAssignee("backend")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ByAssignee failed: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("ByAssignee took %v, expected < 50ms", elapsed)
	}

	// Should have tasks 2 and 3 assigned to "backend"
	if len(backendTasks) != 2 {
		t.Errorf("got %d backend tasks, want 2", len(backendTasks))
	}

	for _, task := range backendTasks {
		if task.Assignee != "backend" {
			t.Errorf("got assignee %q, want %q", task.Assignee, "backend")
		}
	}
}

func TestSQLiteCache_Stats(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	stats, err := cache.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.SynapseCount != 5 {
		t.Errorf("got %d synapses, want 5", stats.SynapseCount)
	}

	// Total blockers: task 2 has 1, task 3 has 1, task 4 has 2 = 4 total
	if stats.BlockerCount != 4 {
		t.Errorf("got %d blockers, want 4", stats.BlockerCount)
	}

	if stats.ReadyCount != 1 {
		t.Errorf("got %d ready tasks, want 1", stats.ReadyCount)
	}

	if stats.DatabaseSizeB <= 0 {
		t.Errorf("got database size %d, want > 0", stats.DatabaseSizeB)
	}
}

func TestSQLiteCache_EmptyBlockers(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	syn := &types.Synapse{
		ID:        1,
		Title:     "No blockers",
		Status:    types.StatusOpen,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		BlockedBy: []int{}, // Empty slice
	}

	if err := cache.Insert(syn); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	retrieved, err := cache.Get(1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Ensure BlockedBy is empty slice, not nil
	if retrieved.BlockedBy == nil {
		t.Error("BlockedBy should be empty slice, not nil")
	}
	if len(retrieved.BlockedBy) != 0 {
		t.Errorf("got %d blockers, want 0", len(retrieved.BlockedBy))
	}
}

func TestSQLiteCache_NullableFields(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	syn := &types.Synapse{
		ID:        1,
		Title:     "Minimal task",
		Status:    types.StatusOpen,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		BlockedBy: []int{},
		// All optional fields left empty
	}

	if err := cache.Insert(syn); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	retrieved, err := cache.Get(1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Description != "" {
		t.Errorf("got description %q, want empty", retrieved.Description)
	}
	if retrieved.Assignee != "" {
		t.Errorf("got assignee %q, want empty", retrieved.Assignee)
	}
	if retrieved.DiscoveredFrom != "" {
		t.Errorf("got discovered_from %q, want empty", retrieved.DiscoveredFrom)
	}
	if retrieved.ParentID != 0 {
		t.Errorf("got parent_id %d, want 0", retrieved.ParentID)
	}
}

func TestSQLiteCache_Performance(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	// Create 1000 test synapses
	synapses := make([]*types.Synapse, 1000)
	now := time.Now().UTC()
	for i := 0; i < 1000; i++ {
		synapses[i] = &types.Synapse{
			ID:        i + 1,
			Title:     "Task " + string(rune(i+1)),
			Status:    types.StatusOpen,
			CreatedAt: now,
			UpdatedAt: now,
			BlockedBy: []int{},
		}
	}

	// Test Rebuild performance
	start := time.Now()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Logf("WARNING: Rebuild 1000 synapses took %v (>50ms)", elapsed)
	} else {
		t.Logf("Rebuild 1000 synapses: %v", elapsed)
	}

	// Test query performance
	queries := map[string]func() error{
		"Get": func() error {
			_, err := cache.Get(500)
			return err
		},
		"All": func() error {
			_, err := cache.All()
			return err
		},
		"Ready": func() error {
			_, err := cache.Ready()
			return err
		},
		"ByStatus": func() error {
			_, err := cache.ByStatus(types.StatusOpen)
			return err
		},
		"ByAssignee": func() error {
			_, err := cache.ByAssignee("backend")
			return err
		},
	}

	for name, fn := range queries {
		start := time.Now()
		if err := fn(); err != nil {
			t.Errorf("%s failed: %v", name, err)
		}
		elapsed := time.Since(start)
		if elapsed > 50*time.Millisecond {
			t.Errorf("%s took %v, expected < 50ms", name, elapsed)
		} else {
			t.Logf("%s: %v", name, elapsed)
		}
	}
}

func TestSQLiteCache_ConcurrentReads(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	// Launch multiple concurrent readers
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				if _, err := cache.All(); err != nil {
					t.Errorf("concurrent read failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all readers to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkSQLiteCache_Rebuild(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	cache := NewSQLiteCache(dbPath)
	if err := cache.Init(); err != nil {
		b.Fatalf("Init failed: %v", err)
	}
	defer cache.Close()

	synapses := createTestSynapses()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cache.Rebuild(synapses); err != nil {
			b.Fatalf("Rebuild failed: %v", err)
		}
	}
}

func BenchmarkSQLiteCache_Get(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	cache := NewSQLiteCache(dbPath)
	if err := cache.Init(); err != nil {
		b.Fatalf("Init failed: %v", err)
	}
	defer cache.Close()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		b.Fatalf("Rebuild failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cache.Get(2); err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func BenchmarkSQLiteCache_Ready(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	cache := NewSQLiteCache(dbPath)
	if err := cache.Init(); err != nil {
		b.Fatalf("Init failed: %v", err)
	}
	defer cache.Close()

	synapses := createTestSynapses()
	if err := cache.Rebuild(synapses); err != nil {
		b.Fatalf("Rebuild failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cache.Ready(); err != nil {
			b.Fatalf("Ready failed: %v", err)
		}
	}
}
