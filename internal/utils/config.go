package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config manages application configuration
type Config struct {
	// Core settings
	Environment string `json:"environment" yaml:"environment"`
	LogLevel    string `json:"log_level" yaml:"log_level"`
	Debug       bool   `json:"debug" yaml:"debug"`

	// Server settings
	Server struct {
		Host string `json:"host" yaml:"host"`
		Port int    `json:"port" yaml:"port"`
	} `json:"server" yaml:"server"`

	// Solana settings
	Solana struct {
		Endpoint    string `json:"endpoint" yaml:"endpoint"`
		WsEndpoint  string `json:"ws_endpoint" yaml:"ws_endpoint"`
		Commitment  string `json:"commitment" yaml:"commitment"`
		MaxRetries  int    `json:"max_retries" yaml:"max_retries"`
		Environment string `json:"environment" yaml:"environment"`
	} `json:"solana" yaml:"solana"`

	// OpenAI settings
	OpenAI struct {
		APIKey      string  `json:"api_key" yaml:"api_key"`
		Model       string  `json:"model" yaml:"model"`
		MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`
		Temperature float32 `json:"temperature" yaml:"temperature"`
	} `json:"openai" yaml:"openai"`

	// Database settings
	Database struct {
		Host     string `json:"host" yaml:"host"`
		Port     int    `json:"port" yaml:"port"`
		Name     string `json:"name" yaml:"name"`
		User     string `json:"user" yaml:"user"`
		Password string `json:"password" yaml:"password"`
		SSLMode  string `json:"ssl_mode" yaml:"ssl_mode"`
	} `json:"database" yaml:"database"`

	// Cache settings
	Cache struct {
		Enabled  bool   `json:"enabled" yaml:"enabled"`
		Type     string `json:"type" yaml:"type"`
		Address  string `json:"address" yaml:"address"`
		Password string `json:"password" yaml:"password"`
		TTL      int    `json:"ttl" yaml:"ttl"`
	} `json:"cache" yaml:"cache"`

	// Metrics settings
	Metrics struct {
		Enabled bool   `json:"enabled" yaml:"enabled"`
		Path    string `json:"path" yaml:"path"`
	} `json:"metrics" yaml:"metrics"`

	mu sync.RWMutex
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	config := &Config{}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Determine file type and parse
	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", filepath.Ext(path))
	}

	// Load environment variables
	config.loadEnvOverrides()

	return config, nil
}

// loadEnvOverrides loads configuration overrides from environment variables
func (c *Config) loadEnvOverrides() {
	if env := os.Getenv("APP_ENVIRONMENT"); env != "" {
		c.Environment = env
	}
	if level := os.Getenv("APP_LOG_LEVEL"); level != "" {
		c.LogLevel = level
	}
	if endpoint := os.Getenv("SOLANA_ENDPOINT"); endpoint != "" {
		c.Solana.Endpoint = endpoint
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.OpenAI.APIKey = apiKey
	}
}

// Save saves the current configuration to a file
func (c *Config) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var data []byte
	var err error

	// Determine file type and encode
	switch filepath.Ext(path) {
	case ".json":
		data, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode JSON config: %w", err)
		}
	case ".yaml", ".yml":
		data, err = yaml.Marshal(c)
		if err != nil {
			return fmt.Errorf("failed to encode YAML config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", filepath.Ext(path))
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get retrieves a configuration value
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Add implementation for getting nested config values
	return nil
}

// Set updates a configuration value
func (c *Config) Set(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add implementation for setting nested config values
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if c.Solana.Endpoint == "" {
		return fmt.Errorf("Solana endpoint is required")
	}
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}
	return nil
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() (*Config, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	clone := &Config{}
	if err := json.Unmarshal(data, clone); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return clone, nil
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}