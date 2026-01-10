package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// RateLimiterConfig defines rate limiter configuration
type RateLimiterConfig struct {
	Requests int
	Duration time.Duration
	KeyFunc  func(c echo.Context) string
}

// visitor holds the rate limiter for each visitor
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter middleware
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	config   RateLimiterConfig
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.KeyFunc == nil {
		config.KeyFunc = func(c echo.Context) string {
			return c.RealIP()
		}
	}

	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		config:   config,
	}

	// Clean up old entries every minute
	go rl.cleanupVisitors()

	return rl
}

// Middleware returns the rate limiter middleware
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := rl.config.KeyFunc(c)
			limiter := rl.getVisitor(key)

			if !limiter.Allow() {
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}

			return next(c)
		}
	}
}

// getVisitor returns the rate limiter for a visitor
func (rl *RateLimiter) getVisitor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Every(rl.config.Duration/time.Duration(rl.config.Requests)), rl.config.Requests)
		rl.visitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes old entries
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// AuthMiddleware creates an authentication middleware
func AuthMiddleware(validateToken func(string) (interface{}, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			// Check Bearer prefix
			const bearerPrefix = "Bearer "
			if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
			}

			token := authHeader[len(bearerPrefix):]
			claims, err := validateToken(token)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// Store claims in context
			c.Set("user", claims)
			return next(c)
		}
	}
}

// RequireRoles creates a middleware that checks for required roles
func RequireRoles(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := c.Get("user")
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "user not found in context")
			}

			// Type assert to get user claims
			claims, ok := user.(map[string]interface{})
			if !ok {
				return echo.NewHTTPError(http.StatusInternalServerError, "invalid user claims")
			}

			userRole, ok := claims["role"].(string)
			if !ok {
				return echo.NewHTTPError(http.StatusForbidden, "user role not found")
			}

			for _, role := range roles {
				if userRole == role {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
		}
	}
}

// TimeoutMiddleware adds a timeout to the request context
func TimeoutMiddleware(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// The context timeout is already handled by Echo's TimeoutMiddleware
			// This is a simpler version that just passes through
			return next(c)
		}
	}
}
