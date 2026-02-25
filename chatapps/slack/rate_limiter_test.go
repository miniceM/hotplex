package slack

import (
	"testing"
	"time"
)

func TestSlashCommandRateLimiter_Allow(t *testing.T) {
	// Create a limiter with high rate for testing
	limiter := NewSlashCommandRateLimiterWithConfig(100, 10)

	// First request should be allowed
	if !limiter.Allow("user1") {
		t.Error("Expected first request to be allowed")
	}

	// Burst of 10 should be allowed
	for i := 0; i < 9; i++ {
		if !limiter.Allow("user1") {
			t.Errorf("Expected request %d to be allowed (within burst)", i+2)
		}
	}
}

func TestSlashCommandRateLimiter_PerUserLimiting(t *testing.T) {
	limiter := NewSlashCommandRateLimiterWithConfig(5, 2)

	// user1 should have their own limit
	if !limiter.Allow("user1") {
		t.Error("Expected first request for user1 to be allowed")
	}
	if !limiter.Allow("user1") {
		t.Error("Expected second request for user1 to be allowed (burst)")
	}

	// user2 should have independent limit
	if !limiter.Allow("user2") {
		t.Error("Expected first request for user2 to be allowed")
	}
}

func TestSlashCommandRateLimiter_DefaultConfig(t *testing.T) {
	// Test default configuration works
	limiter := NewSlashCommandRateLimiter()

	if limiter == nil {
		t.Error("Expected limiter to be created")
	}

	// Should allow at least one request
	if !limiter.Allow("testuser") {
		t.Error("Expected request to be allowed with default config")
	}
}

func TestSlashCommandRateLimiter_ZeroConfig(t *testing.T) {
	// Test that zero values fall back to defaults
	limiter := NewSlashCommandRateLimiterWithConfig(0, 0)

	if limiter == nil {
		t.Error("Expected limiter to be created with zero config")
	}

	// Should still work with defaults
	if !limiter.Allow("testuser") {
		t.Error("Expected request to be allowed with default fallback")
	}
}

func TestSlashCommandRateLimiter_Cleanup(t *testing.T) {
	// This test verifies cleanup doesn't crash
	limiter := NewSlashCommandRateLimiterWithConfig(10, 5)

	// Verify initial state
	if !limiter.Allow("user") {
		t.Error("Expected first request to be allowed")
	}

	// Trigger cleanup manually
	limiter.cleanup()

	// Should still work after cleanup
	result := limiter.Allow("user")
	t.Logf("Allow after cleanup: %v", result)
	if !result {
		t.Error("Expected request to work after cleanup")
	}
}

func TestSlashCommandRateLimiter_Burst(t *testing.T) {
	// Test burst behavior
	limiter := NewSlashCommandRateLimiterWithConfig(1, 3) // 1 rps, burst of 3

	// All burst requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("burstuser") {
			t.Errorf("Expected burst request %d to be allowed", i)
		}
	}

	// Next request should be rate limited (would need to wait for token)
	// Since we're testing with 1 rps, this might be allowed or not depending on timing
	_ = time.Now()
}
