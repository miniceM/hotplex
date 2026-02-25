package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return nil
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_SuccessAfterOneRetry(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		if attemptCount < 2 {
			return errors.New("temporary error: server unavailable")
		}
		return nil
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_SuccessAfterTwoRetries(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		if attemptCount < 3 {
			return errors.New("429 rate limit exceeded")
		}
		return nil
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_MaxAttemptsExceeded(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("always fails")
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if attemptCount != config.MaxAttempts {
		t.Errorf("Expected %d attempts, got %d", config.MaxAttempts, attemptCount)
	}
}

func TestRetryWithBackoff_NonRetryableErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"401 Unauthorized", errors.New("401 unauthorized")},
		{"403 Forbidden", errors.New("403 forbidden")},
		{"404 Not Found", errors.New("404 not found")},
		{"422 Unprocessable Entity", errors.New("422 validation error")},
		{"Invalid input", errors.New("invalid input")},
		{"Malformed request", errors.New("malformed request")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			fn := func() error {
				attemptCount++
				return tt.err
			}

			config := RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   10 * time.Millisecond,
				MaxDelay:    100 * time.Millisecond,
			}

			err := retryWithBackoff(context.Background(), config, fn)

			// Non-retryable errors should fail immediately without retry
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if attemptCount != 1 {
				t.Errorf("Expected 1 attempt for non-retryable error, got %d", attemptCount)
			}
		})
	}
}

func TestRetryWithBackoff_RetryableErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"429 Rate Limit", errors.New("429 rate limit")},
		{"500 Internal Server Error", errors.New("500 internal server error")},
		{"502 Bad Gateway", errors.New("502 bad gateway")},
		{"503 Service Unavailable", errors.New("503 service unavailable")},
		{"504 Gateway Timeout", errors.New("504 gateway timeout")},
		{"Timeout error", errors.New("connection timeout")},
		{"Temporary error", errors.New("temporary failure")},
		{"Connection refused", errors.New("connection refused")},
		{"Connection reset", errors.New("connection reset by peer")},
		{"I/O timeout", errors.New("i/o timeout")},
		{"Too many requests", errors.New("too many requests")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a function that succeeds after one retry
			attemptCount := 0
			fn := func() error {
				attemptCount++
				if attemptCount < 2 {
					return tt.err
				}
				return nil
			}

			config := RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   10 * time.Millisecond,
				MaxDelay:    100 * time.Millisecond,
			}

			err := retryWithBackoff(context.Background(), config, fn)

			if err != nil {
				t.Errorf("Expected no error after retry, got %v", err)
			}
			if attemptCount != 2 {
				t.Errorf("Expected 2 attempts (1 fail + 1 success), got %d", attemptCount)
			}
		})
	}
}

