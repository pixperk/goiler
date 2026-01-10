package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidToken = errors.New("token is invalid")
)

// TokenType represents the type of token
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// TokenPayload contains the token claims
type TokenPayload struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewTokenPayload creates a new token payload
func NewTokenPayload(userID uuid.UUID, email, role string, tokenType TokenType, duration time.Duration) (*TokenPayload, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &TokenPayload{
		ID:        tokenID,
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: tokenType,
		IssuedAt:  now,
		ExpiresAt: now.Add(duration),
	}, nil
}

// Valid checks if the token payload is valid
func (p *TokenPayload) Valid() error {
	if time.Now().After(p.ExpiresAt) {
		return ErrExpiredToken
	}
	return nil
}

// TokenMaker is the interface for token operations
type TokenMaker interface {
	// CreateToken creates a new token for a specific user
	CreateToken(userID uuid.UUID, email, role string, tokenType TokenType, duration time.Duration) (string, *TokenPayload, error)

	// VerifyToken checks if the token is valid and returns the payload
	VerifyToken(token string) (*TokenPayload, error)
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

// NewTokenMaker creates a new token maker based on the type
func NewTokenMaker(tokenType, secret string, symmetricKey []byte) (TokenMaker, error) {
	switch tokenType {
	case "jwt":
		return NewJWTMaker(secret)
	case "paseto":
		return NewPASETOMaker(symmetricKey)
	default:
		return NewJWTMaker(secret)
	}
}
