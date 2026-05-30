// Package apierr provides typed API errors that map to HTTP status codes.
// Each error carries a machine-readable Code and a human-readable Message,
// and is converted to the appropriate HTTP response by the error middleware.
package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code represents a machine-readable error code.
type Code string

const (
	CodeNotFound           Code = "NOT_FOUND"
	CodeUnauthorized       Code = "UNAUTHORIZED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeConflict           Code = "CONFLICT"
	CodeBadRequest         Code = "BAD_REQUEST"
	CodeUnprocessable      Code = "UNPROCESSABLE_ENTITY"
	CodeInternalServer     Code = "INTERNAL_SERVER_ERROR"
	CodeServiceUnavailable Code = "SERVICE_UNAVAILABLE"
)

// APIError is a typed error with an HTTP status code, error code, and message.
type APIError struct {
	Status  int    `json:"-"`
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NotFound returns a 404 error for a resource that was not found.
func NotFound(resource, id string) *APIError {
	return &APIError{
		Status:  http.StatusNotFound,
		Code:    CodeNotFound,
		Message: fmt.Sprintf("%s with id %q not found", resource, id),
	}
}

// NotFoundMsg returns a 404 error with a custom message.
func NotFoundMsg(msg string) *APIError {
	return &APIError{
		Status:  http.StatusNotFound,
		Code:    CodeNotFound,
		Message: msg,
	}
}

// Unauthorized returns a 401 error.
func Unauthorized(msg string) *APIError {
	return &APIError{
		Status:  http.StatusUnauthorized,
		Code:    CodeUnauthorized,
		Message: msg,
	}
}

// Forbidden returns a 403 error when the user lacks permission.
func Forbidden(msg string) *APIError {
	return &APIError{
		Status:  http.StatusForbidden,
		Code:    CodeForbidden,
		Message: msg,
	}
}

// Conflict returns a 409 error for resource conflicts.
func Conflict(msg string) *APIError {
	return &APIError{
		Status:  http.StatusConflict,
		Code:    CodeConflict,
		Message: msg,
	}
}

// BadRequest returns a 400 error for malformed requests.
func BadRequest(msg string) *APIError {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    CodeBadRequest,
		Message: msg,
	}
}

// BadRequestWithDetails returns a 400 error with structured validation details.
func BadRequestWithDetails(msg string, details any) *APIError {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    CodeBadRequest,
		Message: msg,
		Details: details,
	}
}

// Unprocessable returns a 422 error for business rule violations.
func Unprocessable(msg string) *APIError {
	return &APIError{
		Status:  http.StatusUnprocessableEntity,
		Code:    CodeUnprocessable,
		Message: msg,
	}
}

// Internal returns a 500 error for unexpected server errors.
func Internal(msg string) *APIError {
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternalServer,
		Message: msg,
	}
}

// ServiceUnavailable returns a 503 error when a dependency is unavailable.
func ServiceUnavailable(msg string) *APIError {
	return &APIError{
		Status:  http.StatusServiceUnavailable,
		Code:    CodeServiceUnavailable,
		Message: msg,
	}
}

// From converts a generic error to an APIError.
// If err is already an *APIError it is returned unchanged.
// Otherwise a 500 Internal error is returned.
func From(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return Internal(err.Error())
}

// IsNotFound reports whether err is a not-found API error.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == CodeNotFound
}

// IsUnauthorized reports whether err is an unauthorized API error.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == CodeUnauthorized
}
