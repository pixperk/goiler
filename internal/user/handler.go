package user

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pixperk/goiler/internal/auth"
	"github.com/pixperk/goiler/pkg/response"
	"github.com/pixperk/goiler/pkg/validator"
)

// Handler handles HTTP requests for users
type Handler struct {
	service *Service
}

// NewHandler creates a new user handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetProfile returns the current user's profile
// @Summary Get user profile
// @Description Get the current authenticated user's profile
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /api/v1/users/me [get]
func (h *Handler) GetProfile(c echo.Context) error {
	payload := auth.GetCurrentUser(c)
	if payload == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	user, err := h.service.GetByID(c.Request().Context(), payload.UserID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, user)
}

// UpdateProfileRequest represents a profile update request
type UpdateProfileRequest struct {
	Email string `json:"email" validate:"omitempty,email"`
	Name  string `json:"name" validate:"omitempty,min=2,max=100"`
}

// UpdateProfile updates the current user's profile
// @Summary Update user profile
// @Description Update the current authenticated user's profile
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpdateProfileRequest true "Profile update"
// @Success 200 {object} UserResponse
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response
// @Router /api/v1/users/me [put]
func (h *Handler) UpdateProfile(c echo.Context) error {
	payload := auth.GetCurrentUser(c)
	if payload == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	var req UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	user, err := h.service.Update(c.Request().Context(), payload.UserID, &UpdateRequest{
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		return response.InternalError(c, "Failed to update profile")
	}

	return response.SuccessWithMessage(c, "Profile updated successfully", user)
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ChangePassword changes the current user's password
// @Summary Change password
// @Description Change the current authenticated user's password
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body ChangePasswordRequest true "Password change"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 422 {object} response.Response
// @Router /api/v1/users/me/password [put]
func (h *Handler) ChangePassword(c echo.Context) error {
	payload := auth.GetCurrentUser(c)
	if payload == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return response.ValidationError(c, validator.FormatErrors(err))
	}

	err := h.service.ChangePassword(c.Request().Context(), payload.UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == ErrInvalidPassword {
			return response.Unauthorized(c, "Current password is incorrect")
		}
		return response.InternalError(c, "Failed to change password")
	}

	return response.SuccessWithMessage(c, "Password changed successfully", nil)
}

// DeleteAccount deletes the current user's account
// @Summary Delete account
// @Description Delete the current authenticated user's account
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 204 "No Content"
// @Failure 401 {object} response.Response
// @Router /api/v1/users/me [delete]
func (h *Handler) DeleteAccount(c echo.Context) error {
	payload := auth.GetCurrentUser(c)
	if payload == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	err := h.service.Delete(c.Request().Context(), payload.UserID)
	if err != nil {
		return response.InternalError(c, "Failed to delete account")
	}

	return response.NoContent(c)
}

// GetUser returns a user by ID (admin only)
// @Summary Get user by ID
// @Description Get a user by their ID (admin only)
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} UserResponse
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /api/v1/users/{id} [get]
func (h *Handler) GetUser(c echo.Context) error {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	user, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, user)
}
