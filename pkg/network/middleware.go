package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// MiddlewareConfig holds middleware configuration
type MiddlewareConfig struct {
	JWT struct {
		Secret     string
		Issuer     string
		Expiration time.Duration
	}
	RateLimit struct {
		RequestsPerSecond int
		BurstSize        int
	}
	Security struct {
		AllowedOrigins []string
		AllowedMethods []string
		AllowedHeaders []string
		MaxAge         int
	}
	Cache struct {
		Enabled     bool
		DefaultTTL  time.Duration
		MaxSize     int
		PurgeInterval time.Duration
	}
}

// Middleware manager
type MiddlewareManager struct {
	config    *MiddlewareConfig
	logger    *zap.Logger
	metrics   *Metrics
	cache     *sync.Map
	limiters  *sync.Map
	blacklist *sync.Map
}

// NewMiddlewareManager creates a new middleware manager
func NewMiddlewareManager(config *MiddlewareConfig, logger *zap.Logger, metrics *Metrics) *MiddlewareManager {
	return &MiddlewareManager{
		config:    config,
		logger:    logger,
		metrics:   metrics,
		cache:     &sync.Map{},
		limiters:  &sync.Map{},
		blacklist: &sync.Map{},
	}
}

// Security Middleware

func (m *MiddlewareManager) SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// Authentication Middleware

func (m *MiddlewareManager) JWTAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(m.config.JWT.Secret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "user", claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Rate Limiting Middleware

func (m *MiddlewareManager) RateLimit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get or create rate limiter for IP
			ip := r.RemoteAddr
			limiter, _ := m.limiters.LoadOrStore(ip, rate.NewLimiter(
				rate.Limit(m.config.RateLimit.RequestsPerSecond),
				m.config.RateLimit.BurstSize,
			))

			if !limiter.(*rate.Limiter).Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Caching Middleware

func (m *MiddlewareManager) Cache(ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.config.Cache.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Generate cache key
			key := fmt.Sprintf("%s:%s", r.Method, r.URL.String())

			// Check cache
			if cached, ok := m.cache.Load(key); ok {
				entry := cached.(*CacheEntry)
				if !entry.Expired() {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Cache", "HIT")
					w.Write(entry.Data)
					return
				}
			}

			// Create response recorder
			rec := &ResponseRecorder{
				ResponseWriter: w,
				StatusCode:    http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			// Cache response if successful
			if rec.StatusCode == http.StatusOK {
				m.cache.Store(key, &CacheEntry{
					Data:    rec.Body.Bytes(),
					Expires: time.Now().Add(ttl),
				})
			}
		})
	}
}

// Metrics Middleware

func (m *MiddlewareManager) Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &ResponseRecorder{
				ResponseWriter: w,
				StatusCode:    http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			m.metrics.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
			m.metrics.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", rec.StatusCode)).Inc()
			m.metrics.ResponseSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(rec.Body.Len()))
		})
	}
}

// Recovery Middleware

func (m *MiddlewareManager) Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					m.logger.Error("panic recovered",
						zap.Any("error", err),
						zap.String("stack", string(stack)),
					)

					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Helper types and functions

type CacheEntry struct {
	Data    []byte
	Expires time.Time
}

func (c *CacheEntry) Expired() bool {
	return time.Now().After(c.Expires)
}

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Cleanup function for middleware manager
func (m *MiddlewareManager) Cleanup() {
	// Clear caches
	m.cache.Range(func(key, value interface{}) bool {
		m.cache.Delete(key)
		return true
	})

	// Clear rate limiters
	m.limiters.Range(func(key, value interface{}) bool {
		m.limiters.Delete(key)
		return true
	})

	// Clear blacklist
	m.blacklist.Range(func(key, value interface{}) bool {
		m.blacklist.Delete(key)
		return true
	})
}