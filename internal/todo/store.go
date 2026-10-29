package todo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Item represents a single daily todo task.
type Item struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"createdAt"`
}

// Store manages daily tasks with rollover, archiving, and atomic JSON persistence.
type Store struct {
	mu        sync.Mutex
	path      string
	carryOver bool

	Items    []Item            `json:"items"`
	Archive  map[string][]Item `json:"archive,omitempty"`
	LastDate string            `json:"lastDate"`
}

// NewStore loads an existing todo store or initializes a new one.
func NewStore(path string, carryOver bool) (*Store, error) {
	s := &Store{
		path:      path,
		carryOver: carryOver,
		Items:     make([]Item, 0),
		Archive:   make(map[string][]Item),
		LastDate:  time.Now().Format("2006-01-02"),
	}

	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}

	s.checkDayRollover()
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	if s.Archive == nil {
		s.Archive = make(map[string][]Item)
	}
	if s.Items == nil {
		s.Items = make([]Item, 0)
	}
	return nil
}

// Save writes current state atomically to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := s.path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.path)
}

// checkDayRollover archives completed tasks from previous days and carries over pending ones.
func (s *Store) checkDayRollover() {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if s.LastDate == "" || s.LastDate == today {
		s.LastDate = today
		return
	}

	completed := make([]Item, 0)
	pending := make([]Item, 0)

	for _, item := range s.Items {
		if item.Done {
			completed = append(completed, item)
		} else {
			pending = append(pending, item)
		}
	}

	if len(completed) > 0 {
		s.Archive[s.LastDate] = completed
	}

	if s.carryOver {
		s.Items = pending
	} else {
		if len(pending) > 0 {
			s.Archive[s.LastDate+"_incomplete"] = pending
		}
		s.Items = make([]Item, 0)
	}

	s.LastDate = today
	_ = s.saveLocked()
}

// Add creates a new task for today.
func (s *Store) Add(text string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := Item{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Text:      text,
		Done:      false,
		CreatedAt: time.Now(),
	}

	s.Items = append(s.Items, item)
	_ = s.saveLocked()
	return item
}

// Toggle flips the completed state of a task by ID.
func (s *Store) Toggle(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Items {
		if s.Items[i].ID == id {
			s.Items[i].Done = !s.Items[i].Done
			_ = s.saveLocked()
			return true
		}
	}
	return false
}

// Delete removes a task by ID.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.Items {
		if s.Items[i].ID == id {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			_ = s.saveLocked()
			return true
		}
	}
	return false
}

// IncompleteCount returns the number of active, uncompleted tasks.
func (s *Store) IncompleteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.Items {
		if !item.Done {
			count++
		}
	}
	return count
}

// GetItems returns a snapshot of current tasks.
func (s *Store) GetItems() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := make([]Item, len(s.Items))
	copy(res, s.Items)
	return res
}
