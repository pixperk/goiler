package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pixperk/goiler/internal/auth"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrEmailTaken      = errors.New("email already taken")
)

// User represents a user entity
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name,omitempty"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserResponse represents user data in API responses
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service handles user business logic
type Service struct {
	repo   Repository
	hasher auth.PasswordHasher
}

// NewService creates a new user service
func NewService(repo Repository, hasher auth.PasswordHasher) *Service {
	if hasher == nil {
		hasher = auth.DefaultPasswordHasher()
	}
	return &Service{
		repo:   repo,
		hasher: hasher,
	}
}

// GetByID retrieves a user by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// GetByEmail retrieves a user by email
func (s *Service) GetByEmail(ctx context.Context, email string) (*UserResponse, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// UpdateRequest represents a user update request
type UpdateRequest struct {
	Email string
	Name  string
}

// Update updates a user's profile
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *UpdateRequest) (*UserResponse, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Check if email is being changed and is already taken
	if req.Email != "" && req.Email != user.Email {
		existing, _ := s.repo.GetByEmail(ctx, req.Email)
		if existing != nil {
			return nil, ErrEmailTaken
		}
		user.Email = req.Email
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	user.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, id uuid.UUID, currentPassword, newPassword string) error {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ErrUserNotFound
	}

	// Verify current password
	valid, err := s.hasher.Verify(currentPassword, user.PasswordHash)
	if err != nil || !valid {
		return ErrInvalidPassword
	}

	// Hash new password
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	user.UpdatedAt = time.Now()

	return s.repo.Update(ctx, user)
}

// Delete deletes a user account
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List returns a paginated list of users
func (s *Service) List(ctx context.Context, page, perPage int) ([]*UserResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	users, total, err := s.repo.List(ctx, perPage, offset)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*UserResponse, len(users))
	for i, user := range users {
		responses[i] = &UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		}
	}

	return responses, total, nil
}
