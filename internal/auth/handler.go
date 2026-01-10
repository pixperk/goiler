package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pixperk/goiler/pkg/response"
	"github.com/pixperk/goiler/pkg/validator"
)

// Handler handles HTTP requests for authentication
type Handler struct {
	service *Service
}

// NewHandler creates a new auth handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register handles user registration
// @Summary Register a new user
// @Description Create a new user account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 422 {object} response.Response
// @Router /api/v1/auth/register [post]
func (h *Handler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	result, err := h.service.Register(c.Request().Context(), &req)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			return response.Conflict(c, "User with this email already exists")
		}
		return response.InternalError(c, "Failed to create user")
	}

	return c.JSON(http.StatusCreated, response.Response{
		Success: true,
		Message: "User registered successfully",
		Data:    result,
	})
}

// Login handles user login
// @Summary Login
// @Description Authenticate user and get tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	result, err := h.service.Login(c.Request().Context(), &req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return response.Unauthorized(c, "Invalid email or password")
		}
		return response.InternalError(c, "Failed to authenticate")
	}

	return response.SuccessWithMessage(c, "Login successful", result)
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshToken handles token refresh
// @Summary Refresh token
// @Description Get a new access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/refresh [post]
func (h *Handler) RefreshToken(c echo.Context) error {
	var req RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	result, err := h.service.RefreshToken(c.Request().Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) || errors.Is(err, ErrExpiredToken) {
			return response.Unauthorized(c, "Invalid or expired refresh token")
		}
		return response.InternalError(c, "Failed to refresh token")
	}

	return response.SuccessWithMessage(c, "Token refreshed successfully", result)
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Logout handles user logout
// @Summary Logout
// @Description Invalidate refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Refresh token to invalidate"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /api/v1/auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	var req LogoutRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	_ = h.service.Logout(c.Request().Context(), req.RefreshToken)

	return response.SuccessWithMessage(c, "Logged out successfully", nil)
}

// AuthMiddleware returns middleware that validates access tokens
func (h *Handler) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return response.Unauthorized(c, "Missing authorization header")
			}

			const bearerPrefix = "Bearer "
			if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
				return response.Unauthorized(c, "Invalid authorization header format")
			}

			token := authHeader[len(bearerPrefix):]
			payload, err := h.service.ValidateToken(token)
			if err != nil {
				if errors.Is(err, ErrExpiredToken) {
					return response.Unauthorized(c, "Token has expired")
				}
				return response.Unauthorized(c, "Invalid token")
			}

			// Store user info in context
			c.Set("user_id", payload.UserID)
			c.Set("user_email", payload.Email)
			c.Set("user_role", payload.Role)
			c.Set("token_payload", payload)

			return next(c)
		}
	}
}

// GetCurrentUser returns the current authenticated user from context
func GetCurrentUser(c echo.Context) *TokenPayload {
	payload, ok := c.Get("token_payload").(*TokenPayload)
	if !ok {
		return nil
	}
	return payload
}
