package lilith

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alone-labs/pkg/logger"
)

// Agent represents the Lilith AI agent
type Agent struct {
	ID        string
	Name      string
	Version   string
	ctx       context.Context
	cancel    context.CancelFunc
	config    *Config
	processor *Processor
	state     *State
	logger    *logger.Logger
	mu        sync.RWMutex
	isRunning bool
	startTime time.Time
}

// NewAgent creates and initializes a new Lilith agent
func NewAgent(config *Config, logger *logger.Logger) (*Agent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		ID:        generateAgentID(),
		Name:      config.Name,
		Version:   config.Version,
		ctx:       ctx,
		cancel:    cancel,
		config:    config,
		processor: NewProcessor(),
		state:     NewState(),
		logger:    logger,
		isRunning: false,
	}

	// Register default task handlers
	agent.registerDefaultHandlers()

	return agent, nil
}

// Start initializes and runs the Lilith agent
func (a *Agent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return ErrAgentAlreadyRunning
	}

	a.logger.Info("Starting Lilith agent", "id", a.ID, "version", a.Version)

	a.isRunning = true
	a.startTime = time.Now()
	a.state.UpdateStatus(StatusWorking)

	// Start main processing loop
	go a.run()

	// Start memory cleanup routine
	go a.memoryCleanup()

	return nil
}

// Stop gracefully shuts down the Lilith agent
func (a *Agent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return ErrAgentNotRunning
	}

	a.logger.Info("Stopping Lilith agent", "id", a.ID)

	a.state.UpdateStatus(StatusStopped)
	a.cancel()
	a.isRunning = false

	return nil
}

// AddTask adds a new task to the agent's processing queue
func (a *Agent) AddTask(task Task) error {
	if !a.isRunning {
		return ErrAgentNotRunning
	}

	a.processor.AddTask(task)
	a.logger.Debug("Task added to queue", "taskID", task.ID, "type", task.Type)
	return nil
}

// GetStatus returns the current status of the agent
func (a *Agent) GetStatus() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return AgentStatus{
		ID:             a.ID,
		Status:         a.state.Status,
		TasksProcessed: a.state.TasksProcessed,
		Uptime:        time.Since(a.startTime),
		LastActivity:   a.state.LastActivity,
		LastError:      a.state.LastError,
	}
}

// Internal methods

func (a *Agent) run() {
	ticker := time.NewTicker(a.config.ProcessInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.logger.Info("Agent processing loop stopped", "id", a.ID)
			return
		case <-ticker.C:
			if err := a.processor.Process(a.ctx, a.state); err != nil {
				a.state.LastError = err
				a.logger.Error("Processing error", "error", err)
			}
		}
	}
}

func (a *Agent) memoryCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.state.CleanupExpiredMemory()
		}
	}
}

func (a *Agent) registerDefaultHandlers() {
	// Register system task handlers
	a.processor.RegisterHandler("system.health", a.handleHealthCheck)
	a.processor.RegisterHandler("system.reset", a.handleReset)
}

// Default handlers

func (a *Agent) handleHealthCheck(ctx context.Context, state *State, task Task) error {
	// Implement health check logic
	return nil
}

func (a *Agent) handleReset(ctx context.Context, state *State, task Task) error {
	// Implement reset logic
	return nil
}

// AgentStatus represents the current status of the agent
type AgentStatus struct {
	ID             string
	Status         Status
	TasksProcessed uint64
	Uptime         time.Duration
	LastActivity   time.Time
	LastError      error
}

// Helper functions

func generateAgentID() string {
	return fmt.Sprintf("lilith-%d", time.Now().UnixNano())
}