func TestRetryWithBackoff_ContextCanceled(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("500 server error")
	}

	config := RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
	}

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := retryWithBackoff(ctx, config, fn)

	if err == nil {
		t.Error("Expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
	// Should stop after the first attempt due to context cancellation
	if attemptCount > 1 {
		t.Errorf("Expected at most 1 attempt with cancelled context, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_ContextTimeout(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("500 server error")
	}

	config := RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
	}

	// Create a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := retryWithBackoff(ctx, config, fn)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error from timed out context, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}

	// Should not run too long due to context timeout
	if elapsed > 200*time.Millisecond {
		t.Errorf("Expected execution to respect context timeout, took %v", elapsed)
	}
}

func TestRetryWithBackoff_ExponentialBackoffTiming(t *testing.T) {
	tests := []struct {
		name           string
		baseDelay      time.Duration
		maxDelay       time.Duration
		expectedDelays []time.Duration
		maxAttempts    int
	}{
		{
			name:        "Standard exponential backoff 10ms",
			baseDelay:   10 * time.Millisecond,
			maxDelay:    100 * time.Millisecond,
			maxAttempts: 3,
			expectedDelays: []time.Duration{
				10 * time.Millisecond, // attempt 0 -> delay before attempt 1
				20 * time.Millisecond, // attempt 1 -> delay before attempt 2
				40 * time.Millisecond, // attempt 2 -> delay before attempt 3
			},
		},
		{
			name:        "Standard exponential backoff 100ms",
			baseDelay:   100 * time.Millisecond,
			maxDelay:    500 * time.Millisecond,
			maxAttempts: 4,
			expectedDelays: []time.Duration{
				100 * time.Millisecond,
				200 * time.Millisecond,
				400 * time.Millisecond,
				500 * time.Millisecond, // capped at maxDelay
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var attemptTimestamps []time.Time

			fn := func() error {
				mu.Lock()
				attemptTimestamps = append(attemptTimestamps, time.Now())
				mu.Unlock()
				return errors.New("retryable error")
			}

			config := RetryConfig{
				MaxAttempts: tt.maxAttempts,
				BaseDelay:   tt.baseDelay,
				MaxDelay:    tt.maxDelay,
			}

			_ = retryWithBackoff(context.Background(), config, fn)

			// Verify delays between attempts
			for i := 1; i < len(attemptTimestamps); i++ {
				actualDelay := attemptTimestamps[i].Sub(attemptTimestamps[i-1])
				expectedDelay := tt.expectedDelays[i-1]

				// Allow 50% tolerance for timing variations
				minExpected := expectedDelay / 2
				maxExpected := expectedDelay * 3 / 2

				if actualDelay < minExpected || actualDelay > maxExpected {
					t.Errorf("Attempt %d: delay %v not within acceptable range [%v, %v], expected ~%v",
						i, actualDelay, minExpected, maxExpected, expectedDelay)
				}
			}
		})
	}
}

func TestRetryWithBackoff_MaxDelayCap(t *testing.T) {
	var mu sync.Mutex
	var attemptTimestamps []time.Time

	fn := func() error {
		mu.Lock()
		attemptTimestamps = append(attemptTimestamps, time.Now())
		mu.Unlock()
		return errors.New("retryable error")
	}

	config := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond, // Cap at 200ms
	}

	_ = retryWithBackoff(context.Background(), config, fn)

	// Check that delays don't exceed maxDelay
	for i := 1; i < len(attemptTimestamps); i++ {
		actualDelay := attemptTimestamps[i].Sub(attemptTimestamps[i-1])
		if actualDelay > config.MaxDelay*110/100 { // 10% tolerance
			t.Errorf("Attempt %d: delay %v exceeded maxDelay %v", i, actualDelay, config.MaxDelay)
		}
	}
}

