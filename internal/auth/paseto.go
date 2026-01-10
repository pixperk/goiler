package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/o1egl/paseto"
)

const symmetricKeySize = 32

// PASETOMaker implements TokenMaker interface using PASETO v2
type PASETOMaker struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

// NewPASETOMaker creates a new PASETOMaker
func NewPASETOMaker(symmetricKey []byte) (*PASETOMaker, error) {
	if len(symmetricKey) != symmetricKeySize {
		return nil, fmt.Errorf("symmetric key must be exactly %d bytes", symmetricKeySize)
	}
	return &PASETOMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: symmetricKey,
	}, nil
}

// CreateToken creates a new PASETO token
func (m *PASETOMaker) CreateToken(userID uuid.UUID, email, role string, tokenType TokenType, duration time.Duration) (string, *TokenPayload, error) {
	payload, err := NewTokenPayload(userID, email, role, tokenType, duration)
	if err != nil {
		return "", nil, err
	}

	token, err := m.paseto.Encrypt(m.symmetricKey, payload, nil)
	if err != nil {
		return "", nil, err
	}

	return token, payload, nil
}

// VerifyToken verifies the PASETO token and returns the payload
func (m *PASETOMaker) VerifyToken(token string) (*TokenPayload, error) {
	payload := &TokenPayload{}

	err := m.paseto.Decrypt(token, m.symmetricKey, payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := payload.Valid(); err != nil {
		return nil, err
	}

	return payload, nil
}

// PASETOPayloadJSON is used for JSON serialization
type PASETOPayloadJSON struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MarshalJSON implements json.Marshaler
func (p *TokenPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(PASETOPayloadJSON{
		ID:        p.ID.String(),
		UserID:    p.UserID.String(),
		Email:     p.Email,
		Role:      p.Role,
		TokenType: p.TokenType,
		IssuedAt:  p.IssuedAt,
		ExpiresAt: p.ExpiresAt,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (p *TokenPayload) UnmarshalJSON(data []byte) error {
	var pj PASETOPayloadJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return err
	}

	id, err := uuid.Parse(pj.ID)
	if err != nil {
		return err
	}

	userID, err := uuid.Parse(pj.UserID)
	if err != nil {
		return err
	}

	p.ID = id
	p.UserID = userID
	p.Email = pj.Email
	p.Role = pj.Role
	p.TokenType = pj.TokenType
	p.IssuedAt = pj.IssuedAt
	p.ExpiresAt = pj.ExpiresAt

	return nil
}
