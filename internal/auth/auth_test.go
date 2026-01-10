package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- Password Hashing Tests ---

func TestArgon2Hasher_Hash(t *testing.T) {
	hasher := NewArgon2Hasher(nil)
	password := "SecureP@ssw0rd!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash should not be empty")
	}

	if hash == password {
		t.Fatal("Hash should not equal password")
	}
}

func TestArgon2Hasher_Verify(t *testing.T) {
	hasher := NewArgon2Hasher(nil)
	password := "SecureP@ssw0rd!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Test correct password
	valid, err := hasher.Verify(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}
	if !valid {
		t.Fatal("Password should be valid")
	}

	// Test incorrect password
	valid, err = hasher.Verify("WrongPassword", hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}
	if valid {
		t.Fatal("Wrong password should not be valid")
	}
}

func TestBcryptHasher_Hash(t *testing.T) {
	hasher := NewBcryptHasher(10)
	password := "SecureP@ssw0rd!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash should not be empty")
	}
}

func TestBcryptHasher_Verify(t *testing.T) {
	hasher := NewBcryptHasher(10)
	password := "SecureP@ssw0rd!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Test correct password
	valid, err := hasher.Verify(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}
	if !valid {
		t.Fatal("Password should be valid")
	}

	// Test incorrect password
	valid, err = hasher.Verify("WrongPassword", hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}
	if valid {
		t.Fatal("Wrong password should not be valid")
	}
}

// --- JWT Tests ---

