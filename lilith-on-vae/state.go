package lilith

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/alone-labs/pkg/logger"
)

// State manages the agent's current state and memory systems
type State struct {
	mu sync.RWMutex

	// Core state
	Status      Status
	LastError   error
	LastUpdated time.Time

	// Memory systems
	ShortTerm  *MemoryStore
	LongTerm   *MemoryStore
	Volatile   *MemoryStore

	// Metrics
	TasksProcessed uint64
	LastActivity   time.Time

	logger *logger.Logger
}

// MemoryStore represents a specific type of memory storage
type MemoryStore struct {
	mu         sync.RWMutex
	data       map[string]MemoryItem
	maxSize    int
	persistent bool
}

// MemoryItem represents a single memory entry
type MemoryItem struct {
	Value      interface{} `json:"value"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	AccessCount int       `json:"access_count"`
	LastAccess time.Time  `json:"last_access"`
	Priority   int       `json:"priority"`
}

// NewState creates a new state instance
func NewState(config *Config, logger *logger.Logger) *State {
	return &State{
		Status:      StatusIdle,
		LastUpdated: time.Now(),
		ShortTerm: NewMemoryStore(
			config.MaxShortTermMemory,
			false,
		),
		LongTerm: NewMemoryStore(
			config.MaxLongTermMemory,
			true,
		),
		Volatile: NewMemoryStore(
			1000, // Small size for temporary data
			false,
		),
		logger: logger,
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(maxSize int, persistent bool) *MemoryStore {
	return &MemoryStore{
		data:       make(map[string]MemoryItem),
		maxSize:    maxSize,
		persistent: persistent,
	}
}

// Memory Operations

// Remember stores a value in the appropriate memory store
func (s *State) Remember(key string, value interface{}, memoryType MemoryType, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var store *MemoryStore
	switch memoryType {
	case MemoryTypeShortTerm:
		store = s.ShortTerm
	case MemoryTypeLongTerm:
		store = s.LongTerm
	case MemoryTypeVolatile:
		store = s.Volatile
	default:
		return ErrInvalidMemoryType
	}

	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	item := MemoryItem{
		Value:      value,
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
		AccessCount: 0,
		LastAccess: time.Now(),
		Priority:   1,
	}

	return store.Set(key, item)
}

// Recall retrieves a value from memory
func (s *State) Recall(key string, memoryType MemoryType) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var store *MemoryStore
	switch memoryType {
	case MemoryTypeShortTerm:
		store = s.ShortTerm
	case MemoryTypeLongTerm:
		store = s.LongTerm
	case MemoryTypeVolatile:
		store = s.Volatile
	default:
		return nil, ErrInvalidMemoryType
	}

	return store.Get(key)
}

// Forget removes a value from memory
func (s *State) Forget(key string, memoryType MemoryType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var store *MemoryStore
	switch memoryType {
	case MemoryTypeShortTerm:
		store = s.ShortTerm
	case MemoryTypeLongTerm:
		store = s.LongTerm
	case MemoryTypeVolatile:
		store = s.Volatile
	default:
		return ErrInvalidMemoryType
	}

	return store.Delete(key)
}

// MemoryStore Operations

func (m *MemoryStore) Set(key string, item MemoryItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.data) >= m.maxSize {
		m.cleanup()
	}

	m.data[key] = item
	return nil
}

func (m *MemoryStore) Get(key string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.data[key]
	if !exists {
		return nil, ErrMemoryNotFound
	}

	if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
		delete(m.data, key)
		return nil, ErrMemoryExpired
	}

	// Update access metrics
	item.AccessCount++
	item.LastAccess = time.Now()
	m.data[key] = item

	return item.Value, nil
}

func (m *MemoryStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; !exists {
		return ErrMemoryNotFound
	}

	delete(m.data, key)
	return nil
}

// Maintenance Operations

func (m *MemoryStore) cleanup() {
	// Remove expired items
	now := time.Now()
	for key, item := range m.data {
		if item.ExpiresAt != nil && now.After(*item.ExpiresAt) {
			delete(m.data, key)
		}
	}

	// If still over capacity, remove least accessed items
	if len(m.data) >= m.maxSize {
		items := make([]struct {
			key   string
			score float64
		}, 0, len(m.data))

		for key, item := range m.data {
			score := float64(item.Priority) * float64(item.AccessCount) / time.Since(item.LastAccess).Seconds()
			items = append(items, struct {
				key   string
				score float64
			}{key, score})
		}

		// Sort by score ascending (lowest first)
		sort.Slice(items, func(i, j int) bool {
			return items[i].score < items[j].score
		})

		// Remove lowest scoring items until under capacity
		for i := 0; i < len(items) && len(m.data) >= m.maxSize; i++ {
			delete(m.data, items[i].key)
		}
	}
}

// State Management

func (s *State) UpdateStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = status
	s.LastUpdated = time.Now()
	s.LastActivity = time.Now()
}

// Serialization

func (s *State) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type StateSnapshot struct {
		Status         Status    `json:"status"`
		LastUpdated    time.Time `json:"last_updated"`
		TasksProcessed uint64    `json:"tasks_processed"`
		LastActivity   time.Time `json:"last_activity"`
	}

	snapshot := StateSnapshot{
		Status:         s.Status,
		LastUpdated:    s.LastUpdated,
		TasksProcessed: s.TasksProcessed,
		LastActivity:   s.LastActivity,
	}

	return json.Marshal(snapshot)
}

// Types and Constants

type MemoryType int

const (
	MemoryTypeShortTerm MemoryType = iota
	MemoryTypeLongTerm
	MemoryTypeVolatile
)

type Status string

const (
	StatusIdle     Status = "idle"
	StatusWorking  Status = "working"
	StatusError    Status = "error"
	StatusStopped  Status = "stopped"
)