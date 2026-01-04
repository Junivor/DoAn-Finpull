package http

import (
	"fmt"
	"net/http"
)

// AppError represents application-level error with HTTP status.
type AppError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Field   string                 `json:"field,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Status  int                    `json:"-"`
	Err     error                  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns underlying error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error.
func NewAppError(code, field, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Field:   field,
		Status:  status,
		Params:  make(map[string]interface{}),
	}
}

// WithParams sets error params.
func (e *AppError) WithParams(params map[string]interface{}) *AppError {
	e.Params = params
	return e
}

// WithParam sets a single error param.
func (e *AppError) WithParam(key string, value interface{}) *AppError {
	if e.Params == nil {
		e.Params = make(map[string]interface{})
	}
	e.Params[key] = value
	return e
}

// WithError wraps an underlying error.
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	return e
}

// NotFoundError creates a 404 error.
func NotFoundError(message string) *AppError {
	return NewAppError("ERR_NOT_FOUND", "", message, http.StatusNotFound)
}

// NotFoundErrorf creates a 404 error with formatting.
func NotFoundErrorf(format string, a ...interface{}) *AppError {
	return NotFoundError(fmt.Sprintf(format, a...))
}

// BadRequestError creates a 400 error.
func BadRequestError(message string) *AppError {
	return NewAppError("ERR_BAD_REQUEST", "", message, http.StatusBadRequest)
}

// BadRequestErrorf creates a 400 error with formatting.
func BadRequestErrorf(format string, a ...interface{}) *AppError {
	return BadRequestError(fmt.Sprintf(format, a...))
}

// UnauthorizedError creates a 401 error.
func UnauthorizedError(message string) *AppError {
	return NewAppError("ERR_UNAUTHORIZED", "", message, http.StatusUnauthorized)
}

// ForbiddenError creates a 403 error.
func ForbiddenError(message string) *AppError {
	return NewAppError("ERR_FORBIDDEN", "", message, http.StatusForbidden)
}

// InternalError creates a 500 error.
func InternalError(message string) *AppError {
	return NewAppError("ERR_INTERNAL", "", message, http.StatusInternalServerError)
}

// InternalErrorf creates a 500 error with formatting.
func InternalErrorf(format string, a ...interface{}) *AppError {
	return InternalError(fmt.Sprintf(format, a...))
}