func TestJWTMaker_CreateToken(t *testing.T) {
	secret := "12345678901234567890123456789012" // 32 chars
	maker, err := NewJWTMaker(secret)
	if err != nil {
		t.Fatalf("Failed to create JWT maker: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := time.Hour

	token, payload, err := maker.CreateToken(userID, email, role, AccessToken, duration)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	if token == "" {
		t.Fatal("Token should not be empty")
	}

	if payload.UserID != userID {
		t.Errorf("UserID mismatch: got %v, want %v", payload.UserID, userID)
	}

	if payload.Email != email {
		t.Errorf("Email mismatch: got %v, want %v", payload.Email, email)
	}

	if payload.Role != role {
		t.Errorf("Role mismatch: got %v, want %v", payload.Role, role)
	}
}

func TestJWTMaker_VerifyToken(t *testing.T) {
	secret := "12345678901234567890123456789012"
	maker, err := NewJWTMaker(secret)
	if err != nil {
		t.Fatalf("Failed to create JWT maker: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := time.Hour

	token, _, err := maker.CreateToken(userID, email, role, AccessToken, duration)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	payload, err := maker.VerifyToken(token)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}

	if payload.UserID != userID {
		t.Errorf("UserID mismatch: got %v, want %v", payload.UserID, userID)
	}
}

func TestJWTMaker_ExpiredToken(t *testing.T) {
	secret := "12345678901234567890123456789012"
	maker, err := NewJWTMaker(secret)
	if err != nil {
		t.Fatalf("Failed to create JWT maker: %v", err)
	}

	userID := uuid.New()
	// Create an expired token
	token, _, err := maker.CreateToken(userID, "test@example.com", "user", AccessToken, -time.Hour)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	_, err = maker.VerifyToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got: %v", err)
	}
}

func TestJWTMaker_InvalidToken(t *testing.T) {
	secret := "12345678901234567890123456789012"
	maker, err := NewJWTMaker(secret)
	if err != nil {
		t.Fatalf("Failed to create JWT maker: %v", err)
	}

	_, err = maker.VerifyToken("invalid.token.here")
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got: %v", err)
	}
}

func TestJWTMaker_ShortSecret(t *testing.T) {
	_, err := NewJWTMaker("short")
	if err == nil {
		t.Fatal("Expected error for short secret key")
	}
}

// --- PASETO Tests ---

func TestPASETOMaker_CreateToken(t *testing.T) {
	symmetricKey := []byte("12345678901234567890123456789012") // 32 bytes
	maker, err := NewPASETOMaker(symmetricKey)
	if err != nil {
		t.Fatalf("Failed to create PASETO maker: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := time.Hour

	token, payload, err := maker.CreateToken(userID, email, role, AccessToken, duration)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	if token == "" {
		t.Fatal("Token should not be empty")
	}

	if payload.UserID != userID {
		t.Errorf("UserID mismatch: got %v, want %v", payload.UserID, userID)
	}
}

func TestPASETOMaker_VerifyToken(t *testing.T) {
	symmetricKey := []byte("12345678901234567890123456789012")
	maker, err := NewPASETOMaker(symmetricKey)
	if err != nil {
		t.Fatalf("Failed to create PASETO maker: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	role := "admin"
	duration := time.Hour

	token, _, err := maker.CreateToken(userID, email, role, RefreshToken, duration)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	payload, err := maker.VerifyToken(token)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}

	if payload.UserID != userID {
		t.Errorf("UserID mismatch: got %v, want %v", payload.UserID, userID)
	}

	if payload.TokenType != RefreshToken {
		t.Errorf("TokenType mismatch: got %v, want %v", payload.TokenType, RefreshToken)
	}
}

func TestPASETOMaker_ExpiredToken(t *testing.T) {
	symmetricKey := []byte("12345678901234567890123456789012")
	maker, err := NewPASETOMaker(symmetricKey)
	if err != nil {
		t.Fatalf("Failed to create PASETO maker: %v", err)
	}

	userID := uuid.New()
	token, _, err := maker.CreateToken(userID, "test@example.com", "user", AccessToken, -time.Hour)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	_, err = maker.VerifyToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got: %v", err)
	}
}

func TestPASETOMaker_InvalidKeySize(t *testing.T) {
	_, err := NewPASETOMaker([]byte("short"))
	if err == nil {
		t.Fatal("Expected error for invalid key size")
	}
}

// --- Token Payload Tests ---

func TestTokenPayload_Valid(t *testing.T) {
	payload := &TokenPayload{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Email:     "test@example.com",
		Role:      "user",
		TokenType: AccessToken,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := payload.Valid(); err != nil {
		t.Errorf("Token should be valid: %v", err)
	}
}

func TestTokenPayload_Expired(t *testing.T) {
	payload := &TokenPayload{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Email:     "test@example.com",
		Role:      "user",
		TokenType: AccessToken,
		IssuedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if err := payload.Valid(); err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got: %v", err)
	}
}

// --- Benchmark Tests ---

func BenchmarkArgon2Hash(b *testing.B) {
	hasher := NewArgon2Hasher(nil)
	password := "SecureP@ssw0rd!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.Hash(password)
	}
}

func BenchmarkArgon2Verify(b *testing.B) {
	hasher := NewArgon2Hasher(nil)
	password := "SecureP@ssw0rd!"
	hash, _ := hasher.Hash(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.Verify(password, hash)
	}
}

func BenchmarkBcryptHash(b *testing.B) {
	hasher := NewBcryptHasher(10)
	password := "SecureP@ssw0rd!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.Hash(password)
	}
}

func BenchmarkJWTCreateToken(b *testing.B) {
	maker, _ := NewJWTMaker("12345678901234567890123456789012")
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = maker.CreateToken(userID, "test@example.com", "user", AccessToken, time.Hour)
	}
}

func BenchmarkJWTVerifyToken(b *testing.B) {
	maker, _ := NewJWTMaker("12345678901234567890123456789012")
	userID := uuid.New()
	token, _, _ := maker.CreateToken(userID, "test@example.com", "user", AccessToken, time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = maker.VerifyToken(token)
	}
}

func BenchmarkPASETOCreateToken(b *testing.B) {
	maker, _ := NewPASETOMaker([]byte("12345678901234567890123456789012"))
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = maker.CreateToken(userID, "test@example.com", "user", AccessToken, time.Hour)
	}
}

func BenchmarkPASETOVerifyToken(b *testing.B) {
	maker, _ := NewPASETOMaker([]byte("12345678901234567890123456789012"))
	userID := uuid.New()
	token, _, _ := maker.CreateToken(userID, "test@example.com", "user", AccessToken, time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = maker.VerifyToken(token)
	}
}
