package apperror

import (
	"fmt"
	"net/http"
)

// Error is an operational error: an expected failure that maps to a specific
// HTTP status and machine-readable code.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// NotFound reports that a resource (e.g. "Product") was not found.
func NotFound(resource string, identifier any) *Error {
	return New(http.StatusNotFound, "NOT_FOUND",
		fmt.Sprintf("%s not found with identifier: %v", resource, identifier))
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

// Business reports a business rule violation.
func Business(message string) *Error {
	return New(http.StatusUnprocessableEntity, "BUSINESS_RULE_VIOLATION", message)
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError carries every invalid field found in a request.
type ValidationError struct {
	Details []FieldError
}

func (e *ValidationError) Error() string { return "Validation failed" }
