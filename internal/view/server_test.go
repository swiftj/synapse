package view

import (
	"strings"
	"testing"

	"github.com/swiftj/synapse/internal/storage"
	"github.com/swiftj/synapse/pkg/types"
)

func TestNewServer(t *testing.T) {
	store := storage.NewJSONLStore("/tmp/test")
	server := NewServer(store, 8080)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.port != 8080 {
		t.Errorf("expected port 8080, got %d", server.port)
	}

	if server.store != store {
		t.Error("store not set correctly")
	}
}

func TestGenerateMermaid_Empty(t *testing.T) {
	store := storage.NewJSONLStore("/tmp/test")
	server := NewServer(store, 8080)

	mermaid := server.generateMermaid()

	if !strings.Contains(mermaid, "graph TD") {
		t.Error("expected mermaid to contain 'graph TD'")
	}

	if !strings.Contains(mermaid, "No tasks yet") {
		t.Error("expected empty graph message")
	}
}

func TestGenerateMermaid_WithTasks(t *testing.T) {
	store := storage.NewJSONLStore("/tmp/test")
	server := NewServer(store, 8080)

	// Create test synapses
	syn1, _ := store.Create("Setup project")
	syn1.Status = types.StatusDone

	syn2, _ := store.Create("Implement MCP")
	syn2.Status = types.StatusInProgress
	syn2.BlockedBy = []int{1}

	syn3, _ := store.Create("Add visualization")
	syn3.Status = types.StatusBlocked
	syn3.ParentID = 1

	mermaid := server.generateMermaid()

	// Check basic structure
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("expected mermaid to contain 'graph TD'")
	}

	// Check nodes are present
	if !strings.Contains(mermaid, "#1: Setup project") {
		t.Error("expected node for task 1")
	}

	if !strings.Contains(mermaid, "#2: Implement MCP") {
		t.Error("expected node for task 2")
	}

	if !strings.Contains(mermaid, "#3: Add visualization") {
		t.Error("expected node for task 3")
	}

	// Check BlockedBy edge (blocker --> blocked)
	if !strings.Contains(mermaid, "1 --> 2") {
		t.Error("expected edge from task 1 to task 2 (BlockedBy)")
	}

	// Check ParentID edge (parent -.-> child)
	if !strings.Contains(mermaid, "1 -.-> 3") {
		t.Error("expected dotted edge from task 1 to task 3 (ParentID)")
	}

	// Check styling
	if !strings.Contains(mermaid, "style 1 fill:#90EE90") {
		t.Error("expected green fill for done task")
	}

	if !strings.Contains(mermaid, "style 2 fill:#FFFFE0") {
		t.Error("expected yellow fill for in-progress task")
	}

	if !strings.Contains(mermaid, "style 3 fill:#D3D3D3") {
		t.Error("expected gray fill for blocked task")
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"Short title", 20, "Short title"},
		{"This is a very long title that exceeds the maximum length", 20, "This is a very long ..."},
		{"Exactly twenty chars", 20, "Exactly twenty chars"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncateTitle(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateTitle(%q, %d) = %q, expected %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestEscapeForMermaid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`Task with "quotes"`, `Task with #quot;quotes#quot;`},
		{`Task [with] brackets`, `Task #91;with#93; brackets`},
		{`Task (with) parens`, `Task #40;with#41; parens`},
		{`Normal task`, `Normal task`},
		{`All "special" [chars] (here)`, `All #quot;special#quot; #91;chars#93; #40;here#41;`},
	}

	for _, tt := range tests {
		result := escapeForMermaid(tt.input)
		if result != tt.expected {
			t.Errorf("escapeForMermaid(%q) = %q, expected %q",
				tt.input, result, tt.expected)
		}
	}
}
