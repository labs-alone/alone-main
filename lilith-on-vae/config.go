package lilith

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all configuration settings for the Lilith agent
type Config struct {
	// Core Settings
	Name            string        `json:"name"`
	Version         string        `json:"version"`
	ProcessInterval time.Duration `json:"process_interval"`
	Environment     string        `json:"environment"`

	// Memory Settings
	MaxShortTermMemory int           `json:"max_short_term_memory"`
	MaxLongTermMemory  int           `json:"max_long_term_memory"`
	MemoryTTL         time.Duration  `json:"memory_ttl"`
	MemoryPersistPath string         `json:"memory_persist_path"`
	CleanupInterval   time.Duration  `json:"cleanup_interval"`

	// Processing Settings
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
	TaskTimeout       time.Duration  `json:"task_timeout"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration  `json:"retry_delay"`
	TaskQueueSize     int           `json:"task_queue_size"`

	// Security Settings
	EnableEncryption bool   `json:"enable_encryption"`
	EncryptionKey   string `json:"encryption_key,omitempty"`
	AllowedOrigins  []string `json:"allowed_origins"`

	// Monitoring Settings
	EnableMetrics    bool          `json:"enable_metrics"`
	MetricsInterval time.Duration `json:"metrics_interval"`
	EnableTracing    bool          `json:"enable_tracing"`
	TraceSampleRate  float64       `json:"trace_sample_rate"`

	// Logging Settings
	LogLevel        string `json:"log_level"`
	LogFormat       string `json:"log_format"`
	LogPath         string `json:"log_path"`
	EnableDebug     bool   `json:"enable_debug"`

	// Advanced Settings
	CustomParameters map[string]interface{} `json:"custom_parameters"`
}

// Default configuration values
const (
	DefaultName             = "lilith"
	DefaultVersion          = "1.0.0"
	DefaultProcessInterval  = 100 * time.Millisecond
	DefaultEnvironment      = "development"

	DefaultMaxShortTermMemory = 10000
	DefaultMaxLongTermMemory  = 100000
	DefaultMemoryTTL         = 24 * time.Hour
	DefaultCleanupInterval   = 5 * time.Minute

	DefaultMaxConcurrentTasks = 10
	DefaultTaskTimeout       = 30 * time.Second
	DefaultRetryAttempts     = 3
	DefaultRetryDelay        = 1 * time.Second
	DefaultTaskQueueSize     = 1000

	DefaultMetricsInterval = 1 * time.Minute
	DefaultTraceSampleRate = 0.1

	DefaultLogLevel  = "info"
	DefaultLogFormat = "json"
)

// NewDefaultConfig creates a new configuration with default values
func NewDefaultConfig() *Config {
	return &Config{
		// Core Settings
		Name:            DefaultName,
		Version:         DefaultVersion,
		ProcessInterval: DefaultProcessInterval,
		Environment:     DefaultEnvironment,

		// Memory Settings
		MaxShortTermMemory: DefaultMaxShortTermMemory,
		MaxLongTermMemory:  DefaultMaxLongTermMemory,
		MemoryTTL:         DefaultMemoryTTL,
		CleanupInterval:   DefaultCleanupInterval,

		// Processing Settings
		MaxConcurrentTasks: DefaultMaxConcurrentTasks,
		TaskTimeout:       DefaultTaskTimeout,
		RetryAttempts:     DefaultRetryAttempts,
		RetryDelay:        DefaultRetryDelay,
		TaskQueueSize:     DefaultTaskQueueSize,

		// Security Settings
		EnableEncryption: false,
		AllowedOrigins:  []string{"*"},

		// Monitoring Settings
		EnableMetrics:    true,
		MetricsInterval: DefaultMetricsInterval,
		EnableTracing:    false,
		TraceSampleRate:  DefaultTraceSampleRate,

		// Logging Settings
		LogLevel:    DefaultLogLevel,
		LogFormat:   DefaultLogFormat,
		EnableDebug: false,

		// Advanced Settings
		CustomParameters: make(map[string]interface{}),
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	config := NewDefaultConfig()
	if err := json.Unmarshal(file, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if c.ProcessInterval < 10*time.Millisecond {
		return fmt.Errorf("process interval too small (minimum 10ms)")
	}

	if c.MaxConcurrentTasks < 1 {
		return fmt.Errorf("max concurrent tasks must be at least 1")
	}

	if c.TaskTimeout < time.Second {
		return fmt.Errorf("task timeout must be at least 1 second")
	}

	if c.EnableEncryption && c.EncryptionKey == "" {
		return fmt.Errorf("encryption key required when encryption is enabled")
	}

	return nil
}

// SaveConfig saves the configuration to a JSON file
func (c *Config) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// GetCustomParameter retrieves a custom parameter with type assertion
func (c *Config) GetCustomParameter(key string) (interface{}, bool) {
	value, exists := c.CustomParameters[key]
	return value, exists
}

// SetCustomParameter sets a custom parameter
func (c *Config) SetCustomParameter(key string, value interface{}) {
	c.CustomParameters[key] = value
}

// Environment types
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Common errors
var (
	ErrInvalidConfig       = fmt.Errorf("invalid configuration")
	ErrInvalidEnvironment  = fmt.Errorf("invalid environment")
	ErrInvalidLogLevel     = fmt.Errorf("invalid log level")
	ErrInvalidMemoryConfig = fmt.Errorf("invalid memory configuration")
)

// IsProduction returns whether the current environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == EnvProduction
}

// IsDevelopment returns whether the current environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == EnvDevelopment
}