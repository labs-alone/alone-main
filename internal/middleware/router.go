package api

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/labs-alone/alone-main/internal/api/handlers"
	"github.com/labs-alone/alone-main/internal/api/middleware"
	"github.com/labs-alone/alone-main/pkg/logger"
)

// Router handles all API routing
type Router struct {
	router *mux.Router
	log    *logger.Logger
}

// NewRouter creates a new router instance
func NewRouter(log *logger.Logger) *Router {
	return &Router{
		router: mux.NewRouter(),
		log:    log,
	}
}

// Setup configures all routes and middleware
func (r *Router) Setup() {
	// Create middleware instances
	loggingMiddleware := middleware.NewLoggingMiddleware(r.log)
	authMiddleware := middleware.NewAuthMiddleware(r.log)
	corsMiddleware := middleware.NewCORSMiddleware(nil, r.log)

	// Create handlers
	healthHandler := handlers.NewHealthHandler(r.log)
	aiHandler := handlers.NewAIHandler(r.log)
	solanaHandler := handlers.NewSolanaHandler(r.log)

	// Apply global middleware
	r.router.Use(loggingMiddleware.Handle)
	r.router.Use(loggingMiddleware.LogPanic)
	r.router.Use(corsMiddleware.Handle)
	r.router.Use(mux.CORSMethodMiddleware(r.router))

	// Set timeouts
	r.router.Use(middleware.TimeoutMiddleware(30 * time.Second))

	// Public routes
	r.router.HandleFunc("/health", healthHandler.Check).Methods(http.MethodGet)
	r.router.HandleFunc("/v1/auth/token", authMiddleware.GenerateTokenHandler).Methods(http.MethodPost)

	// API routes (protected)
	api := r.router.PathPrefix("/v1").Subrouter()
	api.Use(authMiddleware.Authenticate)

	// AI routes
	ai := api.PathPrefix("/ai").Subrouter()
	ai.HandleFunc("/complete", aiHandler.Complete).Methods(http.MethodPost)
	ai.HandleFunc("/stream", aiHandler.Stream).Methods(http.MethodPost)

	// Solana routes
	solana := api.PathPrefix("/solana").Subrouter()
	solana.HandleFunc("/balance", solanaHandler.GetBalance).Methods(http.MethodGet)
	solana.HandleFunc("/transfer", solanaHandler.Transfer).Methods(http.MethodPost)
	solana.HandleFunc("/swap", solanaHandler.Swap).Methods(http.MethodPost)

	// Admin routes (protected + admin role)
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(authMiddleware.RequireRole("admin"))
	admin.HandleFunc("/metrics", handlers.GetMetrics).Methods(http.MethodGet)
	admin.HandleFunc("/users", handlers.ManageUsers).Methods(http.MethodGet, http.MethodPost)

	// Not found handler
	r.router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.log.Warn("Not found",
			"path", r.URL.Path,
			"method", r.Method,
		)
		http.Error(w, "Not found", http.StatusNotFound)
	})
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

// GetRouter returns the underlying mux router
func (r *Router) GetRouter() *mux.Router {
	return r.router
}

// TimeoutMiddleware adds a timeout to the request context
func TimeoutMiddleware(timeout time.Duration) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan bool)
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				done <- true
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
		})
	}
}