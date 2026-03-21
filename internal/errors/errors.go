// Package errors provides error handling utilities
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// AppError represents an application error
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
	HTTPStatus int    `json:"-"`
}

// Error implements error interface
func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error codes
const (
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeBadRequest     = "BAD_REQUEST"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout        = "TIMEOUT"
	ErrCodeRateLimit      = "RATE_LIMIT_EXCEEDED"
)

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		Message:    message,
		HTTPStatus: http.StatusNotFound,
	}
}

// NewBadRequestError creates a bad request error
func NewBadRequestError(message string, details ...any) *AppError {
	var d any
	if len(details) > 0 {
		d = details[0]
	}
	return &AppError{
		Code:       ErrCodeBadRequest,
		Message:    message,
		Details:    d,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string, err error) *AppError {
	details := ""
	if err != nil {
		details = err.Error()
	}
	return &AppError{
		Code:       ErrCodeInternal,
		Message:    message,
		Details:    details,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// NewServiceUnavailableError creates a service unavailable error
func NewServiceUnavailableError(service string) *AppError {
	return &AppError{
		Code:       ErrCodeServiceUnavailable,
		Message:    fmt.Sprintf("%s service is currently unavailable", service),
		HTTPStatus: http.StatusServiceUnavailable,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string) *AppError {
	return &AppError{
		Code:       ErrCodeTimeout,
		Message:    fmt.Sprintf("Operation '%s' timed out", operation),
		HTTPStatus: http.StatusGatewayTimeout,
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError() *AppError {
	return &AppError{
		Code:       ErrCodeRateLimit,
		Message:    "Rate limit exceeded. Please try again later.",
		HTTPStatus: http.StatusTooManyRequests,
	}
}

// WriteError writes an error response to HTTP response writer
func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if ae, ok := err.(*AppError); ok {
		appErr = ae
	} else {
		appErr = NewInternalError("An unexpected error occurred", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)

	response := map[string]any{
		"error": map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
		},
	}

	if appErr.Details != nil {
		response["error"].(map[string]any)["details"] = appErr.Details
	}

	json.NewEncoder(w).Encode(response)
}

// WrapError wraps a standard error into an AppError
func WrapError(err error, code string, message string) *AppError {
	httpStatus := http.StatusInternalServerError
	switch code {
	case ErrCodeNotFound:
		httpStatus = http.StatusNotFound
	case ErrCodeBadRequest:
		httpStatus = http.StatusBadRequest
	case ErrCodeUnauthorized:
		httpStatus = http.StatusUnauthorized
	case ErrCodeServiceUnavailable:
		httpStatus = http.StatusServiceUnavailable
	case ErrCodeTimeout:
		httpStatus = http.StatusGatewayTimeout
	case ErrCodeRateLimit:
		httpStatus = http.StatusTooManyRequests
	}

	return &AppError{
		Code:       code,
		Message:    message,
		Details:    err.Error(),
		HTTPStatus: httpStatus,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}

	// Service unavailable and rate limit errors are retryable
	return appErr.Code == ErrCodeServiceUnavailable ||
		appErr.Code == ErrCodeRateLimit ||
		appErr.Code == ErrCodeTimeout
}
