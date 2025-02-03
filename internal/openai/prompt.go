package openai

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/labs-alone/alone-main/internal/utils"
)

// PromptManager handles prompt construction and management
type PromptManager struct {
	templates    map[string]string
	cache        *PromptCache
	logger       *utils.Logger
	maxTokens    int
	temperature  float32
	mu           sync.RWMutex
}

// PromptCache provides caching for generated prompts
type PromptCache struct {
	items map[string]PromptCacheItem
	mu    sync.RWMutex
}

// PromptCacheItem represents a cached prompt
type PromptCacheItem struct {
	prompt    string
	messages  []ChatMessage
	created   time.Time
	expiresAt time.Time
}

// PromptTemplate represents a structured prompt template
type PromptTemplate struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Template    string            `json:"template"`
	Variables   []string          `json:"variables"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float32          `json:"temperature"`
	Metadata    map[string]string `json:"metadata"`
}

// PromptOptions configures prompt generation
type PromptOptions struct {
	MaxTokens    int
	Temperature  float32
	UseCache     bool
	CacheTTL     time.Duration
	SystemPrompt string
}

// NewPromptManager creates a new prompt manager
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]string),
		cache: &PromptCache{
			items: make(map[string]PromptCacheItem),
		},
		logger:      utils.NewLogger(),
		maxTokens:   2000,
		temperature: 0.7,
	}
}

// AddTemplate adds a new prompt template
func (pm *PromptManager) AddTemplate(name, template string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if name == "" || template == "" {
		return fmt.Errorf("name and template are required")
	}

	pm.templates[name] = template
	pm.logger.Info("Added template:", name)
	return nil
}

// LoadTemplates loads templates from JSON
func (pm *PromptManager) LoadTemplates(data []byte) error {
	var templates []PromptTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return fmt.Errorf("failed to unmarshal templates: %w", err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, tmpl := range templates {
		pm.templates[tmpl.Name] = tmpl.Template
	}

	pm.logger.Info("Loaded templates:", len(templates))
	return nil
}

// GeneratePrompt creates a prompt from a template
func (pm *PromptManager) GeneratePrompt(
	templateName string,
	variables map[string]string,
	opts *PromptOptions,
) ([]ChatMessage, error) {
	if opts == nil {
		opts = &PromptOptions{
			MaxTokens:    pm.maxTokens,
			Temperature:  pm.temperature,
			UseCache:     true,
			CacheTTL:     time.Hour,
			SystemPrompt: "You are a helpful assistant.",
		}
	}

	// Check cache if enabled
	if opts.UseCache {
		if cached, ok := pm.getFromCache(templateName, variables); ok {
			return cached, nil
		}
	}

	template, err := pm.getTemplate(templateName)
	if err != nil {
		return nil, err
	}

	prompt := pm.interpolateTemplate(template, variables)

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: opts.SystemPrompt,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Cache the result if enabled
	if opts.UseCache {
		pm.cachePrompt(templateName, variables, messages, opts.CacheTTL)
	}

	return messages, nil
}

// GenerateCodePrompt creates a prompt specifically for code-related queries
func (pm *PromptManager) GenerateCodePrompt(
	language string,
	task string,
	context map[string]string,
) ([]ChatMessage, error) {
	systemPrompt := fmt.Sprintf(
		"You are an expert %s programmer. Provide clear, efficient, and well-documented solutions.",
		language,
	)

	prompt := strings.Builder{}
	prompt.WriteString(fmt.Sprintf("Task: %s\n\n", task))

	if context != nil {
		prompt.WriteString("Context:\n")
		for key, value := range context {
			prompt.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: prompt.String(),
		},
	}

	return messages, nil
}

// GetTemplate retrieves a template
func (pm *PromptManager) getTemplate(name string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	template, ok := pm.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	return template, nil
}

// interpolateTemplate replaces variables in template
func (pm *PromptManager) interpolateTemplate(
	template string,
	variables map[string]string,
) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// Cache operations
func (pm *PromptManager) getFromCache(
	templateName string,
	variables map[string]string,
) ([]ChatMessage, bool) {
	key := pm.getCacheKey(templateName, variables)

	pm.cache.mu.RLock()
	defer pm.cache.mu.RUnlock()

	if item, ok := pm.cache.items[key]; ok {
		if time.Now().Before(item.expiresAt) {
			return item.messages, true
		}
	}

	return nil, false
}

func (pm *PromptManager) cachePrompt(
	templateName string,
	variables map[string]string,
	messages []ChatMessage,
	ttl time.Duration,
) {
	key := pm.getCacheKey(templateName, variables)

	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	pm.cache.items[key] = PromptCacheItem{
		messages:  messages,
		created:   time.Now(),
		expiresAt: time.Now().Add(ttl),
	}
}

func (pm *PromptManager) getCacheKey(
	templateName string,
	variables map[string]string,
) string {
	parts := []string{templateName}
	for k, v := range variables {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, "|")
}

// CleanCache removes expired cache entries
func (pm *PromptManager) CleanCache() {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	now := time.Now()
	for key, item := range pm.cache.items {
		if now.After(item.expiresAt) {
			delete(pm.cache.items, key)
		}
	}
}

// ClearCache removes all cache entries
func (pm *PromptManager) ClearCache() {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	pm.cache.items = make(map[string]PromptCacheItem)
}