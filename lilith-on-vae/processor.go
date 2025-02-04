package lilith

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/alone-labs/pkg/logger"
)

// Processor handles task processing and execution for the Lilith agent
type Processor struct {
	tasks     []Task
	mu        sync.RWMutex
	handlers  map[string]TaskHandler
	logger    *logger.Logger
	semaphore chan struct{} // For limiting concurrent tasks
}

// Task represents a unit of work for the agent to process
type Task struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Priority  int                    `json:"priority"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time             `json:"created_at"`
	StartedAt *time.Time            `json:"started_at,omitempty"`
	Deadline  *time.Time            `json:"deadline,omitempty"`
	Attempts  int                   `json:"attempts"`
}

// TaskHandler defines the function signature for task handlers
type TaskHandler func(context.Context, *State, Task) error

// TaskResult represents the outcome of task processing
type TaskResult struct {
	TaskID    string
	Success   bool
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

// NewProcessor creates a new task processor
func NewProcessor(config *Config, logger *logger.Logger) *Processor {
	return &Processor{
		tasks:     make([]Task, 0),
		handlers:  make(map[string]TaskHandler),
		logger:    logger,
		semaphore: make(chan struct{}, config.MaxConcurrentTasks),
	}
}

// AddTask adds a new task to the processing queue
func (p *Processor) AddTask(task Task) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	p.tasks = append(p.tasks, task)
	p.sortTasks()

	p.logger.Debug("Task added to queue", 
		"taskID", task.ID,
		"type", task.Type,
		"priority", task.Priority,
	)

	return nil
}

// Process handles the main task processing loop
func (p *Processor) Process(ctx context.Context, state *State) error {
	p.mu.Lock()
	if len(p.tasks) == 0 {
		p.mu.Unlock()
		return nil
	}

	// Get next task
	task := p.tasks[0]
	p.tasks = p.tasks[1:]
	p.mu.Unlock()

	// Check if task has expired
	if task.Deadline != nil && time.Now().After(*task.Deadline) {
		p.logger.Warn("Task expired", "taskID", task.ID)
		return fmt.Errorf("task expired: %s", task.ID)
	}

	// Acquire semaphore
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Process task
	return p.executeTask(ctx, state, task)
}

// RegisterHandler adds a new task handler
func (p *Processor) RegisterHandler(taskType string, handler TaskHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[taskType] = handler
	p.logger.Debug("Handler registered", "taskType", taskType)
}

// Internal methods

func (p *Processor) executeTask(ctx context.Context, state *State, task Task) error {
	handler, exists := p.handlers[task.Type]
	if !exists {
		return fmt.Errorf("%w: %s", ErrUnknownTaskType, task.Type)
	}

	startTime := time.Now()
	task.StartedAt = &startTime
	task.Attempts++

	p.logger.Debug("Executing task", 
		"taskID", task.ID,
		"type", task.Type,
		"attempt", task.Attempts,
	)

	// Create task context with timeout
	taskCtx, cancel := context.WithTimeout(ctx, p.getTaskTimeout(task))
	defer cancel()

	// Execute handler
	err := handler(taskCtx, state, task)

	result := TaskResult{
		TaskID:    task.ID,
		Success:   err == nil,
		Error:     err,
		StartTime: startTime,
		EndTime:   time.Now(),
	}

	// Handle result
	p.handleTaskResult(result)

	return err
}

func (p *Processor) handleTaskResult(result TaskResult) {
	if result.Success {
		p.logger.Debug("Task completed successfully",
			"taskID", result.TaskID,
			"duration", result.EndTime.Sub(result.StartTime),
		)
	} else {
		p.logger.Error("Task failed",
			"taskID", result.TaskID,
			"error", result.Error,
			"duration", result.EndTime.Sub(result.StartTime),
		)
	}
}

func (p *Processor) sortTasks() {
	sort.SliceStable(p.tasks, func(i, j int) bool {
		// Higher priority first, then earlier creation time
		if p.tasks[i].Priority != p.tasks[j].Priority {
			return p.tasks[i].Priority > p.tasks[j].Priority
		}
		return p.tasks[i].CreatedAt.Before(p.tasks[j].CreatedAt)
	})
}

func (p *Processor) getTaskTimeout(task Task) time.Duration {
	if task.Deadline != nil {
		return time.Until(*task.Deadline)
	}
	return DefaultTaskTimeout
}

// GetQueueLength returns the current number of tasks in the queue
func (p *Processor) GetQueueLength() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.tasks)
}

// GetQueueStatus returns detailed queue statistics
func (p *Processor) GetQueueStatus() QueueStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := QueueStatus{
		TotalTasks:     len(p.tasks),
		PriorityLevels: make(map[int]int),
		TaskTypes:      make(map[string]int),
	}

	for _, task := range p.tasks {
		status.PriorityLevels[task.Priority]++
		status.TaskTypes[task.Type]++
	}

	return status
}

// QueueStatus represents the current state of the task queue
type QueueStatus struct {
	TotalTasks     int
	PriorityLevels map[int]int
	TaskTypes      map[string]int
}