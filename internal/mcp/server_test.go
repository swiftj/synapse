package mcp

import (
	"encoding/json"
	"fmt"
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

func TestStringTypedParameters(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewJSONLStore(dir)
	if _, err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// Create a task to operate on
	syn, _ := store.Create("Test task")
	store.Save()
	taskID := syn.ID

	bcStore := storage.NewBreadcrumbStore(dir)
	server := NewServer(store, bcStore)

	// Test: claim_task with string ID (reproduces the reported bug)
	t.Run("claim_task accepts string id", func(t *testing.T) {
		result, err := server.claimTask(map[string]any{
			"id":       fmt.Sprintf("%d", taskID), // string "1" instead of float64(1)
			"agent_id": "claude",
		})
		if err != nil {
			t.Fatalf("claim_task with string id failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("claim_task returned error: %s", result.Content[0].Text)
		}
	})

	// Test: get_task with string ID
	t.Run("get_task accepts string id", func(t *testing.T) {
		result, err := server.getTask(map[string]any{
			"id": fmt.Sprintf("%d", taskID),
		})
		if err != nil {
			t.Fatalf("get_task with string id failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("get_task returned error: %s", result.Content[0].Text)
		}
	})

	// Test: complete_task with string ID
	t.Run("complete_task accepts string id", func(t *testing.T) {
		result, err := server.completeTask(map[string]any{
			"id": fmt.Sprintf("%d", taskID),
		})
		if err != nil {
			t.Fatalf("complete_task with string id failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("complete_task returned error: %s", result.Content[0].Text)
		}
	})

	// Test: list_tasks with string limit
	t.Run("list_tasks accepts string limit", func(t *testing.T) {
		result, err := server.listTasks(map[string]any{
			"limit": "5",
		})
		if err != nil {
			t.Fatalf("list_tasks with string limit failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("list_tasks returned error: %s", result.Content[0].Text)
		}
	})

	// Test: invalid string should give clear error
	t.Run("non-numeric string gives clear error", func(t *testing.T) {
		_, err := server.getTask(map[string]any{
			"id": "not-a-number",
		})
		if err == nil {
			t.Fatal("expected error for non-numeric string id")
		}
		if !strings.Contains(err.Error(), "must be a number") {
			t.Errorf("expected clear error message, got: %s", err.Error())
		}
	})

	// Test: missing id gives clear error
	t.Run("missing id gives clear error", func(t *testing.T) {
		_, err := server.getTask(map[string]any{})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
		if !strings.Contains(err.Error(), "is required") {
			t.Errorf("expected 'required' error message, got: %s", err.Error())
		}
	})

	// Test: task_id accepted as alias for id (LLM parameter name variation)
	t.Run("add_note accepts task_id as alias for id", func(t *testing.T) {
		// Create a fresh task for this test
		syn2, _ := store.Create("Note test task")
		store.Save()

		result, err := server.addNote(map[string]any{
			"task_id": float64(syn2.ID),
			"note":    "test note via task_id alias",
		})
		if err != nil {
			t.Fatalf("add_note with task_id alias failed: %v", err)
		}
		if result.IsError {
			t.Fatalf("add_note returned error: %s", result.Content[0].Text)
		}
	})
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    float64
		wantOK  bool
	}{
		{"float64", float64(42), 42, true},
		{"string number", "42", 42, true},
		{"string float", "3.14", 3.14, true},
		{"int", int(7), 7, true},
		{"int64", int64(99), 99, true},
		{"empty string", "", 0, false},
		{"non-numeric string", "abc", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
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
