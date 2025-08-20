package errors

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"go.uber.org/zap"
)

// ErrorResponse represents the standardized error response format
type ErrorResponse struct {
	Error     string                 `json:"error"`
	Code      ErrorCode              `json:"code"`
	Details   string                 `json:"details,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp string                 `json:"timestamp"`
	Retryable bool                   `json:"retryable,omitempty"`
}

// Handler provides centralized error handling for HTTP responses
type Handler struct {
	// Include sensitive details in responses (dev mode)
	IncludeSensitiveDetails bool
	// Log all errors (even handled ones)
	LogAllErrors bool
}

// NewHandler creates a new error handler with default configuration
func NewHandler() *Handler {
	return &Handler{
		IncludeSensitiveDetails: false, // Production default
		LogAllErrors:            true,
	}
}

// NewDevelopmentHandler creates an error handler for development with more verbose output
func NewDevelopmentHandler() *Handler {
	return &Handler{
		IncludeSensitiveDetails: true,
		LogAllErrors:            true,
	}
}

// HandleError processes an error and returns an appropriate HTTP response
func (h *Handler) HandleError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// Extract or create AppError
	appErr := h.toAppError(err)

	// Add request context if available
	requestID := c.Get("X-Request-ID")
	if requestID == "" {
		requestID = c.Get("X-Correlation-ID")
	}

	// Log the error with appropriate level based on severity
	h.logError(appErr, requestID, c)

	// Create response
	response := h.createErrorResponse(appErr, requestID)

	// Set appropriate headers
	c.Set("Content-Type", "application/json")

	// Add retry headers for retryable errors
	if appErr.IsRetryable() {
		c.Set("Retry-After", "30") // 30 seconds
	}

	return c.Status(appErr.HTTPStatus).JSON(response)
}

// toAppError converts any error to an AppError
func (h *Handler) toAppError(err error) *AppError {
	// If it's already an AppError, return as-is
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	// Check for common error patterns and classify them
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "context deadline exceeded") || strings.Contains(errStr, "timeout"):
		return NewErrorWithCause(ErrGitLabTimeout, "Request timeout", err)
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		return NewErrorWithCause(ErrServiceUnavailable, "Service unavailable", err)
	case strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401"):
		return NewErrorWithCause(ErrGitLabAuth, "Authentication failed", err)
	case strings.Contains(errStr, "not found") || strings.Contains(errStr, "404"):
		return NewErrorWithCause(ErrGitLabNotFound, "Resource not found", err)
	case strings.Contains(errStr, "yaml") || strings.Contains(errStr, "unmarshal"):
		return NewErrorWithCause(ErrYAMLParseFailed, "YAML parsing failed", err)
	case strings.Contains(errStr, "validation"):
		return NewErrorWithCause(ErrValidationFailed, "Validation failed", err)
	default:
		// Generic internal server error
		return NewErrorWithCause(ErrInternalServer, "Internal server error", err)
	}
}

// createErrorResponse creates a standardized error response
func (h *Handler) createErrorResponse(appErr *AppError, requestID string) ErrorResponse {
	response := ErrorResponse{
		Error:     appErr.Message,
		Code:      appErr.Code,
		RequestID: requestID,
		Timestamp: appErr.Timestamp.Format("2006-01-02T15:04:05Z"),
		Retryable: appErr.IsRetryable(),
	}

	// Include details based on configuration and error severity
	if h.shouldIncludeDetails(appErr) {
		response.Details = appErr.Details
		response.Context = appErr.Context
	}

	// Never include sensitive details in production
	if !h.IncludeSensitiveDetails {
		response = h.sanitizeResponse(response, appErr)
	}

	return response
}

// shouldIncludeDetails determines if error details should be included
func (h *Handler) shouldIncludeDetails(appErr *AppError) bool {
	// Always include details for client errors (4xx)
	if appErr.HTTPStatus >= 400 && appErr.HTTPStatus < 500 {
		return true
	}

	// Include for development or low severity errors
	return h.IncludeSensitiveDetails || appErr.Severity == SeverityLow
}

// sanitizeResponse removes sensitive information from error responses
func (h *Handler) sanitizeResponse(response ErrorResponse, appErr *AppError) ErrorResponse {
	// Map of sensitive error codes to safe messages
	safeMessages := map[ErrorCode]string{
		ErrGitLabAuth:         "Unable to access GitLab API",
		ErrTrillAuth:          "External service authentication failed",
		ErrInternalServer:     "Internal server error",
		ErrDatabaseConnection: "Database temporarily unavailable",
		ErrConfigurationError: "Service configuration error",
	}

	if safeMsg, exists := safeMessages[appErr.Code]; exists {
		response.Error = safeMsg
		response.Details = ""
		response.Context = nil
	}

	return response
}

// logError logs the error with appropriate context and level
func (h *Handler) logError(appErr *AppError, requestID string, c *fiber.Ctx) {
	// Build log fields
	fields := []zap.Field{
		zap.String("error_code", string(appErr.Code)),
		zap.String("severity", string(appErr.Severity)),
		zap.Int("http_status", appErr.HTTPStatus),
		zap.Bool("retryable", appErr.IsRetryable()),
	}

	// Add request context
	if requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if c != nil {
		fields = append(fields,
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("user_agent", c.Get("User-Agent")),
		)
	}

	// Add error context
	if appErr.Context != nil {
		for key, value := range appErr.Context {
			fields = append(fields, zap.Any(key, value))
		}
	}

	// Add underlying error if present
	if appErr.Cause != nil {
		fields = append(fields, zap.Error(appErr.Cause))
	}

	// Log with appropriate level based on severity
	switch appErr.Severity {
	case SeverityLow:
		logging.Info(appErr.Message, toInterfaceSlice(fields)...)
	case SeverityMedium:
		logging.Warn(appErr.Message, toInterfaceSlice(fields)...)
	case SeverityHigh, SeverityCritical:
		logging.Error(appErr.Message, toInterfaceSlice(fields)...)
	default:
		logging.Error(appErr.Message, toInterfaceSlice(fields)...)
	}
}

// FiberErrorHandler creates a Fiber-compatible error handler
func (h *Handler) FiberErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		return h.HandleError(c, err)
	}
}

// LogAndWrap logs an error and wraps it with additional context
func (h *Handler) LogAndWrap(err error, code ErrorCode, message string, context ...zap.Field) *AppError {
	if err == nil {
		return nil
	}

	appErr := NewErrorWithCause(code, message, err)

	// Log the error
	fields := append([]zap.Field{
		zap.String("error_code", string(code)),
		zap.Error(err),
	}, context...)

	logging.Error(message, toInterfaceSlice(fields)...)

	return appErr
}

// toInterfaceSlice converts zap.Field slice to interface{} slice
func toInterfaceSlice(fields []zap.Field) []interface{} {
	result := make([]interface{}, len(fields))
	for i, field := range fields {
		result[i] = field
	}
	return result
}

// RecoverAndHandle handles panics by converting them to errors
func (h *Handler) RecoverAndHandle(c *fiber.Ctx) {
	if r := recover(); r != nil {
		var err error

		// Convert panic to error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = NewError(ErrInternalServer, v)
		default:
			err = NewError(ErrInternalServer, "Unknown panic occurred")
		}

		// Log panic with stack trace
		logging.Error("Panic recovered",
			zap.Any("panic", r),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()),
		)

		// Handle as error
		_ = h.HandleError(c, err)
	}
}
