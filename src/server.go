package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	EnableCORS      bool
	AllowedOrigins  []string
	EnableMetrics   bool
	MetricsPath     string
	EnableHealth    bool
	HealthPath      string
}

// Server represents the HTTP server
type Server struct {
	config     *ServerConfig
	router     *mux.Router
	server     *http.Server
	logger     *zap.Logger
	metrics    *Metrics
	middleware []mux.MiddlewareFunc
	mu         sync.RWMutex
}

// Metrics holds the Prometheus metrics
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec
	ActiveConnGauge  prometheus.Gauge
	ErrorsTotal      *prometheus.CounterVec
}

// NewServer creates a new server instance
func NewServer(config *ServerConfig, logger *zap.Logger) *Server {
	if config == nil {
		config = &ServerConfig{
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			EnableCORS:      true,
			AllowedOrigins:  []string{"*"},
			EnableMetrics:   true,
			MetricsPath:     "/metrics",
			EnableHealth:    true,
			HealthPath:      "/health",
		}
	}

	s := &Server{
		config: config,
		router: mux.NewRouter(),
		logger: logger,
	}

	s.initializeMetrics()
	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// initializeMetrics sets up Prometheus metrics
func (s *Server) initializeMetrics() {
	if !s.config.EnableMetrics {
		return
	}

	s.metrics = &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		),
		ActiveConnGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_active_connections",
				Help: "Number of active HTTP connections",
			},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_errors_total",
				Help: "Total number of HTTP errors",
			},
			[]string{"method", "path", "error_type"},
		),
	}

	// Register metrics with Prometheus
	prometheus.MustRegister(
		s.metrics.RequestsTotal,
		s.metrics.RequestDuration,
		s.metrics.ResponseSize,
		s.metrics.ActiveConnGauge,
		s.metrics.ErrorsTotal,
	)
}

// setupMiddleware configures server middleware
func (s *Server) setupMiddleware() {
	// Add CORS middleware if enabled
	if s.config.EnableCORS {
		corsMiddleware := cors.New(cors.Options{
			AllowedOrigins:   s.config.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           300,
		})
		s.router.Use(corsMiddleware.Handler)
	}

	// Add metrics middleware
	if s.config.EnableMetrics {
		s.router.Use(s.metricsMiddleware)
	}

	// Add logging middleware
	s.router.Use(s.loggingMiddleware)

	// Add recovery middleware
	s.router.Use(s.recoveryMiddleware)
}

// setupRoutes configures server routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	if s.config.EnableHealth {
		s.router.HandleFunc(s.config.HealthPath, s.healthHandler).Methods("GET")
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.router.Handle(s.config.MetricsPath, promhttp.Handler()).Methods("GET")
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	// Channel for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Channel for server errors
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		s.logger.Info("Starting server", zap.Int("port", s.config.Port))
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %v", err)
	case <-stop:
		s.logger.Info("Shutting down server...")
		return s.Shutdown()
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %v", err)
	}

	s.logger.Info("Server shutdown complete")
	return nil
}

// AddRoute adds a new route to the server
func (s *Server) AddRoute(method, path string, handler http.HandlerFunc, middleware ...mux.MiddlewareFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	route := s.router.HandleFunc(path, handler).Methods(method)
	for _, m := range middleware {
		route.Handler(m(handler))
	}
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "up",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// metricsMiddleware collects metrics for each request
func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.ActiveConnGauge.Inc()
		defer s.metrics.ActiveConnGauge.Dec()

		next.ServeHTTP(w, r)

		duration := time.Since(start).Seconds()
		s.metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// loggingMiddleware logs request information
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		s.logger.Info("Request processed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// recoveryMiddleware recovers from panics
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("stack", string(debug.Stack())),
				)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}