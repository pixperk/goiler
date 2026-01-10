package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pixperk/goiler/internal/auth"
	"github.com/pixperk/goiler/internal/channel"
	"github.com/pixperk/goiler/internal/config"
	"github.com/pixperk/goiler/internal/server"
	"github.com/pixperk/goiler/internal/user"
	"github.com/pixperk/goiler/internal/websocket"
	"github.com/pixperk/goiler/internal/worker"
	"github.com/pixperk/goiler/pkg/otel"
)

// @title Goiler API
// @version 1.0
// @description Production-grade Go backend boilerplate
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg := config.Load()

	// Initialize context
	ctx := context.Background()

	// Initialize OpenTelemetry
	tracerProvider, err := otel.NewTracerProvider(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize tracer", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer tracerProvider.Shutdown(ctx)

	meterProvider, err := otel.NewMeterProvider(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize meter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer meterProvider.Shutdown(ctx)

	// Initialize database connection
	dbpool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbpool.Close()

	// Verify database connection
	if err := dbpool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Initialize repositories
	userRepo := user.NewPostgresRepository(dbpool)

	// Initialize auth service
	authService, err := auth.NewServiceFromConfig(cfg, &userRepoAdapter{repo: userRepo}, nil)
	if err != nil {
		logger.Error("failed to initialize auth service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Initialize handlers
	authHandler := auth.NewHandler(authService)
	userService := user.NewService(userRepo, nil)
	userHandler := user.NewHandler(userService)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(logger)
	go wsHub.Run()
	wsHandler := websocket.NewHandler(wsHub, logger)

	// Initialize worker client
	workerClient := worker.NewClient(cfg, logger)
	defer workerClient.Close()

	// Initialize pub/sub
	pubsub := channel.NewPubSub(logger, 100)
	_ = pubsub // Available for use in handlers

	// Initialize server
	srv := server.New(cfg, logger)

	// Setup middleware
	srv.SetupMiddleware()

	// Add OTEL middleware
	srv.Echo().Use(otel.CombinedMiddleware(cfg.OTEL.ServiceName, meterProvider))

	// Setup routes
	srv.SetupRoutes()

	// Register auth routes
	api := srv.Echo().Group("/api/v1")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/refresh", authHandler.RefreshToken)
	api.POST("/auth/logout", authHandler.Logout)

	// Protected routes
	protected := api.Group("")
	protected.Use(authHandler.AuthMiddleware())
	protected.GET("/users/me", userHandler.GetProfile)
	protected.PUT("/users/me", userHandler.UpdateProfile)
	protected.PUT("/users/me/password", userHandler.ChangePassword)
	protected.DELETE("/users/me", userHandler.DeleteAccount)

	// WebSocket routes
	api.GET("/ws", wsHandler.HandleConnection)
	protected.GET("/ws/auth", wsHandler.HandleAuthenticatedConnection)

	// Start server
	if err := srv.Start(); err != nil {
		logger.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// userRepoAdapter adapts user.Repository to auth.UserRepository
type userRepoAdapter struct {
	repo user.Repository
}

func (a *userRepoAdapter) Create(ctx context.Context, u *auth.User) error {
	return a.repo.Create(ctx, &user.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	})
}

func (a *userRepoAdapter) GetByID(ctx context.Context, id uuid.UUID) (*auth.User, error) {
	u, err := a.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

func (a *userRepoAdapter) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	u, err := a.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

func (a *userRepoAdapter) Update(ctx context.Context, u *auth.User) error {
	return a.repo.Update(ctx, &user.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	})
}

func (a *userRepoAdapter) Delete(ctx context.Context, id uuid.UUID) error {
	return a.repo.Delete(ctx, id)
}
