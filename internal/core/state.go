package core

import (
	"sync"
	"time"
	"encoding/json"

	"github.com/labs-alone/alone-main/internal/utils"
)

// State manages the application's runtime state
type State struct {
	mu            sync.RWMutex
	status        Status
	lastUpdated   time.Time
	connections   map[string]*Connection
	transactions  map[string]*Transaction
	cache         *Cache
	logger        *utils.Logger
}

// Status represents the current state status
type Status struct {
	IsHealthy    bool      `json:"is_healthy"`
	StartTime    time.Time `json:"start_time"`
	Environment  string    `json:"environment"`
	Version      string    `json:"version"`
	NodeCount    int       `json:"node_count"`
	ActiveUsers  int       `json:"active_users"`
}

// Connection tracks active connections
type Connection struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	StartTime time.Time `json:"start_time"`
	LastPing  time.Time `json:"last_ping"`
	Metadata  Metadata  `json:"metadata"`
}

// Transaction represents a tracked transaction
type Transaction struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Data      Metadata  `json:"data"`
}

// Metadata stores additional information
type Metadata map[string]interface{}

// Cache provides in-memory caching
type Cache struct {
	data map[string][]byte
	ttl  map[string]time.Time
	mu   sync.RWMutex
}

// NewState creates a new state instance
func NewState() (*State, error) {
	cache := &Cache{
		data: make(map[string][]byte),
		ttl:  make(map[string]time.Time),
	}

	return &State{
		status: Status{
			IsHealthy:   true,
			StartTime:   time.Now(),
			Environment: utils.GetEnvironment(),
			Version:     "0.1.0",
		},
		connections:  make(map[string]*Connection),
		transactions: make(map[string]*Transaction),
		cache:       cache,
		logger:      utils.NewLogger(),
		lastUpdated: time.Now(),
	}, nil
}

// GetStatus returns the current state status
func (s *State) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// UpdateStatus updates the state status
func (s *State) UpdateStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.lastUpdated = time.Now()
}

// AddConnection adds a new connection
func (s *State) AddConnection(conn *Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[conn.ID] = conn
	s.status.ActiveUsers++
	s.lastUpdated = time.Now()
}

// RemoveConnection removes a connection
func (s *State) RemoveConnection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.connections[id]; exists {
		delete(s.connections, id)
		s.status.ActiveUsers--
		s.lastUpdated = time.Now()
	}
}

// TrackTransaction adds a new transaction
func (s *State) TrackTransaction(tx *Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.ID] = tx
	s.lastUpdated = time.Now()
}

// UpdateTransaction updates an existing transaction
func (s *State) UpdateTransaction(id string, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tx, exists := s.transactions[id]; exists {
		tx.Status = status
		tx.EndTime = time.Now()
		s.lastUpdated = time.Now()
	}
}

// GetTransaction retrieves a transaction by ID
func (s *State) GetTransaction(id string) (*Transaction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, exists := s.transactions[id]
	return tx, exists
}

// CacheSet stores data in cache
func (s *State) CacheSet(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	
	s.cache.data[key] = data
	s.cache.ttl[key] = time.Now().Add(ttl)
	return nil
}

// CacheGet retrieves data from cache
func (s *State) CacheGet(key string, value interface{}) (bool, error) {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	data, exists := s.cache.data[key]
	if !exists {
		return false, nil
	}

	if ttl, ok := s.cache.ttl[key]; ok && time.Now().After(ttl) {
		return false, nil
	}

	return true, json.Unmarshal(data, value)
}

// Cleanup performs state cleanup
func (s *State) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cleanup expired cache entries
	s.cache.mu.Lock()
	now := time.Now()
	for key, ttl := range s.cache.ttl {
		if now.After(ttl) {
			delete(s.cache.data, key)
			delete(s.cache.ttl, key)
		}
	}
	s.cache.mu.Unlock()

	// Cleanup stale connections
	for id, conn := range s.connections {
		if time.Since(conn.LastPing) > 5*time.Minute {
			delete(s.connections, id)
			s.status.ActiveUsers--
		}
	}

	// Cleanup old transactions
	for id, tx := range s.transactions {
		if time.Since(tx.EndTime) > 24*time.Hour {
			delete(s.transactions, id)
		}
	}

	s.lastUpdated = time.Now()
}

// Export returns a JSON representation of the state
func (s *State) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return json.Marshal(struct {
		Status       Status                  `json:"status"`
		Connections  map[string]*Connection  `json:"connections"`
		Transactions map[string]*Transaction `json:"transactions"`
		LastUpdated  time.Time              `json:"last_updated"`
	}{
		Status:       s.status,
		Connections:  s.connections,
		Transactions: s.transactions,
		LastUpdated:  s.lastUpdated,
	})
}