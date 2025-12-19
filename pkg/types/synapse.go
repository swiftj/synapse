// Package types defines the core data structures for Synapse.
package types

import "time"

// Status represents the lifecycle state of a Synapse task.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in-progress"
	StatusBlocked    Status = "blocked"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusOpen, StatusInProgress, StatusBlocked, StatusReview, StatusDone}
}

// IsValid checks if the status is a recognized value.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusBlocked, StatusReview, StatusDone:
		return true
	}
	return false
}

// Synapse represents an atomic memory unit / task in the system.
type Synapse struct {
	ID             int       `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	Status         Status    `json:"status"`
	BlockedBy      []int     `json:"blocked_by,omitempty"`
	ParentID       int       `json:"parent_id,omitempty"`
	Assignee       string    `json:"assignee,omitempty"`
	DiscoveredFrom string    `json:"discovered_from,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewSynapse creates a new Synapse with the given title and default values.
func NewSynapse(id int, title string) *Synapse {
	now := time.Now().UTC()
	return &Synapse{
		ID:        id,
		Title:     title,
		Status:    StatusOpen,
		BlockedBy: []int{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsReady returns true if this synapse can be worked on.
// A task is ready when:
// - Status is "open" or "blocked" (blocked tasks become ready when blockers complete)
// - Status is NOT in-progress, review, or done
// - All blockers are done
// The caller must provide a function to check if a blocker ID is done.
func (s *Synapse) IsReady(isBlockerDone func(id int) bool) bool {
	// Already claimed or completed
	if s.Status == StatusInProgress || s.Status == StatusReview || s.Status == StatusDone {
		return false
	}
	// Check all blockers are done
	for _, blockerID := range s.BlockedBy {
		if !isBlockerDone(blockerID) {
			return false
		}
	}
	return true
}

// MarkInProgress transitions the synapse to in-progress status.
func (s *Synapse) MarkInProgress() {
	s.Status = StatusInProgress
	s.UpdatedAt = time.Now().UTC()
}

// MarkDone transitions the synapse to done status.
func (s *Synapse) MarkDone() {
	s.Status = StatusDone
	s.UpdatedAt = time.Now().UTC()
}

// MarkBlocked transitions the synapse to blocked status.
func (s *Synapse) MarkBlocked() {
	s.Status = StatusBlocked
	s.UpdatedAt = time.Now().UTC()
}

// AddBlocker adds a blocking dependency.
func (s *Synapse) AddBlocker(blockerID int) {
	for _, id := range s.BlockedBy {
		if id == blockerID {
			return // already exists
		}
	}
	s.BlockedBy = append(s.BlockedBy, blockerID)
	s.UpdatedAt = time.Now().UTC()
}

// RemoveBlocker removes a blocking dependency.
func (s *Synapse) RemoveBlocker(blockerID int) {
	for i, id := range s.BlockedBy {
		if id == blockerID {
			s.BlockedBy = append(s.BlockedBy[:i], s.BlockedBy[i+1:]...)
			s.UpdatedAt = time.Now().UTC()
			return
		}
	}
}
