package slack

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// =============================================================================
// Session Management User Case Tests
// Tests for session ID generation and management business rules
// =============================================================================

// createTestAdapterForSession creates an adapter configured for session tests
func createTestAdapterForSession() *Adapter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAdapter(&Config{
		BotToken:      "xoxb-test-token",
		SigningSecret: "test-secret",
		Mode:          "http",
		AllowedUsers:  []string{"U123", "U456", "U789"},
	}, logger, base.WithoutServer())
}

// =============================================================================
// BR-001: Same user+bot+channel → Same SessionID
// =============================================================================

// TestSession_BR001_SameUserBotChannel returns same session ID for identical inputs
func TestSession_BR001_SameUserBotChannel(t *testing.T) {
	adapter := createTestAdapterForSession()

	// First call - should create session
	sessionID1 := adapter.GetOrCreateSession("U123", "B456", "C789")
	if sessionID1 == "" {
		t.Fatal("Expected non-empty session ID on first call")
	}

	// Second call with same parameters - should return same session ID
	sessionID2 := adapter.GetOrCreateSession("U123", "B456", "C789")
	if sessionID2 == "" {
		t.Fatal("Expected non-empty session ID on second call")
	}

	if sessionID1 != sessionID2 {
		t.Errorf("Expected same session ID for same user+bot+channel, got %s and %s", sessionID1, sessionID2)
	}

	// Verify session can be retrieved by user and channel
	session := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session == nil {
		t.Fatal("Expected to find session by user and channel")
	}
	if session.SessionID != sessionID1 {
		t.Errorf("Session ID mismatch: expected %s, got %s", sessionID1, session.SessionID)
	}
}

// =============================================================================
// BR-002: Different user → Different SessionID
// =============================================================================

// TestSession_BR002_DifferentUser tests that different users get different sessions
func TestSession_BR002_DifferentUser(t *testing.T) {
	adapter := createTestAdapterForSession()

	// User 1
	sessionID1 := adapter.GetOrCreateSession("U123", "B456", "C789")

	// User 2, same bot and channel
	sessionID2 := adapter.GetOrCreateSession("U456", "B456", "C789")

	if sessionID1 == sessionID2 {
		t.Errorf("Expected different session IDs for different users, got same: %s", sessionID1)
	}

	// Verify both sessions exist and are findable
	session1 := adapter.FindSessionByUserAndChannel("U123", "C789")
	session2 := adapter.FindSessionByUserAndChannel("U456", "C789")

	if session1 == nil {
		t.Fatal("Session for user 1 should be findable")
	}
	if session2 == nil {
		t.Fatal("Session for user 2 should be findable")
	}
	if session1.SessionID == session2.SessionID {
		t.Error("Sessions for different users should have different IDs")
	}
}

// =============================================================================
// BR-003: Different bot → Different SessionID
// =============================================================================

// TestSession_BR003_DifferentBot tests that different bots get different sessions
func TestSession_BR003_DifferentBot(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Bot 1
	sessionID1 := adapter.GetOrCreateSession("U123", "B456", "C789")

	// Bot 2, same user and channel
	sessionID2 := adapter.GetOrCreateSession("U123", "B789", "C789")

	if sessionID1 == sessionID2 {
		t.Errorf("Expected different session IDs for different bots, got same: %s", sessionID1)
	}

	// Note: FindSessionByUserAndChannel only indexes by user+channel, so both sessions
	// map to the same key. The second one overwrites in the index but both exist in sessions map.
	// This is expected behavior - the index is for quick lookup of "most recent" session.
	session := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session == nil {
		t.Error("Session should be findable")
	}
}

// =============================================================================
// BR-004: Different channel → Different SessionID
// =============================================================================

// TestSession_BR004_DifferentChannel tests that different channels get different sessions
func TestSession_BR004_DifferentChannel(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Channel 1
	sessionID1 := adapter.GetOrCreateSession("U123", "B456", "C789")

	// Channel 2, same user and bot
	sessionID2 := adapter.GetOrCreateSession("U123", "B456", "C999")

	if sessionID1 == sessionID2 {
		t.Errorf("Expected different session IDs for different channels, got same: %s", sessionID1)
	}

	// Verify both sessions exist and are findable
	session1 := adapter.FindSessionByUserAndChannel("U123", "C789")
	session2 := adapter.FindSessionByUserAndChannel("U123", "C999")

	if session1 == nil {
		t.Fatal("Session for channel 1 should be findable")
	}
	if session2 == nil {
		t.Fatal("Session for channel 2 should be findable")
	}
	if session1.SessionID == session2.SessionID {
		t.Error("Sessions for different channels should have different IDs")
	}
}

// =============================================================================
// BR-005: Different platform → Different SessionID
// =============================================================================

