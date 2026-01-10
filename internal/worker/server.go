package worker

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/pixperk/goiler/internal/config"
)

// Server represents the Asynq worker server
type Server struct {
	server   *asynq.Server
	mux      *asynq.ServeMux
	handlers *Handlers
	logger   *slog.Logger
}

// NewServer creates a new worker server
func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			// Number of concurrent workers
			Concurrency: 10,

			// Queue priorities
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},

			// Retry configuration
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,

			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.ErrorContext(ctx, "task processing failed",
					slog.String("type", task.Type()),
					slog.String("error", err.Error()),
				)
			}),

			// Logger adapter
			Logger: &asynqLogger{logger: logger},
		},
	)

	handlers := NewHandlers(logger)
	mux := asynq.NewServeMux()

	return &Server{
		server:   server,
		mux:      mux,
		handlers: handlers,
		logger:   logger,
	}
}

// RegisterHandlers registers all task handlers
func (s *Server) RegisterHandlers() {
	s.mux.HandleFunc(TypeEmailDelivery, s.handlers.HandleEmailDelivery)
	s.mux.HandleFunc(TypeWelcomeEmail, s.handlers.HandleWelcomeEmail)
	s.mux.HandleFunc(TypePasswordResetEmail, s.handlers.HandlePasswordResetEmail)
	s.mux.HandleFunc(TypeNotification, s.handlers.HandleNotification)
	s.mux.HandleFunc(TypeReportGeneration, s.handlers.HandleReportGeneration)
	s.mux.HandleFunc(TypeDataCleanup, s.handlers.HandleDataCleanup)
}

// Start starts the worker server
func (s *Server) Start() error {
	s.RegisterHandlers()
	s.logger.Info("starting worker server")
	return s.server.Start(s.mux)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	s.logger.Info("shutting down worker server")
	s.server.Shutdown()
}

// asynqLogger adapts slog.Logger to asynq.Logger interface
type asynqLogger struct {
	logger *slog.Logger
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug("asynq", slog.Any("message", args))
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info("asynq", slog.Any("message", args))
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn("asynq", slog.Any("message", args))
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error("asynq", slog.Any("message", args))
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error("asynq fatal", slog.Any("message", args))
}
