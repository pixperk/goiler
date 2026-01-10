package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pixperk/goiler/internal/config"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRepository defines the interface for user data access
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// TokenRepository defines the interface for token blacklist/storage
type TokenRepository interface {
	// StoreRefreshToken stores a refresh token
	StoreRefreshToken(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error
	// RevokeRefreshToken revokes a refresh token
	RevokeRefreshToken(ctx context.Context, tokenID uuid.UUID) error
	// IsRefreshTokenRevoked checks if a refresh token is revoked
	IsRefreshTokenRevoked(ctx context.Context, tokenID uuid.UUID) (bool, error)
	// RevokeAllUserTokens revokes all tokens for a user
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// Service handles authentication business logic
type Service struct {
	userRepo      UserRepository
	tokenRepo     TokenRepository
	tokenMaker    TokenMaker
	hasher        PasswordHasher
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// ServiceConfig holds service configuration
type ServiceConfig struct {
	UserRepo      UserRepository
	TokenRepo     TokenRepository
	TokenMaker    TokenMaker
	Hasher        PasswordHasher
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// NewService creates a new auth service
func NewService(cfg ServiceConfig) *Service {
	if cfg.Hasher == nil {
		cfg.Hasher = DefaultPasswordHasher()
	}
	if cfg.AccessExpiry == 0 {
		cfg.AccessExpiry = 15 * time.Minute
	}
	if cfg.RefreshExpiry == 0 {
		cfg.RefreshExpiry = 7 * 24 * time.Hour
	}

	return &Service{
		userRepo:      cfg.UserRepo,
		tokenRepo:     cfg.TokenRepo,
		tokenMaker:    cfg.TokenMaker,
		hasher:        cfg.Hasher,
		accessExpiry:  cfg.AccessExpiry,
		refreshExpiry: cfg.RefreshExpiry,
	}
}

// NewServiceFromConfig creates a new auth service from config
func NewServiceFromConfig(cfg *config.Config, userRepo UserRepository, tokenRepo TokenRepository) (*Service, error) {
	var symmetricKey []byte
	if cfg.Auth.PASETOSymmetricKey != "" {
		symmetricKey = []byte(cfg.Auth.PASETOSymmetricKey)
		// Pad or truncate to 32 bytes
		if len(symmetricKey) < 32 {
			padded := make([]byte, 32)
			copy(padded, symmetricKey)
			symmetricKey = padded
		} else if len(symmetricKey) > 32 {
			symmetricKey = symmetricKey[:32]
		}
	}

	tokenMaker, err := NewTokenMaker(cfg.Auth.Type, cfg.Auth.JWTSecret, symmetricKey)
	if err != nil {
		return nil, err
	}

	return NewService(ServiceConfig{
		UserRepo:      userRepo,
		TokenRepo:     tokenRepo,
		TokenMaker:    tokenMaker,
		Hasher:        DefaultPasswordHasher(),
		AccessExpiry:  cfg.Auth.JWTAccessExpiry,
		RefreshExpiry: cfg.Auth.JWTRefreshExpiry,
	}), nil
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    time.Time     `json:"expires_at"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	// Check if user exists
	existingUser, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Hash password
	passwordHash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	// Set default role
	role := req.Role
	if role == "" {
		role = "user"
	}

	// Create user
	user := &User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Generate tokens
	return s.generateTokenPair(ctx, user)
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	valid, err := s.hasher.Verify(req.Password, user.PasswordHash)
	if err != nil || !valid {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokenPair(ctx, user)
}

// RefreshToken refreshes the access token
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	payload, err := s.tokenMaker.VerifyToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	if payload.TokenType != RefreshToken {
		return nil, ErrInvalidRefreshToken
	}

	// Check if token is revoked
	if s.tokenRepo != nil {
		revoked, err := s.tokenRepo.IsRefreshTokenRevoked(ctx, payload.ID)
		if err != nil || revoked {
			return nil, ErrInvalidRefreshToken
		}
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, payload.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Revoke old refresh token
	if s.tokenRepo != nil {
		_ = s.tokenRepo.RevokeRefreshToken(ctx, payload.ID)
	}

	return s.generateTokenPair(ctx, user)
}

// Logout invalidates the refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	payload, err := s.tokenMaker.VerifyToken(refreshToken)
	if err != nil {
		return nil
	}

	if s.tokenRepo != nil {
		return s.tokenRepo.RevokeRefreshToken(ctx, payload.ID)
	}

	return nil
}

// ValidateToken validates an access token and returns the payload
func (s *Service) ValidateToken(token string) (*TokenPayload, error) {
	return s.tokenMaker.VerifyToken(token)
}

// generateTokenPair generates access and refresh tokens
func (s *Service) generateTokenPair(ctx context.Context, user *User) (*AuthResponse, error) {
	accessToken, accessPayload, err := s.tokenMaker.CreateToken(
		user.ID,
		user.Email,
		user.Role,
		AccessToken,
		s.accessExpiry,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, refreshPayload, err := s.tokenMaker.CreateToken(
		user.ID,
		user.Email,
		user.Role,
		RefreshToken,
		s.refreshExpiry,
	)
	if err != nil {
		return nil, err
	}

	// Store refresh token
	if s.tokenRepo != nil {
		err = s.tokenRepo.StoreRefreshToken(ctx, refreshPayload.ID, user.ID, refreshPayload.ExpiresAt)
		if err != nil {
			return nil, err
		}
	}

	return &AuthResponse{
		User: &UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessPayload.ExpiresAt,
	}, nil
}
