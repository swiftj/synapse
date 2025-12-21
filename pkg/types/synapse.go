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

// DefaultClaimTimeout is the default duration after which a claim expires.
const DefaultClaimTimeout = 30 * time.Minute

// Synapse represents an atomic memory unit / task in the system.
type Synapse struct {
	ID             int        `json:"id"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	Status         Status     `json:"status"`
	Priority       int        `json:"priority,omitempty"` // Higher number = higher priority
	BlockedBy      []int      `json:"blocked_by,omitempty"`
	ParentID       int        `json:"parent_id,omitempty"`
	Assignee       string     `json:"assignee,omitempty"`
	DiscoveredFrom string     `json:"discovered_from,omitempty"`
	Labels         []string   `json:"labels,omitempty"`
	Notes          []string   `json:"notes,omitempty"`
	ClaimedBy      string     `json:"claimed_by,omitempty"`  // Agent ID that claimed this task
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`  // When the task was claimed
	CompletedBy    string     `json:"completed_by,omitempty"` // Agent ID that completed this task
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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

// Claim attempts to claim the task for an agent. Returns true if successful.
// A claim can fail if:
// - The task is already claimed by another agent and the claim hasn't expired
// - The task is not in a claimable state (already done)
func (s *Synapse) Claim(agentID string, timeout time.Duration) bool {
	now := time.Now().UTC()

	// Can't claim completed tasks
	if s.Status == StatusDone {
		return false
	}

	// Check if already claimed by another agent with an active claim
	if s.ClaimedBy != "" && s.ClaimedBy != agentID && s.ClaimedAt != nil {
		// Check if claim has expired
		if now.Sub(*s.ClaimedAt) < timeout {
			return false // Claim still active
		}
	}

	// Claim the task
	s.ClaimedBy = agentID
	s.ClaimedAt = &now
	s.Status = StatusInProgress
	s.UpdatedAt = now
	return true
}

// ReleaseClaim releases the claim on this task.
func (s *Synapse) ReleaseClaim() {
	s.ClaimedBy = ""
	s.ClaimedAt = nil
	if s.Status == StatusInProgress {
		s.Status = StatusOpen
	}
	s.UpdatedAt = time.Now().UTC()
}

// IsClaimExpired checks if the current claim has expired.
func (s *Synapse) IsClaimExpired(timeout time.Duration) bool {
	if s.ClaimedAt == nil {
		return true
	}
	return time.Now().UTC().Sub(*s.ClaimedAt) >= timeout
}

// MarkDone transitions the synapse to done status.
func (s *Synapse) MarkDone() {
	s.Status = StatusDone
	s.UpdatedAt = time.Now().UTC()
}

// MarkDoneBy transitions the synapse to done status and records the completing agent.
func (s *Synapse) MarkDoneBy(agentID string) {
	s.Status = StatusDone
	s.CompletedBy = agentID
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

// AddNote appends a note to the task for context persistence.
func (s *Synapse) AddNote(note string) {
	s.Notes = append(s.Notes, note)
	s.UpdatedAt = time.Now().UTC()
}
