// Package storage provides persistence for Synapse data.
package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/swiftj/synapse/pkg/types"
)

const (
	// BreadcrumbFile is the JSONL file for breadcrumb storage.
	BreadcrumbFile = "breadcrumbs.jsonl"
)

// BreadcrumbStore manages JSONL-based persistence for Breadcrumbs.
type BreadcrumbStore struct {
	mu          sync.RWMutex
	dir         string
	breadcrumbs map[string]*types.Breadcrumb
}

// NewBreadcrumbStore creates a new breadcrumb store at the given directory.
func NewBreadcrumbStore(dir string) *BreadcrumbStore {
	return &BreadcrumbStore{
		dir:         dir,
		breadcrumbs: make(map[string]*types.Breadcrumb),
	}
}

// Load reads all breadcrumbs from the JSONL file into memory.
func (s *BreadcrumbStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.filePath()
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Empty store is valid
		}
		return fmt.Errorf("open breadcrumbs file: %w", err)
	}
	defer file.Close()

	s.breadcrumbs = make(map[string]*types.Breadcrumb)

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var b types.Breadcrumb
		if err := json.Unmarshal(line, &b); err != nil {
			return fmt.Errorf("parse line %d: %w", lineNum, err)
		}

		s.breadcrumbs[b.Key] = &b
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan breadcrumbs file: %w", err)
	}

	return nil
}

// Save writes all breadcrumbs to the JSONL file in deterministic order.
func (s *BreadcrumbStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Sort by key for deterministic Git diffs
	keys := make([]string, 0, len(s.breadcrumbs))
	for key := range s.breadcrumbs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Write to temp file then rename for atomicity
	filePath := s.filePath()
	tmpPath := filePath + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	encoder := json.NewEncoder(file)
	for _, key := range keys {
		if err := encoder.Encode(s.breadcrumbs[key]); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode breadcrumb %s: %w", key, err)
		}
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Set creates or updates a breadcrumb. Returns true if created, false if updated.
func (s *BreadcrumbStore) Set(key, value string, taskID int) (created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.breadcrumbs[key]
	if exists {
		existing.Update(value)
		if taskID > 0 {
			existing.TaskID = taskID
		}
		return false, nil
	}

	var b *types.Breadcrumb
	if taskID > 0 {
		b = types.NewBreadcrumbWithTask(key, value, taskID)
	} else {
		b = types.NewBreadcrumb(key, value)
	}
	s.breadcrumbs[key] = b
	return true, nil
}

// Get retrieves a breadcrumb by key.
func (s *BreadcrumbStore) Get(key string) (*types.Breadcrumb, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.breadcrumbs[key]
	return b, ok
}

// Delete removes a breadcrumb by key. Returns true if deleted, false if not found.
func (s *BreadcrumbStore) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.breadcrumbs[key]; !ok {
		return false
	}
	delete(s.breadcrumbs, key)
	return true
}

// List returns all breadcrumbs, optionally filtered by prefix.
func (s *BreadcrumbStore) List(prefix string) []*types.Breadcrumb {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Breadcrumb
	for _, b := range s.breadcrumbs {
		if prefix == "" || strings.HasPrefix(b.Key, prefix) {
			result = append(result, b)
		}
	}

	// Sort by key for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// ListByTask returns all breadcrumbs linked to a specific task.
func (s *BreadcrumbStore) ListByTask(taskID int) []*types.Breadcrumb {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*types.Breadcrumb
	for _, b := range s.breadcrumbs {
		if b.TaskID == taskID {
			result = append(result, b)
		}
	}

	// Sort by key for consistent output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// Count returns the total number of breadcrumbs.
func (s *BreadcrumbStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.breadcrumbs)
}

// filePath returns the full path to the breadcrumbs file.
func (s *BreadcrumbStore) filePath() string {
	return filepath.Join(s.dir, BreadcrumbFile)
}