func TestIsRetryableError_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// Non-retryable errors
		{"401 Unauthorized", errors.New("401 unauthorized"), false},
		{"403 Forbidden", errors.New("403 forbidden"), false},
		{"404 Not Found", errors.New("404 not found"), false},
		{"422 Unprocessable", errors.New("422 unprocessable entity"), false},
		{"Unauthorized text", errors.New("unauthorized access"), false},
		{"Forbidden text", errors.New("access forbidden"), false},
		{"Not found text", errors.New("resource not found"), false},
		{"Validation error", errors.New("validation failed"), false},
		{"Invalid input", errors.New("invalid input"), false},
		{"Malformed request", errors.New("malformed request"), false},

		// Retryable errors
		{"429 Rate Limit", errors.New("429 rate limit exceeded"), true},
		{"500 Internal Server Error", errors.New("500 internal server error"), true},
		{"502 Bad Gateway", errors.New("502 bad gateway"), true},
		{"503 Service Unavailable", errors.New("503 service unavailable"), true},
		{"504 Gateway Timeout", errors.New("504 gateway timeout"), true},
		{"Timeout", errors.New("connection timeout"), true},
		{"Temporary", errors.New("temporary failure"), true},
		{"Unavailable", errors.New("service unavailable"), true},
		{"Rate limit text", errors.New("rate limit exceeded"), true},
		{"Too many requests", errors.New("too many requests"), true},
		{"Server error", errors.New("internal server error"), true},
		{"Connection refused", errors.New("connection refused"), true},
		{"Connection reset", errors.New("connection reset by peer"), true},
		{"I/O timeout", errors.New("i/o timeout"), true},

		// Default case - unknown errors should be retryable
		{"Unknown error", errors.New("some unknown error"), true},
		{"Empty error", errors.New(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableError_NilError(t *testing.T) {
	got := isRetryableError(nil)
	if got != false {
		t.Errorf("isRetryableError(nil) = %v, want false", got)
	}
}

func TestIsRetryableError_HTTPResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		// Non-retryable HTTP status codes
		{"400 Bad Request", 400, true},    // Default to retry
		{"401 Unauthorized", 401, false},  // Contains "unauthorized"
		{"403 Forbidden", 403, false},     // Contains "forbidden"
		{"404 Not Found", 404, false},     // Contains "not found"
		{"422 Unprocessable", 422, false}, // Contains "validation"

		// Retryable HTTP status codes
		{"429 Rate Limit", 429, true},
		{"500 Internal Server Error", 500, true},
		{"502 Bad Gateway", 502, true},
		{"503 Service Unavailable", 503, true},
		{"504 Gateway Timeout", 504, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an error with the HTTP status code
			err := fmt.Errorf("http %d: %s", tt.statusCode, http.StatusText(tt.statusCode))
			got := isRetryableError(err)
			if got != tt.want {
				t.Errorf("isRetryableError(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestRetryWithBackoff_AllRetryableThenSuccess(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		// Fail multiple times with different retryable errors
		errors := []error{
			errors.New("500 internal server error"),
			errors.New("503 service unavailable"),
			errors.New("429 rate limit"),
		}
		if attemptCount <= len(errors) {
			return errors[attemptCount-1]
		}
		return nil
	}

	config := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attemptCount != 4 { // 3 failures + 1 success
		t.Errorf("Expected 4 attempts, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_MixedRetryableAndNonRetryable(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		if attemptCount == 1 {
			return errors.New("500 server error") // Retryable
		}
		if attemptCount == 2 {
			return errors.New("401 unauthorized") // Non-retryable - should stop here
		}
		return nil // Should never reach here
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	// Should fail on non-retryable error
	if err == nil {
		t.Error("Expected error on non-retryable 401, got nil")
	}
	// Should have exactly 2 attempts (1 retryable failure + 1 non-retryable)
	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts (retryable + non-retryable), got %d", attemptCount)
	}
}

func TestRetryWithBackoff_OneAttemptConfig(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("always fails")
	}

	config := RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_ZeroMaxAttempts(t *testing.T) {
	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("always fails")
	}

	config := RetryConfig{
		MaxAttempts: 0,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	err := retryWithBackoff(context.Background(), config, fn)

	// With 0 max attempts, the loop never runs and returns nil (lastErr remains nil)
	// This is the actual behavior - no attempts made, no error returned
	if err != nil {
		t.Errorf("Expected nil (no attempts made), got %v", err)
	}
	if attemptCount != 0 {
		t.Errorf("Expected 0 attempts with MaxAttempts=0, got %d", attemptCount)
	}
}

// Benchmark tests

func BenchmarkRetryWithBackoff_Success(b *testing.B) {
	fn := func() error {
		return nil
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryWithBackoff(context.Background(), config, fn)
	}
}

func BenchmarkRetryWithBackoff_MaxRetries(b *testing.B) {
	fn := func() error {
		return errors.New("500 server error")
	}

	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryWithBackoff(context.Background(), config, fn)
	}
}

func BenchmarkIsRetryableError(b *testing.B) {
	errors := []error{
		errors.New("401 unauthorized"),
		errors.New("500 internal server error"),
		errors.New("429 rate limit"),
		errors.New("timeout"),
		errors.New("some unknown error"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range errors {
			_ = isRetryableError(err)
		}
	}
}
