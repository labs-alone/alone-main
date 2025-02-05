package middleware

import (
	"net/http"
	"strings"

	"github.com/labs-alone/alone-main/pkg/logger"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
	Debug          bool
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
			"Origin",
			"X-Requested-With",
		},
		MaxAge: 86400, // 24 hours
		Debug:  false,
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing
type CORSMiddleware struct {
	config *CORSConfig
	log    *logger.Logger
}

// NewCORSMiddleware creates a new CORS middleware instance
func NewCORSMiddleware(config *CORSConfig, log *logger.Logger) *CORSMiddleware {
	if config == nil {
		config = DefaultCORSConfig()
	}
	return &CORSMiddleware{
		config: config,
		log:    log,
	}
}

// Handle implements the CORS middleware
func (m *CORSMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			m.handlePreflight(w, r)
			return
		}

		// Set CORS headers for all requests
		m.setCORSHeaders(w, origin)

		// Check if origin is allowed
		if !m.isOriginAllowed(origin) {
			if m.config.Debug {
				m.log.Debug("CORS: Origin not allowed", "origin", origin)
			}
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handlePreflight handles OPTIONS requests
func (m *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	method := r.Header.Get("Access-Control-Request-Method")
	headers := r.Header.Get("Access-Control-Request-Headers")

	if !m.isOriginAllowed(origin) {
		if m.config.Debug {
			m.log.Debug("CORS: Preflight origin not allowed", "origin", origin)
		}
		http.Error(w, "Origin not allowed", http.StatusForbidden)
		return
	}

	if !m.isMethodAllowed(method) {
		if m.config.Debug {
			m.log.Debug("CORS: Method not allowed", "method", method)
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !m.areHeadersAllowed(headers) {
		if m.config.Debug {
			m.log.Debug("CORS: Headers not allowed", "headers", headers)
		}
		http.Error(w, "Headers not allowed", http.StatusForbidden)
		return
	}

	m.setCORSHeaders(w, origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ","))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ","))
	w.Header().Set("Access-Control-Max-Age", string(m.config.MaxAge))
	w.WriteHeader(http.StatusNoContent)
}

// setCORSHeaders sets the basic CORS headers
func (m *CORSMiddleware) setCORSHeaders(w http.ResponseWriter, origin string) {
	if m.config.AllowedOrigins[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Vary", "Origin")
}

// isOriginAllowed checks if the origin is allowed
func (m *CORSMiddleware) isOriginAllowed(origin string) bool {
	if len(m.config.AllowedOrigins) == 0 {
		return false
	}

	if m.config.AllowedOrigins[0] == "*" {
		return true
	}

	for _, allowedOrigin := range m.config.AllowedOrigins {
		if allowedOrigin == origin {
			return true
		}
	}

	return false
}

// isMethodAllowed checks if the method is allowed
func (m *CORSMiddleware) isMethodAllowed(method string) bool {
	if method == "" {
		return false
	}

	for _, allowedMethod := range m.config.AllowedMethods {
		if allowedMethod == method {
			return true
		}
	}

	return false
}

// areHeadersAllowed checks if the headers are allowed
func (m *CORSMiddleware) areHeadersAllowed(headers string) bool {
	if headers == "" {
		return true
	}

	for _, header := range strings.Split(headers, ",") {
		header = strings.TrimSpace(header)
		found := false
		for _, allowedHeader := range m.config.AllowedHeaders {
			if strings.EqualFold(allowedHeader, header) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}