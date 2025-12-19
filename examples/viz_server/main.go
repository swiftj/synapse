// Example program demonstrating the visualization server.
//
// Usage:
//   go run examples/viz_server.go
//
// Then open http://localhost:8080 in your browser.
package main

import (
	"log"

	"github.com/swiftj/synapse/internal/storage"
	"github.com/swiftj/synapse/internal/view"
	"github.com/swiftj/synapse/pkg/types"
)

func main() {
	// Initialize storage
	store := storage.NewJSONLStore(".synapse")
	if _, err := store.Init(); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	if err := store.Load(); err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	// Create sample data if empty
	if store.Count() == 0 {
		log.Println("Creating sample tasks...")

		syn1, _ := store.Create("Setup project infrastructure")
		syn1.Status = types.StatusDone
		store.Update(syn1)

		syn2, _ := store.Create("Implement core MCP protocol")
		syn2.Status = types.StatusInProgress
		syn2.BlockedBy = []int{1}
		store.Update(syn2)

		syn3, _ := store.Create("Add visualization server")
		syn3.Status = types.StatusDone
		syn3.ParentID = 1
		store.Update(syn3)

		syn4, _ := store.Create("Write documentation")
		syn4.Status = types.StatusOpen
		syn4.ParentID = 1
		syn4.BlockedBy = []int{2, 3}
		store.Update(syn4)

		syn5, _ := store.Create("Deploy to production")
		syn5.Status = types.StatusBlocked
		syn5.BlockedBy = []int{4}
		store.Update(syn5)

		if err := store.Save(); err != nil {
			log.Fatalf("Failed to save data: %v", err)
		}

		log.Println("Sample tasks created")
	}

	// Start visualization server
	server := view.NewServer(store, 8080)
	log.Fatal(server.Run())
}
