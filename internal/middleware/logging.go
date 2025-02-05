package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labs-alone/alone-main/pkg/logger"
)

// LoggingMiddleware handles request logging
type LoggingMiddleware struct {
	log *logger.Logger
}

// NewLoggingMiddleware creates a new logging middleware instance
func NewLoggingMiddleware(log *logger.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{log: log}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Handle implements the logging middleware
func (m *LoggingMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		// Add request ID to context
		ctx := r.Context()
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code
		wrapped := wrapResponseWriter(w)

		// Add request ID to response headers
		wrapped.Header().Set("X-Request-ID", requestID)

		// Log request details
		m.log.Info("Request started",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Log response details
		m.log.Info("Request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.Status(),
			"duration", duration.String(),
			"duration_ms", duration.Milliseconds(),
		)

		// Log detailed error information for non-2xx responses
		if wrapped.Status() >= 400 {
			m.log.Error("Request error",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.Status(),
				"duration", duration.String(),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		}

		// Log performance warning for slow requests
		if duration > 1*time.Second {
			m.log.Warn("Slow request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"duration", duration.String(),
			)
		}
	})
}

// LogPanic recovers from panics and logs them
func (m *LoggingMiddleware) LogPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := r.Header.Get("X-Request-ID")
				m.log.Error("Panic recovered",
					"request_id", requestID,
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}