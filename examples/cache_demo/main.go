package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/swiftj/synapse/internal/storage"
	"github.com/swiftj/synapse/pkg/types"
)

func main() {
	// Create temp directory for demo
	tmpDir, err := os.MkdirTemp("", "synapse-demo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "cache.db")
	fmt.Printf("Demo SQLite cache at: %s\n\n", dbPath)

	// Initialize cache
	cache := storage.NewSQLiteCache(dbPath)
	if err := cache.Init(); err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer cache.Close()

	// Create sample synapses
	now := time.Now().UTC()
	synapses := []*types.Synapse{
		{
			ID:          1,
			Title:       "Design authentication system",
			Description: "Research OAuth2 and JWT patterns",
			Status:      types.StatusDone,
			Assignee:    "architect",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{},
		},
		{
			ID:          2,
			Title:       "Implement user login",
			Status:      types.StatusOpen,
			Assignee:    "backend-dev",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{1}, // Blocked by completed task
		},
		{
			ID:          3,
			Title:       "Add password reset",
			Status:      types.StatusBlocked,
			Assignee:    "backend-dev",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{2}, // Blocked by open task
		},
		{
			ID:          4,
			Title:       "Write integration tests",
			Status:      types.StatusOpen,
			Assignee:    "qa",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{1, 2}, // One done, one open
		},
		{
			ID:          5,
			Title:       "Update API documentation",
			Status:      types.StatusInProgress,
			Assignee:    "tech-writer",
			CreatedAt:   now,
			UpdatedAt:   now,
			BlockedBy:   []int{},
		},
	}

	// Rebuild cache from synapses
	fmt.Println("Building cache...")
	start := time.Now()
	if err := cache.Rebuild(synapses); err != nil {
		log.Fatalf("Rebuild failed: %v", err)
	}
	fmt.Printf("✓ Rebuilt cache in %v\n\n", time.Since(start))

	// Get statistics
	stats, err := cache.GetStats()
	if err != nil {
		log.Fatalf("GetStats failed: %v", err)
	}
	fmt.Printf("Cache Statistics:\n")
	fmt.Printf("  Total synapses: %d\n", stats.SynapseCount)
	fmt.Printf("  Total blockers: %d\n", stats.BlockerCount)
	fmt.Printf("  Ready to work:  %d\n", stats.ReadyCount)
	fmt.Printf("  Database size:  %d bytes\n\n", stats.DatabaseSizeB)

	// Query ready tasks
	fmt.Println("Tasks ready to work on:")
	start = time.Now()
	ready, err := cache.Ready()
	if err != nil {
		log.Fatalf("Ready failed: %v", err)
	}
	fmt.Printf("✓ Query completed in %v\n", time.Since(start))
	for _, syn := range ready {
		fmt.Printf("  [%d] %s (assignee: %s, blockers: %v)\n",
			syn.ID, syn.Title, syn.Assignee, syn.BlockedBy)
	}
	fmt.Println()

	// Query by status
	fmt.Println("Open tasks:")
	start = time.Now()
	openTasks, err := cache.ByStatus(types.StatusOpen)
	if err != nil {
		log.Fatalf("ByStatus failed: %v", err)
	}
	fmt.Printf("✓ Query completed in %v\n", time.Since(start))
	for _, syn := range openTasks {
		fmt.Printf("  [%d] %s\n", syn.ID, syn.Title)
	}
	fmt.Println()

	// Query by assignee
	fmt.Println("Backend developer tasks:")
	start = time.Now()
	backendTasks, err := cache.ByAssignee("backend-dev")
	if err != nil {
		log.Fatalf("ByAssignee failed: %v", err)
	}
	fmt.Printf("✓ Query completed in %v\n", time.Since(start))
	for _, syn := range backendTasks {
		fmt.Printf("  [%d] %s (status: %s)\n", syn.ID, syn.Title, syn.Status)
	}
	fmt.Println()

	// Update a task
	fmt.Println("Updating task 2 to in-progress...")
	task2, err := cache.Get(2)
	if err != nil {
		log.Fatalf("Get failed: %v", err)
	}
	task2.MarkInProgress()
	start = time.Now()
	if err := cache.Update(task2); err != nil {
		log.Fatalf("Update failed: %v", err)
	}
	fmt.Printf("✓ Updated in %v\n\n", time.Since(start))

	// Check ready tasks again
	fmt.Println("Ready tasks after update:")
	ready, err = cache.Ready()
	if err != nil {
		log.Fatalf("Ready failed: %v", err)
	}
	if len(ready) == 0 {
		fmt.Println("  (no tasks ready - task 2 is now in-progress)")
	}
	for _, syn := range ready {
		fmt.Printf("  [%d] %s\n", syn.ID, syn.Title)
	}
	fmt.Println()

	// Performance test
	fmt.Println("Performance test (100 queries):")
	operations := map[string]func() error{
		"Get(1)": func() error {
			_, err := cache.Get(1)
			return err
		},
		"All()": func() error {
			_, err := cache.All()
			return err
		},
		"Ready()": func() error {
			_, err := cache.Ready()
			return err
		},
		"ByStatus(open)": func() error {
			_, err := cache.ByStatus(types.StatusOpen)
			return err
		},
	}

	for name, fn := range operations {
		start := time.Now()
		for i := 0; i < 100; i++ {
			if err := fn(); err != nil {
				log.Fatalf("%s failed: %v", name, err)
			}
		}
		elapsed := time.Since(start)
		avg := elapsed / 100
		fmt.Printf("  %-20s %v total, %v avg\n", name+":", elapsed, avg)
	}

	fmt.Println("\nDemo completed successfully!")
}