// TestSession_BR005_DifferentPlatform tests that different platforms get different sessions
func TestSession_BR005_DifferentPlatform(t *testing.T) {
	// Create two adapters for different platforms using base adapter directly
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create Slack adapter with platform="slack"
	slackAdapter := base.NewAdapter("slack", base.Config{
		ServerAddr:   ":8080",
		SystemPrompt: "",
	}, logger, base.WithoutServer())

	// Create Discord adapter with platform="discord"
	discordAdapter := base.NewAdapter("discord", base.Config{
		ServerAddr:   ":8081",
		SystemPrompt: "",
	}, logger, base.WithoutServer())

	// Same user ID, but different platforms
	slackSessionID := slackAdapter.GetOrCreateSession("U123", "B456", "C789")
	discordSessionID := discordAdapter.GetOrCreateSession("U123", "B456", "C789")

	t.Logf("Slack session ID: %s", slackSessionID)
	t.Logf("Discord session ID: %s", discordSessionID)
	t.Logf("Slack platform: %s", slackAdapter.Platform())
	t.Logf("Discord platform: %s", discordAdapter.Platform())

	// Platform name is part of the UUID5 input, so they should differ
	if slackSessionID == discordSessionID {
		t.Errorf("Expected different session IDs for different platforms, got same: %s", slackSessionID)
	}
}

// =============================================================================
// BR-006: Empty botUserID (DM) → Valid SessionID
// =============================================================================

// TestSession_BR006_EmptyBotUserID tests that empty botUserID (DM scenario) generates valid session
func TestSession_BR006_EmptyBotUserID(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Empty botUserID (typical for DM or single-bot setups)
	sessionID := adapter.GetOrCreateSession("U123", "", "C789")

	if sessionID == "" {
		t.Error("Expected valid session ID with empty botUserID")
	}

	// Verify session can be retrieved by user and channel
	session := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session == nil {
		t.Fatal("Session should exist with empty botUserID")
	}
	if session.SessionID != sessionID {
		t.Errorf("Session ID mismatch: expected %s, got %s", sessionID, session.SessionID)
	}
}

// =============================================================================
// BR-007: Empty channelID (DM) → Valid SessionID
// =============================================================================

// TestSession_BR007_EmptyChannelID tests that empty channelID (DM scenario) generates valid session
func TestSession_BR007_EmptyChannelID(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Empty channelID (typical for direct messages)
	sessionID := adapter.GetOrCreateSession("U123", "B456", "")

	if sessionID == "" {
		t.Error("Expected valid session ID with empty channelID")
	}

	// Verify session can be retrieved by user and channel
	session := adapter.FindSessionByUserAndChannel("U123", "")
	if session == nil {
		t.Fatal("Session should exist with empty channelID")
	}
	if session.SessionID != sessionID {
		t.Errorf("Session ID mismatch: expected %s, got %s", sessionID, session.SessionID)
	}
}

// =============================================================================
// BR-008: Session can be found by FindSessionByUserAndChannel
// =============================================================================

// TestSession_BR008_FindSessionByUserAndChannel tests O(1) session lookup
func TestSession_BR008_FindSessionByUserAndChannel(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Create session
	expectedSessionID := adapter.GetOrCreateSession("U123", "B456", "C789")

	// Find session by user and channel
	foundSession := adapter.FindSessionByUserAndChannel("U123", "C789")

	if foundSession == nil {
		t.Fatal("Expected to find session by user and channel")
	}

	if foundSession.SessionID != expectedSessionID {
		t.Errorf("Session ID mismatch: expected %s, got %s", expectedSessionID, foundSession.SessionID)
	}

	if foundSession.UserID != "U123" {
		t.Errorf("User ID mismatch: expected U123, got %s", foundSession.UserID)
	}
}

// TestSession_BR008_FindSessionByUserAndChannel_NotFound tests lookup for non-existent session
func TestSession_BR008_FindSessionByUserAndChannel_NotFound(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Try to find session that doesn't exist
	foundSession := adapter.FindSessionByUserAndChannel("U999", "C999")

	if foundSession != nil {
		t.Errorf("Expected nil for non-existent session, got %v", foundSession)
	}
}

// =============================================================================
// BR-009: Session LastActive updated on reuse
// =============================================================================

// TestSession_BR009_LastActiveUpdatedOnReuse tests that LastActive timestamp updates on session reuse
func TestSession_BR009_LastActiveUpdatedOnReuse(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Create session
	sessionID := adapter.GetOrCreateSession("U123", "B456", "C789")

	// Get initial LastActive
	session1 := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session1 == nil {
		t.Fatal("Session should exist")
	}
	initialLastActive := session1.LastActive

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Reuse session (same parameters)
	adapter.GetOrCreateSession("U123", "B456", "C789")

	// Get updated LastActive
	session2 := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session2 == nil {
		t.Fatal("Session should still exist")
	}

	// LastActive should be updated (greater than initial)
	if !session2.LastActive.After(initialLastActive) {
		t.Errorf("LastActive should be updated: initial=%v, updated=%v", initialLastActive, session2.LastActive)
	}

	// Suppress unused variable warning
	_ = sessionID
}

// =============================================================================
// BR-010: Concurrent session creation is thread-safe
// =============================================================================

