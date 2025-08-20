package errors

import (
	"context"
	"math"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"go.uber.org/zap"
)

// RetryableFunc represents a function that can be retried
type RetryableFunc func() error

// RetryConfig defines retry configuration for operations
type RetryConfig struct {
	MaxAttempts      int
	InitialDelay     time.Duration
	MaxDelay         time.Duration
	ExponentialBase  float64
	Jitter           bool
	RetryCondition   func(error) bool
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		MaxDelay:         time.Minute,
		ExponentialBase:  2.0,
		Jitter:           true,
		RetryCondition:   DefaultRetryCondition,
	}
}

// GitLabRetryConfig returns retry configuration optimized for GitLab API calls
func GitLabRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second * 2,
		MaxDelay:         time.Second * 30,
		ExponentialBase:  2.0,
		Jitter:           true,
		RetryCondition:   GitLabRetryCondition,
	}
}

// TrillRetryConfig returns retry configuration optimized for Trill service calls
func TrillRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      5,
		InitialDelay:     time.Second * 5,
		MaxDelay:         time.Minute * 2,
		ExponentialBase:  1.5,
		Jitter:           true,
		RetryCondition:   TrillRetryCondition,
	}
}

// DefaultRetryCondition determines if an error should be retried
func DefaultRetryCondition(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's an AppError with retry policy
	if appErr, ok := err.(*AppError); ok {
		return appErr.IsRetryable()
	}

	// Fallback to checking error message for common retryable patterns
	return IsTemporaryError(err)
}

// GitLabRetryCondition determines if a GitLab API error should be retried
func GitLabRetryCondition(err error) bool {
	if err == nil {
		return false
	}

	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrGitLabRateLimit, ErrGitLabTimeout, ErrGitLabAPIFailed:
			return true
		case ErrGitLabAuth, ErrGitLabNotFound:
			return false // Don't retry auth or not found errors
		default:
			return appErr.IsRetryable()
		}
	}

	return IsTemporaryError(err)
}

// TrillRetryCondition determines if a Trill service error should be retried
func TrillRetryCondition(err error) bool {
	if err == nil {
		return false
	}

	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrTrillServiceFailed, ErrTrillTimeout:
			return true
		case ErrTrillAuth:
			return false // Don't retry auth errors
		default:
			return appErr.IsRetryable()
		}
	}

	return IsTemporaryError(err)
}

// IsTemporaryError checks if an error appears to be temporary based on its message
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	temporaryPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"temporary failure",
		"service unavailable",
		"too many requests",
		"rate limit",
		"502 bad gateway",
		"503 service unavailable",
		"504 gateway timeout",
	}

	for _, pattern := range temporaryPatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || len(substr) == 0 || 
		    (len(s) > len(substr) && (s[0:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr || 
		     indexOfSubstring(s, substr) != -1)))
}

// indexOfSubstring finds the index of a substring (case-insensitive)
func indexOfSubstring(s, substr string) int {
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// RetryWithContext executes a function with retry logic and context support
func RetryWithContext(ctx context.Context, fn RetryableFunc, config RetryConfig) error {
	var lastErr error
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		if ctx.Err() != nil {
			return NewErrorWithCause(ErrServiceUnavailable, "Operation cancelled", ctx.Err())
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 {
				logging.Info("Operation succeeded after retry",
					zap.Int("attempt", attempt),
					zap.Int("total_attempts", config.MaxAttempts),
				)
			}
			return nil
		}

		lastErr = err

		// Check if we should retry this error
		if !config.RetryCondition(err) {
			logging.Info("Error not retryable, giving up",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			return err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, config)
		
		logging.Warn("Operation failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Duration("retry_delay", delay),
		)

		// Wait before retry, but respect context cancellation
		select {
		case <-ctx.Done():
			return NewErrorWithCause(ErrServiceUnavailable, "Operation cancelled during retry", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All attempts failed
	logging.Error("All retry attempts failed",
		zap.Error(lastErr),
		zap.Int("total_attempts", config.MaxAttempts),
	)

	return NewErrorWithCause(ErrInternalServer, "Operation failed after all retries", lastErr)
}

// Retry executes a function with retry logic (no context)
func Retry(fn RetryableFunc, config RetryConfig) error {
	return RetryWithContext(context.Background(), fn, config)
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	if attempt <= 1 {
		return config.InitialDelay
	}

	// Calculate exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.ExponentialBase, float64(attempt-1))
	
	// Apply maximum delay limit
	if time.Duration(delay) > config.MaxDelay {
		delay = float64(config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if config.Jitter && delay > 0 {
		// Add random jitter of ±25%
		jitterAmount := delay * 0.25
		delay = delay + (jitterAmount * (2*pseudoRandom() - 1))
		
		// Ensure delay is not negative
		if delay < 0 {
			delay = float64(config.InitialDelay)
		}
	}

	return time.Duration(delay)
}

// pseudoRandom generates a simple pseudo-random number between 0 and 1
// This is a simple implementation to avoid importing crypto/rand
func pseudoRandom() float64 {
	// Use current time nanoseconds for simple randomness
	now := time.Now().UnixNano()
	return float64(now%1000) / 1000.0
}

// RetryableOperation wraps an operation with automatic retry logic
type RetryableOperation struct {
	Name   string
	Config RetryConfig
}

// NewRetryableOperation creates a new retryable operation with default config
func NewRetryableOperation(name string) *RetryableOperation {
	return &RetryableOperation{
		Name:   name,
		Config: DefaultRetryConfig(),
	}
}

// NewGitLabOperation creates a retryable operation optimized for GitLab API calls
func NewGitLabOperation(name string) *RetryableOperation {
	return &RetryableOperation{
		Name:   name,
		Config: GitLabRetryConfig(),
	}
}

// NewTrillOperation creates a retryable operation optimized for Trill service calls
func NewTrillOperation(name string) *RetryableOperation {
	return &RetryableOperation{
		Name:   name,
		Config: TrillRetryConfig(),
	}
}

// Execute runs the operation with retry logic
func (op *RetryableOperation) Execute(ctx context.Context, fn RetryableFunc) error {
			logging.Info("Starting retryable operation",
			zap.String("operation", op.Name),
			zap.Int("max_attempts", op.Config.MaxAttempts),
		)

	err := RetryWithContext(ctx, fn, op.Config)
	
	if err != nil {
		logging.Error("Retryable operation failed",
			zap.String("operation", op.Name),
			zap.Error(err),
		)
	} else {
		logging.Info("Retryable operation succeeded",
			zap.String("operation", op.Name),
		)
	}

	return err
}

// ExecuteWithTimeout runs the operation with retry logic and a timeout
func (op *RetryableOperation) ExecuteWithTimeout(timeout time.Duration, fn RetryableFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	return op.Execute(ctx, fn)
}
