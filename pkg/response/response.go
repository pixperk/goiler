package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response represents a standardized API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Meta contains pagination and other metadata
type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// Success returns a successful response
func Success(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMessage returns a successful response with a message
func SuccessWithMessage(c echo.Context, message string, data interface{}) error {
	return c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created returns a 201 created response
func Created(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

// NoContent returns a 204 no content response
func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// Paginated returns a paginated response
func Paginated(c echo.Context, data interface{}, page, perPage int, total int64) error {
	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

// Error returns an error response
func Error(c echo.Context, statusCode int, code, message string) error {
	return c.JSON(statusCode, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// ErrorWithDetails returns an error response with details
func ErrorWithDetails(c echo.Context, statusCode int, code, message string, details map[string]string) error {
	return c.JSON(statusCode, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// BadRequest returns a 400 bad request error
func BadRequest(c echo.Context, message string) error {
	return Error(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

// Unauthorized returns a 401 unauthorized error
func Unauthorized(c echo.Context, message string) error {
	return Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden returns a 403 forbidden error
func Forbidden(c echo.Context, message string) error {
	return Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound returns a 404 not found error
func NotFound(c echo.Context, message string) error {
	return Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// Conflict returns a 409 conflict error
func Conflict(c echo.Context, message string) error {
	return Error(c, http.StatusConflict, "CONFLICT", message)
}

// ValidationError returns a 422 validation error with details
func ValidationError(c echo.Context, details map[string]string) error {
	return ErrorWithDetails(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Validation failed", details)
}

// InternalError returns a 500 internal server error
func InternalError(c echo.Context, message string) error {
	return Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}
