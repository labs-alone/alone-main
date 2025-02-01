package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// RouteConfig holds configuration for a route
type RouteConfig struct {
	Path        string
	Method      string
	Handler     http.HandlerFunc
	Middleware  []mux.MiddlewareFunc
	RateLimit   *RateLimit
	Auth        bool
	ValidateReq bool
}

// RateLimit defines rate limiting parameters
type RateLimit struct {
	Requests int
	Window   time.Duration
}

// Router handles HTTP routing and middleware
type Router struct {
	*mux.Router
	logger     *zap.Logger
	metrics    *Metrics
	middleware map[string][]mux.MiddlewareFunc
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *MetaData   `json:"meta,omitempty"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// MetaData holds response metadata
type MetaData struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
	Page      int       `json:"page,omitempty"`
	PerPage   int       `json:"per_page,omitempty"`
	Total     int       `json:"total,omitempty"`
}

// NewRouter creates a new router instance
func NewRouter(logger *zap.Logger, metrics *Metrics) *Router {
	r := &Router{
		Router:     mux.NewRouter(),
		logger:     logger,
		metrics:    metrics,
		middleware: make(map[string][]mux.MiddlewareFunc),
	}

	// Setup default middleware
	r.setupDefaultMiddleware()
	return r
}

// setupDefaultMiddleware configures default middleware
func (r *Router) setupDefaultMiddleware() {
	// Request ID middleware
	r.Use(r.requestIDMiddleware)

	// Panic recovery middleware
	r.Use(r.recoveryMiddleware)

	// Request logging middleware
	r.Use(r.loggingMiddleware)
}

// AddRoute adds a new route with configuration
func (r *Router) AddRoute(config RouteConfig) error {
	route := r.HandleFunc(config.Path, r.wrapHandler(config))
	route.Methods(config.Method)

	// Apply route-specific middleware
	for _, m := range config.Middleware {
		route.Handler(m(route.GetHandler()))
	}

	// Apply rate limiting if configured
	if config.RateLimit != nil {
		route.Handler(r.rateLimitMiddleware(config.RateLimit)(route.GetHandler()))
	}

	// Apply authentication if required
	if config.Auth {
		route.Handler(r.authMiddleware(route.GetHandler()))
	}

	return nil
}

// wrapHandler wraps the handler with standard response formatting
func (r *Router) wrapHandler(config RouteConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var response APIResponse
		response.Meta = &MetaData{
			Timestamp: time.Now().UTC(),
			RequestID: req.Context().Value("request_id").(string),
		}

		// Validate request if required
		if config.ValidateReq {
			if err := r.validateRequest(req); err != nil {
				r.sendError(w, err, http.StatusBadRequest)
				return
			}
		}

		// Execute handler
		handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			config.Handler(w, req)
		})

		handler.ServeHTTP(w, req)
	}
}

// sendError sends an error response
func (r *Router) sendError(w http.ResponseWriter, err error, status int) {
	response := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    fmt.Sprintf("ERR_%d", status),
			Message: err.Error(),
		},
		Meta: &MetaData{
			Timestamp: time.Now().UTC(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// sendJSON sends a JSON response
func (r *Router) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	response := APIResponse{
		Success: true,
		Data:    data,
		Meta: &MetaData{
			Timestamp: time.Now().UTC(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// Middleware implementations

func (r *Router) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := req.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		ctx := context.WithValue(req.Context(), "request_id", requestID)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func (r *Router) rateLimitMiddleware(limit *RateLimit) mux.MiddlewareFunc {
	limiter := rate.NewLimiter(rate.Every(limit.Window), limit.Requests)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !limiter.Allow() {
				r.sendError(w, fmt.Errorf("rate limit exceeded"), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func (r *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := req.Header.Get("Authorization")
		if token == "" {
			r.sendError(w, fmt.Errorf("unauthorized"), http.StatusUnauthorized)
			return
		}
		// Validate token here
		next.ServeHTTP(w, req)
	})
}

func (r *Router) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		
		next.ServeHTTP(sw, req)
		
		r.logger.Info("Request processed",
			zap.String("method", req.Method),
			zap.String("path", req.URL.Path),
			zap.Int("status", sw.status),
			zap.Duration("duration", time.Since(start)),
			zap.String("request_id", req.Context().Value("request_id").(string)),
		)
	})
}

// Helper types and functions

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func generateRequestID() string {
	return uuid.New().String()
}

func (r *Router) validateRequest(req *http.Request) error {
	// Add request validation logic here
	return nil
}