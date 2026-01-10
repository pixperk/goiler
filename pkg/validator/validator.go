package validator

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CustomValidator wraps the validator.Validate
type CustomValidator struct {
	validator *validator.Validate
}

// New creates a new custom validator
func New() *CustomValidator {
	v := validator.New()

	// Use JSON tag names in error messages
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Register custom validations here
	_ = v.RegisterValidation("password", validatePassword)

	return &CustomValidator{validator: v}
}

// Validate validates the given struct
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// FormatErrors formats validation errors into a map
func FormatErrors(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			errors[field] = formatErrorMessage(e)
		}
	}

	return errors
}

// formatErrorMessage returns a human-readable error message
func formatErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Must be at least " + e.Param() + " characters"
	case "max":
		return "Must be at most " + e.Param() + " characters"
	case "eqfield":
		return "Must match " + e.Param()
	case "password":
		return "Password must be at least 8 characters with uppercase, lowercase, number, and special character"
	case "uuid":
		return "Must be a valid UUID"
	case "url":
		return "Must be a valid URL"
	case "oneof":
		return "Must be one of: " + e.Param()
	default:
		return "Invalid value"
	}
}

// validatePassword validates password strength
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?", char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}