// TestSession_BR010_ConcurrentCreation tests thread safety of concurrent session creation
func TestSession_BR010_ConcurrentCreation(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Test concurrent creation of different sessions
	users := []string{"U1", "U2", "U3", "U4", "U5"}
	bots := []string{"B1", "B2"}
	channels := []string{"C1", "C2", "C3"}

	// Create sessions concurrently
	var wg sync.WaitGroup
	sessionCount := len(users) * len(bots) * len(channels)
	wg.Add(sessionCount)

	for _, user := range users {
		for _, bot := range bots {
			for _, channel := range channels {
				go func(u, b, c string) {
					defer wg.Done()
					sessionID := adapter.GetOrCreateSession(u, b, c)
					if sessionID == "" {
						t.Errorf("Expected non-empty session ID for user=%s, bot=%s, channel=%s", u, b, c)
					}
				}(user, bot, channel)
			}
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify all sessions were created
	// Note: FindSessionByUserAndChannel indexes by user+channel only (not bot),
	// so we verify by checking unique user+channel combinations.
	// Total sessions created: 5 users × 2 bots × 3 channels = 30
	// Unique user+channel combinations: 5 × 3 = 15
	foundCount := 0
	for _, user := range users {
		for _, channel := range channels {
			session := adapter.FindSessionByUserAndChannel(user, channel)
			if session != nil {
				foundCount++
			}
		}
	}

	expectedCount := len(users) * len(channels)
	if foundCount != expectedCount {
		t.Errorf("Expected to find %d user+channel combinations, found %d", expectedCount, foundCount)
	}

	t.Logf("Created %d total sessions, verified %d unique user+channel combinations", sessionCount, foundCount)
}

// TestSession_BR010_ConcurrentSameSession tests thread safety when creating same session concurrently
func TestSession_BR010_ConcurrentSameSession(t *testing.T) {
	adapter := createTestAdapterForSession()

	sessionCount := 100

	var wg sync.WaitGroup
	wg.Add(sessionCount)

	// All goroutines try to create the same session
	for i := 0; i < sessionCount; i++ {
		go func() {
			defer wg.Done()
			sessionID := adapter.GetOrCreateSession("U123", "B456", "C789")
			if sessionID == "" {
				t.Error("Expected non-empty session ID")
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify session exists and is findable
	session := adapter.FindSessionByUserAndChannel("U123", "C789")
	if session == nil {
		t.Fatal("Expected session to exist")
	}
	if session.UserID != "U123" {
		t.Errorf("Expected user U123, got %s", session.UserID)
	}
}

// =============================================================================
// Integration Tests: Real-world Slack scenarios
// =============================================================================

// TestSession_Integration_DMScenario tests direct message scenario (empty bot and channel)
func TestSession_Integration_DMScenario(t *testing.T) {
	adapter := createTestAdapterForSession()

	// User sends DM to bot
	sessionID1 := adapter.GetOrCreateSession("U123", "", "D123")

	// User sends another DM - should reuse session
	sessionID2 := adapter.GetOrCreateSession("U123", "", "D123")

	if sessionID1 != sessionID2 {
		t.Errorf("DM session should be reused: %s vs %s", sessionID1, sessionID2)
	}

	// Different user's DM should get different session
	sessionID3 := adapter.GetOrCreateSession("U456", "", "D456")
	if sessionID1 == sessionID3 {
		t.Error("Different users should have different DM sessions")
	}

	// All sessions should be findable
	s1 := adapter.FindSessionByUserAndChannel("U123", "D123")
	s3 := adapter.FindSessionByUserAndChannel("U456", "D456")

	if s1 == nil || s3 == nil {
		t.Error("All DM sessions should be findable")
	}
}

// TestSession_Integration_MultiChannelUser tests user active in multiple channels
func TestSession_Integration_MultiChannelUser(t *testing.T) {
	adapter := createTestAdapterForSession()

	// User in channel 1
	sessionC1 := adapter.GetOrCreateSession("U123", "B456", "C1")

	// Same user in channel 2
	sessionC2 := adapter.GetOrCreateSession("U123", "B456", "C2")

	// Same user in channel 3
	sessionC3 := adapter.GetOrCreateSession("U123", "B456", "C3")

	// All should be different
	if sessionC1 == sessionC2 || sessionC1 == sessionC3 || sessionC2 == sessionC3 {
		t.Error("Different channels should have different sessions for same user")
	}

	// All should be findable
	s1 := adapter.FindSessionByUserAndChannel("U123", "C1")
	s2 := adapter.FindSessionByUserAndChannel("U123", "C2")
	s3 := adapter.FindSessionByUserAndChannel("U123", "C3")

	if s1 == nil || s2 == nil || s3 == nil {
		t.Error("All sessions should be findable")
	}
}

// TestSession_Integration_ThreadedConversation tests session handling for threaded conversations
func TestSession_Integration_ThreadedConversation(t *testing.T) {
	adapter := createTestAdapterForSession()

	// Initial message in thread
	sessionID1 := adapter.GetOrCreateSession("U123", "B456", "C123")

	// Reply in same thread (same user, bot, channel)
	sessionID2 := adapter.GetOrCreateSession("U123", "B456", "C123")

	// Should reuse same session for thread context
	if sessionID1 != sessionID2 {
		t.Errorf("Thread conversation should use same session: %s vs %s", sessionID1, sessionID2)
	}
}
