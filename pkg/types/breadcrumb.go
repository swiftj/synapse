// Package types defines the core data structures for Synapse.
package types

import "time"

// Breadcrumb represents a persistent key-value pair for cross-session knowledge storage.
type Breadcrumb struct {
	Key       string    `json:"key"`               // Namespaced key (e.g., "auth.method")
	Value     string    `json:"value"`             // The stored value
	TaskID    int       `json:"task_id,omitempty"` // Optional: task that created this
	CreatedAt time.Time `json:"created_at"`        // Initial creation timestamp
	UpdatedAt time.Time `json:"updated_at"`        // Last modification timestamp
}

// NewBreadcrumb creates a new Breadcrumb with the given key and value.
func NewBreadcrumb(key, value string) *Breadcrumb {
	now := time.Now().UTC()
	return &Breadcrumb{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewBreadcrumbWithTask creates a new Breadcrumb linked to a specific task.
func NewBreadcrumbWithTask(key, value string, taskID int) *Breadcrumb {
	b := NewBreadcrumb(key, value)
	b.TaskID = taskID
	return b
}

// Update modifies the value and updates the timestamp.
func (b *Breadcrumb) Update(value string) {
	b.Value = value
	b.UpdatedAt = time.Now().UTC()
}
