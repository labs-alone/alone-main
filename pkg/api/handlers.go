package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labs-alone/alone-main/internal/core"
	"github.com/labs-alone/alone-main/internal/solana"
	"github.com/labs-alone/alone-main/internal/openai"
	"github.com/labs-alone/alone-main/internal/utils"
)

// Handler manages API request handling
type Handler struct {
	engine  *core.Engine
	solana  *solana.Client
	openai  *openai.Client
	logger  *utils.Logger
	metrics *Metrics
}

// Metrics tracks API usage
type Metrics struct {
	RequestCount    uint64
	ErrorCount     uint64
	AverageLatency time.Duration
	LastRequest    time.Time
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// NewHandler creates a new API handler
func NewHandler(engine *core.Engine, solana *solana.Client, openai *openai.Client) *Handler {
	return &Handler{
		engine:  engine,
		solana:  solana,
		openai:  openai,
		logger:  utils.NewLogger(),
		metrics: &Metrics{},
	}
}

// handleHealth handles health check requests
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"services": map[string]string{
			"engine": h.engine.Status(),
			"solana": h.solana.Status(),
			"openai": "connected",
		},
	}

	h.sendJSON(w, Response{Success: true, Data: status})
}

// handleSolanaBalance handles balance check requests
func (h *Handler) handleSolanaBalance(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		h.sendError(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	balance, err := h.solana.GetBalance(r.Context(), address)
	if err != nil {
		h.sendError(w, "failed to get balance: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendJSON(w, Response{Success: true, Data: balance})
}

// handleSolanaTransaction handles transaction requests
func (h *Handler) handleSolanaTransaction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Amount uint64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	signature, err := h.solana.SendTransaction(r.Context(), req.From, req.To, req.Amount)
	if err != nil {
		h.sendError(w, "failed to send transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendJSON(w, Response{Success: true, Data: map[string]string{"signature": signature}})
}

// handleOpenAICompletion handles AI completion requests
func (h *Handler) handleOpenAICompletion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt      string  `json:"prompt"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float32 `json:"temperature,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	completion, err := h.openai.CreateChatCompletion(r.Context(), &openai.ChatCompletionRequest{
		Messages: []openai.ChatMessage{
			{Role: "user", Content: req.Prompt},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})

	if err != nil {
		h.sendError(w, "failed to get completion: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendJSON(w, Response{Success: true, Data: completion})
}

// handleMetrics handles metrics requests
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"api": h.metrics,
		"solana": map[string]interface{}{
			"requests": h.solana.GetMetrics(),
		},
		"openai": map[string]interface{}{
			"requests": h.openai.GetMetrics(),
		},
	}

	h.sendJSON(w, Response{Success: true, Data: metrics})
}

// Middleware for logging
func (h *Handler) loggerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		h.logger.Info("Request started",
			map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			})

		next(w, r)

		duration := time.Since(start)
		h.updateMetrics(duration)

		h.logger.Info("Request completed",
			map[string]interface{}{
				"method":   r.Method,
				"path":     r.URL.Path,
				"duration": duration,
			})
	}
}

// Helper methods
func (h *Handler) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", 
			map[string]interface{}{"error": err.Error()})
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) sendError(w http.ResponseWriter, message string, code int) {
	h.metrics.ErrorCount++
	h.logger.Error(message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response{Success: false, Error: message})
}

func (h *Handler) updateMetrics(duration time.Duration) {
	h.metrics.RequestCount++
	h.metrics.LastRequest = time.Now()
	h.metrics.AverageLatency = (h.metrics.AverageLatency + duration) / 2
}

// GetRoutes returns the handler routes
func (h *Handler) GetRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/health":             h.loggerMiddleware(h.handleHealth),
		"/solana/balance":     h.loggerMiddleware(h.handleSolanaBalance),
		"/solana/transaction": h.loggerMiddleware(h.handleSolanaTransaction),
		"/openai/completion":  h.loggerMiddleware(h.handleOpenAICompletion),
		"/metrics":           h.loggerMiddleware(h.handleMetrics),
	}
}