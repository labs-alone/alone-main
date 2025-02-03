package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/labs-alone/alone-main/internal/utils"
)

// Router manages API routing
type Router struct {
	router  *mux.Router
	handler *Handler
	logger  *utils.Logger
	config  *utils.Config
}

// RouterConfig holds router configuration
type RouterConfig struct {
	EnableCORS     bool
	EnableMetrics  bool
	RateLimit      int
	Timeout       time.Duration
	MaxBodySize   int64
	TrustedProxies []string
}

// NewRouter creates a new router instance
func NewRouter(handler *Handler, config *utils.Config) *Router {
	r := &Router{
		router:  mux.NewRouter(),
		handler: handler,
		logger:  utils.NewLogger(),
		config:  config,
	}

	r.setupRoutes()
	r.setupMiddleware()

	return r
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// API version prefix
	api := r.router.PathPrefix("/api/v1").Subrouter()

	// Health and metrics
	api.HandleFunc("/health", r.handler.handleHealth).Methods(http.MethodGet)
	api.HandleFunc("/metrics", r.handler.handleMetrics).Methods(http.MethodGet)

	// Solana endpoints
	solana := api.PathPrefix("/solana").Subrouter()
	solana.HandleFunc("/balance", r.handler.handleSolanaBalance).Methods(http.MethodGet)
	solana.HandleFunc("/transaction", r.handler.handleSolanaTransaction).Methods(http.MethodPost)
	solana.HandleFunc("/account/{address}", r.handleSolanaAccount()).Methods(http.MethodGet)
	solana.HandleFunc("/transaction/{signature}", r.handleSolanaTransactionStatus()).Methods(http.MethodGet)

	// OpenAI endpoints
	ai := api.PathPrefix("/ai").Subrouter()
	ai.HandleFunc("/completion", r.handler.handleOpenAICompletion).Methods(http.MethodPost)
	ai.HandleFunc("/analyze", r.handleAIAnalysis()).Methods(http.MethodPost)

	// Documentation
	api.HandleFunc("/docs", r.handleDocs()).Methods(http.MethodGet)
	api.HandleFunc("/swagger.json", r.handleSwagger()).Methods(http.MethodGet)
}

// setupMiddleware configures global middleware
func (r *Router) setupMiddleware() {
	r.router.Use(r.loggingMiddleware)
	r.router.Use(r.recoveryMiddleware)
	r.router.Use(r.corsMiddleware)
	r.router.Use(r.securityMiddleware)
	r.router.Use(r.rateLimitMiddleware)
	r.router.Use(r.timeoutMiddleware)
}

// Middleware implementations
func (r *Router) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		rw := &responseWriter{w, http.StatusOK}

		next.ServeHTTP(rw, req)

		duration := time.Since(start)
		r.logger.Info("Request processed",
			map[string]interface{}{
				"method":   req.Method,
				"path":     req.URL.Path,
				"status":   rw.status,
				"duration": duration,
				"ip":       req.RemoteAddr,
			})
	})
}

func (r *Router) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Error("Panic recovered",
					map[string]interface{}{"error": fmt.Sprint(err)})
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

func (r *Router) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func (r *Router) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next.ServeHTTP(w, req)
	})
}

func (r *Router) rateLimitMiddleware(next http.Handler) http.Handler {
	// Implement rate limiting logic here
	return next
}

func (r *Router) timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
		defer cancel()

		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// Additional route handlers
func (r *Router) handleSolanaAccount() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		address := vars["address"]
		// Implement account info retrieval
		r.handler.sendJSON(w, Response{Success: true, Data: map[string]string{"address": address}})
	}
}

func (r *Router) handleSolanaTransactionStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		signature := vars["signature"]
		// Implement transaction status retrieval
		r.handler.sendJSON(w, Response{Success: true, Data: map[string]string{"signature": signature}})
	}
}

func (r *Router) handleAIAnalysis() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Implement AI analysis
		r.handler.sendJSON(w, Response{Success: true, Data: "Analysis completed"})
	}
}

func (r *Router) handleDocs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Serve API documentation
		r.handler.sendJSON(w, Response{Success: true, Data: "API Documentation"})
	}
}

func (r *Router) handleSwagger() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Serve Swagger JSON
		r.handler.sendJSON(w, Response{Success: true, Data: "Swagger specification"})
	}
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// GetRouter returns the underlying mux router
func (r *Router) GetRouter() *mux.Router {
	return r.router
}

// Custom response writer to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}