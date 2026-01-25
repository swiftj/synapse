package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/swiftj/synapse/internal/storage"
)

func TestListTasks_ResponseSizeLimiting(t *testing.T) {
	// Create a temporary store with tasks that have large notes
	dir := t.TempDir()
	store := storage.NewJSONLStore(dir)
	if _, err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// Create tasks with large notes to exceed the size limit
	largeNote := strings.Repeat("This is a very long note that takes up space. ", 100)
	for i := 0; i < 20; i++ {
		syn, err := store.Create("Task with large notes")
		if err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
		// Add multiple large notes
		for j := 0; j < 10; j++ {
			syn.AddNote(largeNote)
		}
		if err := store.Update(syn); err != nil {
			t.Fatalf("failed to update task: %v", err)
		}
	}

	bcStore := storage.NewBreadcrumbStore(dir)

	server := NewServer(store, bcStore)

	// Test 1: Full mode with small max_chars should trigger truncation
	t.Run("auto-truncates when exceeding max_chars", func(t *testing.T) {
		result, err := server.listTasks(map[string]any{
			"summary":   false,
			"max_chars": float64(5000), // Small limit to force truncation
		})
		if err != nil {
			t.Fatalf("listTasks failed: %v", err)
		}

		var response map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should be truncated
		if truncated, ok := response["truncated"].(bool); !ok || !truncated {
			t.Error("expected response to be truncated")
		}

		// Should have a hint
		if hint, ok := response["hint"].(string); !ok || hint == "" {
			t.Error("expected truncation hint")
		}

		// Tasks should still be present but in summary form
		tasks, ok := response["tasks"].([]any)
		if !ok || len(tasks) == 0 {
			t.Error("expected tasks in response")
		}

		// First task should have notes_count instead of full notes
		firstTask := tasks[0].(map[string]any)
		if _, hasNotes := firstTask["notes"]; hasNotes {
			t.Error("truncated response should not include full notes array")
		}
		if notesCount, ok := firstTask["notes_count"].(float64); !ok || notesCount == 0 {
			t.Error("truncated response should include notes_count")
		}
	})

	// Test 2: Summary mode should not be affected
	t.Run("summary mode unaffected", func(t *testing.T) {
		result, err := server.listTasks(map[string]any{
			"summary": true,
		})
		if err != nil {
			t.Fatalf("listTasks failed: %v", err)
		}

		var response map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should NOT be truncated (summary mode doesn't need it)
		if _, ok := response["truncated"]; ok {
			t.Error("summary mode should not have truncated flag")
		}
	})

	// Test 3: Large max_chars should not truncate
	t.Run("respects large max_chars", func(t *testing.T) {
		result, err := server.listTasks(map[string]any{
			"summary":   false,
			"limit":     float64(2), // Only 2 tasks
			"max_chars": float64(1000000),
		})
		if err != nil {
			t.Fatalf("listTasks failed: %v", err)
		}

		var response map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should NOT be truncated
		if _, ok := response["truncated"]; ok {
			t.Error("response with large max_chars and few tasks should not be truncated")
		}
	})
}

func TestListTasks_FieldsSelection(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewJSONLStore(dir)
	if _, err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	syn, _ := store.Create("Test task")
	syn.Description = "A description"
	syn.Labels = []string{"bug", "urgent"}
	syn.AddNote("A note")
	store.Update(syn)

	bcStore := storage.NewBreadcrumbStore(dir)
	server := NewServer(store, bcStore)

	result, err := server.listTasks(map[string]any{
		"fields": []any{"id", "title", "labels"},
	})
	if err != nil {
		t.Fatalf("listTasks failed: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	tasks := response["tasks"].([]any)
	task := tasks[0].(map[string]any)

	// Should have requested fields
	if _, ok := task["id"]; !ok {
		t.Error("expected id field")
	}
	if _, ok := task["title"]; !ok {
		t.Error("expected title field")
	}
	if _, ok := task["labels"]; !ok {
		t.Error("expected labels field")
	}

	// Should NOT have unrequested fields
	if _, ok := task["description"]; ok {
		t.Error("should not have description field")
	}
	if _, ok := task["notes"]; ok {
		t.Error("should not have notes field")
	}
}

func TestMaxResponseSize_Constant(t *testing.T) {
	// Verify the constant is set to a reasonable value
	if MaxResponseSize < 10000 {
		t.Error("MaxResponseSize too small, may cause unnecessary truncation")
	}
	if MaxResponseSize > 200000 {
		t.Error("MaxResponseSize too large, may cause MCP client issues")
	}
}
