package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minSecretKeySize = 32

// JWTMaker implements TokenMaker interface using JWT
type JWTMaker struct {
	secretKey string
}

// JWTClaims represents JWT custom claims
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
}

// NewJWTMaker creates a new JWTMaker
func NewJWTMaker(secretKey string) (*JWTMaker, error) {
	if len(secretKey) < minSecretKeySize {
		return nil, fmt.Errorf("secret key must be at least %d characters", minSecretKeySize)
	}
	return &JWTMaker{secretKey: secretKey}, nil
}

// CreateToken creates a new JWT token
func (m *JWTMaker) CreateToken(userID uuid.UUID, email, role string, tokenType TokenType, duration time.Duration) (string, *TokenPayload, error) {
	payload, err := NewTokenPayload(userID, email, role, tokenType, duration)
	if err != nil {
		return "", nil, err
	}

	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        payload.ID.String(),
			Subject:   payload.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(payload.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(payload.ExpiresAt),
			Issuer:    "goiler",
		},
		UserID:    payload.UserID,
		Email:     payload.Email,
		Role:      payload.Role,
		TokenType: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		return "", nil, err
	}

	return tokenString, payload, nil
}

// VerifyToken verifies the JWT token and returns the payload
func (m *JWTMaker) VerifyToken(tokenString string) (*TokenPayload, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secretKey), nil
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	tokenID, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return &TokenPayload{
		ID:        tokenID,
		UserID:    claims.UserID,
		Email:     claims.Email,
		Role:      claims.Role,
		TokenType: claims.TokenType,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
