package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// SetupRoutes configures all routes
func (s *Server) SetupRoutes() {
	// Health check
	s.echo.GET("/health", s.healthCheck)
	s.echo.GET("/ready", s.readyCheck)

	// Swagger docs (only in development)
	if s.config.App.Env == "development" {
		s.echo.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	// API v1 routes
	v1 := s.echo.Group("/api/v1")

	// Apply rate limiting to API routes
	rateLimiter := NewRateLimiter(RateLimiterConfig{
		Requests: s.config.RateLimit.Requests,
		Duration: s.config.RateLimit.Duration,
	})
	v1.Use(rateLimiter.Middleware())

	// Public routes (no auth required)
	public := v1.Group("")
	_ = public // Will be used for auth routes

	// Protected routes (auth required)
	// protected := v1.Group("")
	// protected.Use(AuthMiddleware(tokenValidator))

	// Example route groups:
	// s.setupAuthRoutes(public)
	// s.setupUserRoutes(protected)
	// s.setupWebSocketRoutes(v1)
}

// RegisterAuthRoutes registers auth-related routes
func (s *Server) RegisterAuthRoutes(group *echo.Group, handler interface{}) {
	// Type assert handler to auth handler interface
	type AuthHandler interface {
		Register(c echo.Context) error
		Login(c echo.Context) error
		RefreshToken(c echo.Context) error
		Logout(c echo.Context) error
	}

	if h, ok := handler.(AuthHandler); ok {
		group.POST("/auth/register", h.Register)
		group.POST("/auth/login", h.Login)
		group.POST("/auth/refresh", h.RefreshToken)
		group.POST("/auth/logout", h.Logout)
	}
}

// RegisterUserRoutes registers user-related routes
func (s *Server) RegisterUserRoutes(group *echo.Group, handler interface{}, authMiddleware echo.MiddlewareFunc) {
	type UserHandler interface {
		GetProfile(c echo.Context) error
		UpdateProfile(c echo.Context) error
		ChangePassword(c echo.Context) error
		DeleteAccount(c echo.Context) error
	}

	if h, ok := handler.(UserHandler); ok {
		users := group.Group("/users", authMiddleware)
		users.GET("/me", h.GetProfile)
		users.PUT("/me", h.UpdateProfile)
		users.PUT("/me/password", h.ChangePassword)
		users.DELETE("/me", h.DeleteAccount)
	}
}

// RegisterWebSocketRoutes registers WebSocket routes
func (s *Server) RegisterWebSocketRoutes(group *echo.Group, handler interface{}) {
	type WSHandler interface {
		HandleConnection(c echo.Context) error
	}

	if h, ok := handler.(WSHandler); ok {
		group.GET("/ws", h.HandleConnection)
	}
}

// healthCheck returns the health status
// @Summary Health check
// @Description Returns the health status of the service
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (s *Server) healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": s.config.App.Name,
	})
}

// readyCheck returns the readiness status
// @Summary Readiness check
// @Description Returns the readiness status of the service
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /ready [get]
func (s *Server) readyCheck(c echo.Context) error {
	// TODO: Add actual readiness checks (DB connection, Redis, etc.)
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ready",
	})
}
