package errors

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents a specific error type for categorization and metrics
type ErrorCode string

const (
	// Validation errors
	ErrInvalidInput       ErrorCode = "INVALID_INPUT"
	ErrMissingField       ErrorCode = "MISSING_FIELD"
	ErrInvalidFormat      ErrorCode = "INVALID_FORMAT"
	ErrValidationFailed   ErrorCode = "VALIDATION_FAILED"
	
	// GitLab API errors
	ErrGitLabAPIFailed    ErrorCode = "GITLAB_API_FAILED"
	ErrGitLabAuth         ErrorCode = "GITLAB_AUTH_FAILED"
	ErrGitLabNotFound     ErrorCode = "GITLAB_NOT_FOUND"
	ErrGitLabRateLimit    ErrorCode = "GITLAB_RATE_LIMIT"
	ErrGitLabTimeout      ErrorCode = "GITLAB_TIMEOUT"
	
	// Rule evaluation errors
	ErrRuleEvaluation     ErrorCode = "RULE_EVALUATION_FAILED"
	ErrRuleNotFound       ErrorCode = "RULE_NOT_FOUND"
	ErrRuleConfig         ErrorCode = "RULE_CONFIG_ERROR"
	
	// File processing errors
	ErrFileProcessing     ErrorCode = "FILE_PROCESSING_FAILED"
	ErrFileNotFound       ErrorCode = "FILE_NOT_FOUND"
	ErrFileReadFailed     ErrorCode = "FILE_READ_FAILED"
	ErrYAMLParseFailed    ErrorCode = "YAML_PARSE_FAILED"
	
	// External service errors
	ErrTrillServiceFailed ErrorCode = "TRILL_SERVICE_FAILED"
	ErrTrillTimeout       ErrorCode = "TRILL_TIMEOUT"
	ErrTrillAuth          ErrorCode = "TRILL_AUTH_FAILED"
	
	// System errors
	ErrDatabaseConnection ErrorCode = "DATABASE_CONNECTION_FAILED"
	ErrConfigurationError ErrorCode = "CONFIGURATION_ERROR"
	ErrInternalServer     ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// ErrorSeverity indicates the severity level of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "LOW"
	SeverityMedium   ErrorSeverity = "MEDIUM"
	SeverityHigh     ErrorSeverity = "HIGH"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// RetryPolicy defines whether an error is retryable and retry configuration
type RetryPolicy struct {
	Retryable     bool          `json:"retryable"`
	MaxRetries    int           `json:"max_retries,omitempty"`
	BackoffDelay  time.Duration `json:"backoff_delay,omitempty"`
	ExponentialBO bool          `json:"exponential_backoff,omitempty"`
}

// AppError represents a structured application error with rich context
type AppError struct {
	Code       ErrorCode                `json:"code"`
	Message    string                   `json:"message"`
	Details    string                   `json:"details,omitempty"`
	Severity   ErrorSeverity            `json:"severity"`
	HTTPStatus int                      `json:"http_status"`
	Context    map[string]interface{}   `json:"context,omitempty"`
	Timestamp  time.Time                `json:"timestamp"`
	Retry      RetryPolicy              `json:"retry_policy"`
	Cause      error                    `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for Go 1.13+ error unwrapping
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext adds contextual information to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithMRContext adds MR-specific context to the error
func (e *AppError) WithMRContext(projectID, mrIID int) *AppError {
	return e.WithContext("project_id", projectID).WithContext("mr_iid", mrIID)
}

// IsRetryable returns whether this error should be retried
func (e *AppError) IsRetryable() bool {
	return e.Retry.Retryable
}

// IsTemporary indicates if this is a temporary error that might resolve itself
func (e *AppError) IsTemporary() bool {
	return e.Retry.Retryable || e.Code == ErrGitLabTimeout || e.Code == ErrTrillTimeout
}

// NewError creates a new AppError with the given code and message
func NewError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Severity:   SeverityMedium,
		HTTPStatus: getDefaultHTTPStatus(code),
		Timestamp:  time.Now(),
		Retry:      getDefaultRetryPolicy(code),
	}
}

// NewErrorWithCause creates a new AppError wrapping an existing error
func NewErrorWithCause(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Severity:   SeverityMedium,
		HTTPStatus: getDefaultHTTPStatus(code),
		Timestamp:  time.Now(),
		Retry:      getDefaultRetryPolicy(code),
		Cause:      cause,
	}
}

// NewValidationError creates a validation error with details
func NewValidationError(field, reason string) *AppError {
	return &AppError{
		Code:       ErrValidationFailed,
		Message:    fmt.Sprintf("Validation failed for field '%s'", field),
		Details:    reason,
		Severity:   SeverityLow,
		HTTPStatus: http.StatusBadRequest,
		Timestamp:  time.Now(),
		Retry:      RetryPolicy{Retryable: false},
	}
}

// NewGitLabError creates a GitLab API specific error
func NewGitLabError(operation string, statusCode int, responseBody string) *AppError {
	var code ErrorCode
	var severity ErrorSeverity
	var retryable bool

	switch statusCode {
	case 401:
		code = ErrGitLabAuth
		severity = SeverityHigh
		retryable = false
	case 404:
		code = ErrGitLabNotFound
		severity = SeverityMedium
		retryable = false
	case 429:
		code = ErrGitLabRateLimit
		severity = SeverityMedium
		retryable = true
	case 500, 502, 503, 504:
		code = ErrGitLabAPIFailed
		severity = SeverityHigh
		retryable = true
	default:
		code = ErrGitLabAPIFailed
		severity = SeverityMedium
		retryable = false
	}

	return &AppError{
		Code:       code,
		Message:    fmt.Sprintf("GitLab API %s failed", operation),
		Details:    fmt.Sprintf("HTTP %d: %s", statusCode, responseBody),
		Severity:   severity,
		HTTPStatus: getHTTPStatusForGitLabError(statusCode),
		Timestamp:  time.Now(),
		Retry: RetryPolicy{
			Retryable:     retryable,
			MaxRetries:    3,
			BackoffDelay:  time.Second * 2,
			ExponentialBO: true,
		},
	}
}

// NewTrillError creates a Trill service specific error
func NewTrillError(operation string, cause error) *AppError {
	return &AppError{
		Code:       ErrTrillServiceFailed,
		Message:    fmt.Sprintf("Trill %s failed", operation),
		Details:    cause.Error(),
		Severity:   SeverityHigh,
		HTTPStatus: http.StatusServiceUnavailable,
		Timestamp:  time.Now(),
		Retry: RetryPolicy{
			Retryable:     true,
			MaxRetries:    3,
			BackoffDelay:  time.Second * 5,
			ExponentialBO: true,
		},
		Cause: cause,
	}
}

// getDefaultHTTPStatus returns the default HTTP status code for an error code
func getDefaultHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrInvalidInput, ErrMissingField, ErrInvalidFormat, ErrValidationFailed:
		return http.StatusBadRequest
	case ErrGitLabAuth, ErrTrillAuth:
		return http.StatusUnauthorized
	case ErrGitLabNotFound, ErrFileNotFound, ErrRuleNotFound:
		return http.StatusNotFound
	case ErrGitLabRateLimit:
		return http.StatusTooManyRequests
	case ErrTrillServiceFailed, ErrServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// getHTTPStatusForGitLabError maps GitLab API errors to appropriate HTTP status
func getHTTPStatusForGitLabError(gitlabStatus int) int {
	switch gitlabStatus {
	case 401:
		return http.StatusServiceUnavailable // Don't expose auth issues
	case 404:
		return http.StatusBadRequest // Invalid MR/project
	case 429:
		return http.StatusServiceUnavailable // Rate limited
	default:
		return http.StatusInternalServerError
	}
}

// getDefaultRetryPolicy returns the default retry policy for an error code
func getDefaultRetryPolicy(code ErrorCode) RetryPolicy {
	switch code {
	case ErrGitLabTimeout, ErrTrillTimeout, ErrGitLabRateLimit:
		return RetryPolicy{
			Retryable:     true,
			MaxRetries:    3,
			BackoffDelay:  time.Second * 2,
			ExponentialBO: true,
		}
	case ErrGitLabAPIFailed, ErrTrillServiceFailed:
		return RetryPolicy{
			Retryable:     true,
			MaxRetries:    2,
			BackoffDelay:  time.Second * 1,
			ExponentialBO: false,
		}
	default:
		return RetryPolicy{Retryable: false}
	}
}
