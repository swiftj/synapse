// Package storage provides persistence for Synapse data.
package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/johnswift/synapse/pkg/types"
)

const (
	// DefaultDir is the default directory for synapse data.
	DefaultDir = ".synapse"
	// MemoryFile is the JSONL source of truth.
	MemoryFile = "memory.jsonl"
)

// JSONLStore manages JSONL-based persistence for Synapses.
type JSONLStore struct {
	mu       sync.RWMutex
	dir      string
	synapses map[int]*types.Synapse
	nextID   int
}

// NewJSONLStore creates a new JSONL store at the given directory.
func NewJSONLStore(dir string) *JSONLStore {
	return &JSONLStore{
		dir:      dir,
		synapses: make(map[int]*types.Synapse),
		nextID:   1,
	}
}

// Init creates the storage directory if it doesn't exist.
func (s *JSONLStore) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}

	// Create empty memory file if it doesn't exist
	memPath := s.memoryPath()
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		f, err := os.Create(memPath)
		if err != nil {
			return fmt.Errorf("create memory file: %w", err)
		}
		f.Close()
	}

	return nil
}

// Load reads all synapses from the JSONL file into memory.
func (s *JSONLStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	memPath := s.memoryPath()
	file, err := os.Open(memPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Empty store is valid
		}
		return fmt.Errorf("open memory file: %w", err)
	}
	defer file.Close()

	s.synapses = make(map[int]*types.Synapse)
	s.nextID = 1

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var syn types.Synapse
		if err := json.Unmarshal(line, &syn); err != nil {
			return fmt.Errorf("parse line %d: %w", lineNum, err)
		}

		s.synapses[syn.ID] = &syn
		if syn.ID >= s.nextID {
			s.nextID = syn.ID + 1
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan memory file: %w", err)
	}

	return nil
}

// Save writes all synapses to the JSONL file in deterministic order.
func (s *JSONLStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Sort by ID for deterministic Git diffs
	ids := make([]int, 0, len(s.synapses))
	for id := range s.synapses {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	// Write to temp file then rename for atomicity
	memPath := s.memoryPath()
	tmpPath := memPath + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	encoder := json.NewEncoder(file)
	for _, id := range ids {
		if err := encoder.Encode(s.synapses[id]); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode synapse %d: %w", id, err)
		}
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, memPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Create adds a new synapse and returns its ID.
func (s *JSONLStore) Create(title string) (*types.Synapse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	syn := types.NewSynapse(s.nextID, title)
	s.synapses[syn.ID] = syn
	s.nextID++

	return syn, nil
}

// Get retrieves a synapse by ID.
func (s *JSONLStore) Get(id int) (*types.Synapse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	syn, ok := s.synapses[id]
	if !ok {
		return nil, fmt.Errorf("synapse %d not found", id)
	}
	return syn, nil
}

// Update modifies an existing synapse.
func (s *JSONLStore) Update(syn *types.Synapse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.synapses[syn.ID]; !ok {
		return fmt.Errorf("synapse %d not found", syn.ID)
	}
	s.synapses[syn.ID] = syn
	return nil
}

// Delete removes a synapse by ID.
func (s *JSONLStore) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.synapses[id]; !ok {
		return fmt.Errorf("synapse %d not found", id)
	}
	delete(s.synapses, id)
	return nil
}

// All returns all synapses sorted by ID.
func (s *JSONLStore) All() []*types.Synapse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]int, 0, len(s.synapses))
	for id := range s.synapses {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	result := make([]*types.Synapse, len(ids))
	for i, id := range ids {
		result[i] = s.synapses[id]
	}
	return result
}

// Ready returns all synapses that are ready to be worked on.
func (s *JSONLStore) Ready() []*types.Synapse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	isDone := func(id int) bool {
		syn, ok := s.synapses[id]
		return ok && syn.Status == types.StatusDone
	}

	var ready []*types.Synapse
	for _, syn := range s.synapses {
		if syn.IsReady(isDone) {
			ready = append(ready, syn)
		}
	}

	// Sort by ID for deterministic ordering
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].ID < ready[j].ID
	})

	return ready
}

// ByStatus returns all synapses with the given status.
func (s *JSONLStore) ByStatus(status types.Status) []*types.Synapse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Synapse
	for _, syn := range s.synapses {
		if syn.Status == status {
			result = append(result, syn)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// ByAssignee returns all synapses assigned to the given role.
func (s *JSONLStore) ByAssignee(assignee string) []*types.Synapse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Synapse
	for _, syn := range s.synapses {
		if syn.Assignee == assignee {
			result = append(result, syn)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// Count returns the total number of synapses.
func (s *JSONLStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.synapses)
}

// memoryPath returns the full path to the memory file.
func (s *JSONLStore) memoryPath() string {
	return filepath.Join(s.dir, MemoryFile)
}

// Dir returns the storage directory path.
func (s *JSONLStore) Dir() string {
	return s.dir
}
