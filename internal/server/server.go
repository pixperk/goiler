package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pixperk/goiler/internal/config"
	"github.com/pixperk/goiler/pkg/validator"
)

// Server represents the HTTP server
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *slog.Logger
}

// New creates a new server instance
func New(cfg *config.Config, logger *slog.Logger) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Set custom validator
	e.Validator = validator.New()

	// Set custom error handler
	e.HTTPErrorHandler = customErrorHandler(logger)

	return &Server{
		echo:   e,
		config: cfg,
		logger: logger,
	}
}

// SetupMiddleware configures all middleware
func (s *Server) SetupMiddleware() {
	// Request ID
	s.echo.Use(middleware.RequestID())

	// Logger
	s.echo.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogError:    true,
		LogMethod:   true,
		LogLatency:  true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				s.logger.Info("request",
					slog.String("method", v.Method),
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				)
			} else {
				s.logger.Error("request error",
					slog.String("method", v.Method),
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.String("error", v.Error.Error()),
					slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				)
			}
			return nil
		},
	}))

	// Recover
	s.echo.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 4 << 10,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			s.logger.Error("panic recovered",
				slog.String("error", err.Error()),
				slog.String("stack", string(stack)),
			)
			return nil
		},
	}))

	// CORS
	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// Secure headers
	s.echo.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'",
	}))

	// Body limit
	s.echo.Use(middleware.BodyLimit("2M"))

	// Gzip compression
	s.echo.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
}

// Echo returns the underlying echo instance
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// Start starts the server with graceful shutdown
func (s *Server) Start() error {
	// Start server in goroutine
	go func() {
		addr := ":" + s.config.App.Port
		s.logger.Info("starting server", slog.String("addr", addr))
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	s.logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.echo.Shutdown(ctx); err != nil {
		return err
	}

	s.logger.Info("server stopped")
	return nil
}

// customErrorHandler returns a custom error handler
func customErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		code := http.StatusInternalServerError
		message := "Internal server error"

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if m, ok := he.Message.(string); ok {
				message = m
			}
		}

		logger.Error("HTTP error",
			slog.Int("status", code),
			slog.String("message", message),
			slog.String("error", err.Error()),
			slog.String("path", c.Request().URL.Path),
		)

		if err := c.JSON(code, map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    http.StatusText(code),
				"message": message,
			},
		}); err != nil {
			logger.Error("failed to send error response", slog.String("error", err.Error()))
		}
	}
}
