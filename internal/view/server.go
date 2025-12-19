// Package view provides web-based visualization for Synapse DAGs.
package view

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/johnswift/synapse/internal/storage"
	"github.com/johnswift/synapse/pkg/types"
)

//go:embed templates/*
var templates embed.FS

// Server provides HTTP endpoints for DAG visualization.
type Server struct {
	store *storage.JSONLStore
	port  int
}

// NewServer creates a new visualization server.
func NewServer(store *storage.JSONLStore, port int) *Server {
	return &Server{
		store: store,
		port:  port,
	}
}

// Run starts the HTTP server and blocks until shutdown.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// Serve the HTML page
	mux.HandleFunc("/", s.handleIndex)

	// API endpoints
	mux.HandleFunc("/api/synapses", s.handleSynapses)
	mux.HandleFunc("/api/ready", s.handleReady)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting visualization server on http://localhost%s", addr)

	return http.ListenAndServe(addr, mux)
}

// handleIndex serves the main visualization HTML page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := templates.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Failed to load template", http.StatusInternalServerError)
		log.Printf("Error reading template: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleSynapses returns all synapses as JSON.
func (s *Server) handleSynapses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	synapses := s.store.All()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(synapses); err != nil {
		log.Printf("Error encoding synapses: %v", err)
	}
}

// handleReady returns ready synapses as JSON.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := s.store.Ready()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ready); err != nil {
		log.Printf("Error encoding ready synapses: %v", err)
	}
}

// generateMermaid creates Mermaid graph syntax from synapses.
// This method is available for programmatic access but the visualization
// page generates Mermaid code client-side for better interactivity.
func (s *Server) generateMermaid() string {
	synapses := s.store.All()

	if len(synapses) == 0 {
		return "graph TD\n    empty[No tasks yet]"
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Create a map for quick lookup
	synMap := make(map[int]*types.Synapse)
	for _, syn := range synapses {
		synMap[syn.ID] = syn
	}

	// Generate nodes
	for _, syn := range synapses {
		title := truncateTitle(syn.Title, 40)
		label := escapeForMermaid(fmt.Sprintf("#%d: %s", syn.ID, title))
		sb.WriteString(fmt.Sprintf("    %d[\"%s\"]\n", syn.ID, label))
	}

	sb.WriteString("\n")

	// Generate edges for BlockedBy relationships
	for _, syn := range synapses {
		if len(syn.BlockedBy) > 0 {
			for _, blockerID := range syn.BlockedBy {
				if _, exists := synMap[blockerID]; exists {
					sb.WriteString(fmt.Sprintf("    %d --> %d\n", blockerID, syn.ID))
				}
			}
		}
	}

	// Generate edges for ParentID relationships (dotted style)
	for _, syn := range synapses {
		if syn.ParentID > 0 {
			if _, exists := synMap[syn.ParentID]; exists {
				sb.WriteString(fmt.Sprintf("    %d -.-> %d\n", syn.ParentID, syn.ID))
			}
		}
	}

	sb.WriteString("\n")

	// Style nodes by status
	statusColors := map[types.Status]string{
		types.StatusOpen:       "#FFFFFF",
		types.StatusInProgress: "#FFFFE0",
		types.StatusBlocked:    "#D3D3D3",
		types.StatusReview:     "#87CEEB",
		types.StatusDone:       "#90EE90",
	}

	for _, syn := range synapses {
		color := statusColors[syn.Status]
		if color == "" {
			color = "#FFFFFF"
		}
		sb.WriteString(fmt.Sprintf("    style %d fill:%s\n", syn.ID, color))
	}

	return sb.String()
}

// truncateTitle shortens a title to maxLen characters.
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen] + "..."
}

// escapeForMermaid escapes special characters for Mermaid syntax.
func escapeForMermaid(text string) string {
	replacer := strings.NewReplacer(
		`"`, `#quot;`,
		`[`, `#91;`,
		`]`, `#93;`,
		`(`, `#40;`,
		`)`, `#41;`,
	)
	return replacer.Replace(text)
}
