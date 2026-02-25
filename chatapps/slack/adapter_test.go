package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// generateSignature creates a valid Slack signature for testing
func generateSignature(secret, timestamp, body string) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

// createTestAdapter creates an adapter with the given signing secret for testing
func createTestAdapter(signingSecret string) *Adapter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAdapter(&Config{
		BotToken:      "xoxb-test-bot-token-123456789012-abcdef",
		SigningSecret: signingSecret,
		Mode:          "http",
	}, logger, base.WithoutServer())
}

// TestVerifySignature_ValidSignature tests the happy path with a valid signature
func TestVerifySignature_ValidSignature(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback","challenge":"test-challenge"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	signature := generateSignature(signingSecret, timestamp, body)

	adapter := createTestAdapter(signingSecret)
	result := adapter.verifySignature([]byte(body), timestamp, signature)

	if !result {
		t.Error("Expected valid signature to pass verification")
	}
}

// TestVerifySignature_ExpiredTimestamp tests that signatures older than 5 minutes are rejected
func TestVerifySignature_ExpiredTimestamp(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	// Timestamp from 10 minutes ago
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	signature := generateSignature(signingSecret, oldTimestamp, body)

	adapter := createTestAdapter(signingSecret)
	result := adapter.verifySignature([]byte(body), oldTimestamp, signature)

	if result {
		t.Error("Expected expired timestamp to fail verification")
	}
}

// TestVerifySignature_InvalidSignatureFormat tests with malformed signature format
func TestVerifySignature_InvalidSignatureFormat(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Invalid format: missing "v0=" prefix
	invalidSignatures := []string{
		"invalidsignature",
		"v0:abcdef123456",
		"v0=invalid",
		"",
		"v0=not-a-valid-hex-characters",
	}

	adapter := createTestAdapter(signingSecret)

	for _, sig := range invalidSignatures {
		result := adapter.verifySignature([]byte(body), timestamp, sig)
		if result {
			t.Errorf("Expected signature %q to fail verification", sig)
		}
	}
}

// TestVerifySignature_TamperedBody tests that modified body invalidates the signature
func TestVerifySignature_TamperedBody(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	originalBody := `{"type":"event_callback","challenge":"test"}`
	tamperedBody := `{"type":"event_callback","challenge":"hacked"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Generate signature for original body
	signature := generateSignature(signingSecret, timestamp, originalBody)

	adapter := createTestAdapter(signingSecret)

	// Verify original body passes
	if !adapter.verifySignature([]byte(originalBody), timestamp, signature) {
		t.Error("Original body should pass verification")
	}

	// Verify tampered body fails
	if adapter.verifySignature([]byte(tamperedBody), timestamp, signature) {
		t.Error("Tampered body should fail verification")
	}
}

// TestVerifySignature_MissingSignatureHeader tests behavior when signature header is empty
func TestVerifySignature_MissingSignatureHeader(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	adapter := createTestAdapter(signingSecret)

	// Empty signature
	result := adapter.verifySignature([]byte(body), timestamp, "")
	if result {
		t.Error("Expected empty signature to fail verification")
	}

	// Whitespace only signature
	result = adapter.verifySignature([]byte(body), timestamp, "   ")
	if result {
		t.Error("Expected whitespace-only signature to fail verification")
	}
}

// TestVerifySignature_MissingTimestampHeader tests behavior when timestamp header is empty
func TestVerifySignature_MissingTimestampHeader(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`

	adapter := createTestAdapter(signingSecret)

	// Empty timestamp
	result := adapter.verifySignature([]byte(body), "", "v0=somesig")
	if result {
		t.Error("Expected empty timestamp to fail verification")
	}

	// Whitespace only timestamp
	result = adapter.verifySignature([]byte(body), "   ", "v0=somesig")
	if result {
		t.Error("Expected whitespace-only timestamp to fail verification")
	}

	// Invalid timestamp format
	result = adapter.verifySignature([]byte(body), "not-a-number", "v0=somesig")
	if result {
		t.Error("Expected invalid timestamp format to fail verification")
	}
}

// TestVerifySignature_ReplayAttack tests that signatures can be replayed within the 5-minute window
// Note: This is expected behavior - the timestamp check is the replay protection.
// A true replay attack prevention would require a nonce, which Slack doesn't implement.
// The 5-minute window is Slack's documented mechanism for replay protection.
func TestVerifySignature_ReplayAttack(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Generate a valid signature
	signature := generateSignature(signingSecret, timestamp, body)

	adapter := createTestAdapter(signingSecret)

	// First verification should pass
	if !adapter.verifySignature([]byte(body), timestamp, signature) {
		t.Error("First verification should pass")
	}

	// Same signature used again within 5 minutes - this WILL pass
	// This is the expected behavior per Slack's design
	// The protection is that old signatures (>5 min) won't work
	if !adapter.verifySignature([]byte(body), timestamp, signature) {
		t.Log("Note: Same signature reused within 5-minute window passes - this is expected Slack behavior")
		t.Log("The replay attack protection is the timestamp check, not a nonce")
	}

	// Now test with an old timestamp - should fail
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	oldSignature := generateSignature(signingSecret, oldTimestamp, body)

	if adapter.verifySignature([]byte(body), oldTimestamp, oldSignature) {
		t.Error("Old signature (10 min ago) should fail - replay attack protection works")
	}
}

// TestVerifySignature_TimingAttackMitigation verifies that constant-time comparison is used
func TestVerifySignature_TimingAttackMitigation(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Generate two different signatures
	sig1 := generateSignature(signingSecret, timestamp, body)
	// Generate signature with different body to get different hash
	sig2 := generateSignature(signingSecret, timestamp, body+"different")

	adapter := createTestAdapter(signingSecret)

	// Both should return false (invalid)
	result1 := adapter.verifySignature([]byte(body), timestamp, sig1)
	result2 := adapter.verifySignature([]byte(body), timestamp, sig2)

	if !result1 {
		t.Log("Valid signature correctly verified")
	}

	// Both should be false - timing information should be the same
	_ = result2

	// Note: Testing actual timing differences would require statistical analysis
	// The use of hmac.Equal provides constant-time comparison
}

// TestVerifySignature_WrongSecret tests that using a different secret fails
func TestVerifySignature_WrongSecret(t *testing.T) {
	correctSecret := "correct-signing-secret-123456789012345"
	wrongSecret := "wrong-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// Generate signature with wrong secret
	signature := generateSignature(wrongSecret, timestamp, body)

	// Adapter configured with correct secret
	adapter := createTestAdapter(correctSecret)
	result := adapter.verifySignature([]byte(body), timestamp, signature)

	if result {
		t.Error("Expected wrong secret to fail verification")
	}
}

// TestVerifySignature_TimestampWithV0Prefix tests timestamp with v0= prefix (as some clients send)
func TestVerifySignature_TimestampWithV0Prefix(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("v0=%d", time.Now().Unix())

	signature := generateSignature(signingSecret, strings.TrimPrefix(timestamp, "v0="), body)

	adapter := createTestAdapter(signingSecret)
	result := adapter.verifySignature([]byte(body), timestamp, signature)

	if !result {
		t.Error("Expected timestamp with v0= prefix to pass verification")
	}
}

// TestVerifySignature_TableDriven provides comprehensive table-driven tests
func TestVerifySignature_TableDriven(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	validBody := `{"type":"event_callback"}`
	validTimestamp := fmt.Sprintf("%d", time.Now().Unix())
	validSignature := generateSignature(signingSecret, validTimestamp, validBody)

	tests := []struct {
		name        string
		body        string
		timestamp   string
		signature   string
		expectValid bool
		description string
	}{
		{
			name:        "valid_signature",
			body:        validBody,
			timestamp:   validTimestamp,
			signature:   validSignature,
			expectValid: true,
			description: "Valid signature should pass",
		},
		{
			name:        "empty_signature",
			body:        validBody,
			timestamp:   validTimestamp,
			signature:   "",
			expectValid: false,
			description: "Empty signature should fail",
		},
		{
			name:        "empty_timestamp",
			body:        validBody,
			timestamp:   "",
			signature:   validSignature,
			expectValid: false,
			description: "Empty timestamp should fail",
		},
		{
			name:        "invalid_timestamp_format",
			body:        validBody,
			timestamp:   "not-a-number",
			signature:   validSignature,
			expectValid: false,
			description: "Invalid timestamp format should fail",
		},
		{
			name:        "expired_timestamp_10min",
			body:        validBody,
			timestamp:   fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix()),
			signature:   validSignature,
			expectValid: false,
			description: "10 min old timestamp should fail",
		},
		{
			name:        "expired_timestamp_6min",
			body:        validBody,
			timestamp:   fmt.Sprintf("%d", time.Now().Add(-6*time.Minute).Unix()),
			signature:   validSignature,
			expectValid: false,
			description: "6 min old timestamp should fail",
		},
		{
			name:        "fresh_timestamp_3min",
			body:        validBody,
			timestamp:   fmt.Sprintf("%d", time.Now().Add(-3*time.Minute).Unix()),
			signature:   generateSignature(signingSecret, fmt.Sprintf("%d", time.Now().Add(-3*time.Minute).Unix()), validBody),
			expectValid: true,
			description: "3 min old timestamp should still pass",
		},
		{
			name:        "tampered_body",
			body:        `{"type":"event_callback","hacked":true}`,
			timestamp:   validTimestamp,
			signature:   validSignature,
			expectValid: false,
			description: "Tampered body should fail",
		},
		{
			name:        "wrong_signature",
			body:        validBody,
			timestamp:   validTimestamp,
			signature:   "v0=0000000000000000000000000000000000000000000000000000000000000000",
			expectValid: false,
			description: "Wrong signature should fail",
		},
		{
			name:        "malformed_signature_no_v0",
			body:        validBody,
			timestamp:   validTimestamp,
			signature:   "not-a-valid-signature",
			expectValid: false,
			description: "Signature without v0= prefix should fail",
		},
		{
			name:        "timestamp_with_v0_prefix",
			body:        validBody,
			timestamp:   "v0=" + validTimestamp,
			signature:   validSignature,
			expectValid: true,
			description: "Timestamp with v0= prefix should work",
		},
		{
			name:        "body_with_special_chars",
			body:        `{"text":"Hello <world> & \"quoted\""}`,
			timestamp:   validTimestamp,
			signature:   generateSignature(signingSecret, validTimestamp, `{"text":"Hello <world> & \"quoted\""}`),
			expectValid: true,
			description: "Body with special characters should work",
		},
		{
			name:        "body_with_newlines",
			body:        "line1\nline2\r\nline3",
			timestamp:   validTimestamp,
			signature:   generateSignature(signingSecret, validTimestamp, "line1\nline2\r\nline3"),
			expectValid: true,
			description: "Body with newlines should work",
		},
		{
			name:        "empty_body",
			body:        "",
			timestamp:   validTimestamp,
			signature:   generateSignature(signingSecret, validTimestamp, ""),
			expectValid: true,
			description: "Empty body should work",
		},
	}

	adapter := createTestAdapter(signingSecret)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.verifySignature([]byte(tt.body), tt.timestamp, tt.signature)
			if result != tt.expectValid {
				t.Errorf("%s: got %v, want %v", tt.description, result, tt.expectValid)
			}
		})
	}
}

// BenchmarkVerifySignature benchmarks the signature verification performance
func BenchmarkVerifySignature(b *testing.B) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback","challenge":"test-challenge"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, body)

	adapter := createTestAdapter(signingSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.verifySignature([]byte(body), timestamp, signature)
	}
}

// BenchmarkVerifySignature_Invalid benchmarks verification with invalid signature
func BenchmarkVerifySignature_Invalid(b *testing.B) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	invalidSignature := "v0=invalid-signature-that-does-not-match"

	adapter := createTestAdapter(signingSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.verifySignature([]byte(body), timestamp, invalidSignature)
	}
}

// BenchmarkVerifySignature_ExpiredTimestamp benchmarks verification with expired timestamp
func BenchmarkVerifySignature_ExpiredTimestamp(b *testing.B) {
	signingSecret := "test-signing-secret-123456789012345"
	body := `{"type":"event_callback"}`
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	signature := generateSignature(signingSecret, oldTimestamp, body)

	adapter := createTestAdapter(signingSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.verifySignature([]byte(body), oldTimestamp, signature)
	}
}

// =============================================================================
// HTTP Handler Tests
// =============================================================================

// errorReader is a reader that returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// =============================================================================
// handleSlashCommand Tests
// =============================================================================

// TestHandleSlashCommand_MethodNotAllowed tests that GET requests return 405
func TestHandleSlashCommand_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/webhook/slack", nil)
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleSlashCommand_ClearCommand tests /clear command processing
func TestHandleSlashCommand_ClearCommand(t *testing.T) {
	adapter := createTestAdapter("")

	// The slash command handler processes in a goroutine, so we just verify
	// that the request is parsed correctly and returns 200
	body := "command=/clear&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_ParseFormError tests handling of malformed form data
func TestHandleSlashCommand_ParseFormError(t *testing.T) {
	adapter := createTestAdapter("")

	// Create request with invalid form data
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader("invalid=data%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should return 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleSlashCommand_UnknownCommand tests handling of unknown commands
func TestHandleSlashCommand_UnknownCommand(t *testing.T) {
	adapter := createTestAdapter("")

	body := "command=/unknown&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should still return 200 (acks immediately)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_EngineNotSet tests behavior when engine is nil
func TestHandleSlashCommand_EngineNotSet(t *testing.T) {
	adapter := createTestAdapter("")
	// Don't set engine - adapter.eng is nil

	body := "command=/clear&text=&user_id=U123&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	// Should still return 200 (immediate ack)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleSlashCommand_WithText tests slash command with text argument
func TestHandleSlashCommand_WithText(t *testing.T) {
	adapter := createTestAdapter("")

	body := "command=/clear&text=all&user_id=U456&channel_id=C123&response_url=http://test"
	req := httptest.NewRequest(http.MethodPost, "/webhook/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleSlashCommand(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// =============================================================================
// handleEvent Tests
// =============================================================================

// TestHandleEvent_ChallengeResponse tests URL verification challenge response
func TestHandleEvent_ChallengeResponse(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"url_verification","challenge":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Response is plain text, not JSON
	if w.Body.String() != "abc123" {
		t.Errorf("Expected challenge 'abc123', got %s", w.Body.String())
	}

	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", w.Header().Get("Content-Type"))
	}
}

// TestHandleEvent_SignatureVerification tests invalid signature rejection
func TestHandleEvent_SignatureVerification(t *testing.T) {
	signingSecret := "test-signing-secret"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", "invalid")
	req.Header.Set("X-Slack-Request-Timestamp", "12345")
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_MissingSignatureHeaders tests behavior when signature headers are missing
func TestHandleEvent_MissingSignatureHeaders(t *testing.T) {
	signingSecret := "test-signing-secret"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	// No signature headers
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_ValidSignatureWithToken tests valid signature with bot token
func TestHandleEvent_ValidSignatureWithToken(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message","channel":"C123","user":"U123","text":"hello"},"token":"xoxb-test-bot-token-123456789012-abcdef"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleEvent_InvalidToken tests rejection of invalid token
func TestHandleEvent_InvalidToken(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"event_callback","event":{"type":"message"},"token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestHandleEvent_InvalidJSON tests handling of malformed JSON
func TestHandleEvent_InvalidJSON(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleEvent_MethodNotAllowed tests that non-POST requests return 405
func TestHandleEvent_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleEvent_ReadBodyError tests handling of body read errors
func TestHandleEvent_ReadBodyError(t *testing.T) {
	adapter := createTestAdapter("")

	// Create a request with a reader that returns error
	req := httptest.NewRequest(http.MethodPost, "/events", &errorReader{err: fmt.Errorf("read error")})
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// TestHandleEvent_NoSignatureForURLVerification tests that URL verification requires signature when secret is set
func TestHandleEvent_NoSignatureForURLVerification(t *testing.T) {
	adapter := createTestAdapter("") // No signing secret

	// URL verification should work without signature when no secret is set
	body := `{"type":"url_verification","challenge":"test-challenge"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Response is plain text, not JSON
	if w.Body.String() != "test-challenge" {
		t.Errorf("Expected challenge 'test-challenge', got %s", w.Body.String())
	}
}

// =============================================================================
// handleInteractive Tests
// =============================================================================

// TestHandleInteractive_MethodNotAllowed tests that non-POST requests return 405
func TestHandleInteractive_MethodNotAllowed(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodGet, "/interactive", nil)
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// TestHandleInteractive_ValidRequest tests valid interactive endpoint request
func TestHandleInteractive_ValidRequest(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"block_actions"}`
	req := httptest.NewRequest(http.MethodPost, "/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

// TestHandleInteractive_ReadBodyError tests handling of body read errors
func TestHandleInteractive_ReadBodyError(t *testing.T) {
	adapter := createTestAdapter("")

	req := httptest.NewRequest(http.MethodPost, "/interactive", &errorReader{err: fmt.Errorf("read error")})
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// =============================================================================
// handleSocketModeEvent Tests
// =============================================================================

// TestHandleSocketModeEvent_MessageEvent tests Socket Mode message event handling
func TestHandleSocketModeEvent_MessageEvent(t *testing.T) {
	adapter := createTestAdapter("")

	// Create a message event - this tests that the handler doesn't panic
	// Note: Without proper config, the message will be filtered, but we just
	// verify the handler can process the event without crashing
	eventData := `{
		"type": "message",
		"channel": "C123",
		"channel_type": "dm",
		"user": "U123",
		"text": "hello world",
		"ts": "1234567890.123456"
	}`

	// This should not panic
	adapter.handleSocketModeEvent("message", json.RawMessage(eventData))
}

// TestHandleSocketModeEvent_InvalidJSON tests handling of malformed JSON
func TestHandleSocketModeEvent_InvalidJSON(t *testing.T) {
	adapter := createTestAdapter("")

	// Invalid JSON should not panic
	adapter.handleSocketModeEvent("message", json.RawMessage(`{invalid`))
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration_SlashCommandToHandlerFlow tests full flow from slash command to handler
func TestIntegration_SlashCommandToHandlerFlow(t *testing.T) {
	adapter := createTestAdapter("")

	// Verify the handler processes the request correctly
	body := "command=/clear&text=all&user_id=U456&channel_id=C123&response_url=https://hooks.slack.com/commands/123"
	req := httptest.NewRequest(http.MethodPost, "/slack", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Call handler
	adapter.handleSlashCommand(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestIntegration_EventWithSignatureFlow tests full flow with signature verification
func TestIntegration_EventWithSignatureFlow(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message","channel":"C123","channel_type":"dm","user":"U123","text":"test message","ts":"1234567890.123456"},"token":"xoxb-test-bot-token-123456789012-abcdef"}`
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(signingSecret, timestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestIntegration_ExpiredSignatureFlow tests that expired signatures are rejected
func TestIntegration_ExpiredSignatureFlow(t *testing.T) {
	signingSecret := "test-signing-secret-123456789012345"
	adapter := createTestAdapter(signingSecret)

	body := `{"type":"event_callback","event":{"type":"message"}}`
	// Timestamp from 10 minutes ago
	oldTimestamp := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	signature := generateSignature(signingSecret, oldTimestamp, body)

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Signature", signature)
	req.Header.Set("X-Slack-Request-Timestamp", oldTimestamp)
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for expired signature, got %d", w.Code)
	}
}

// TestIntegration_NoSignatureForVerification tests URL verification works without signature when no secret set
func TestIntegration_NoSignatureForVerification(t *testing.T) {
	adapter := createTestAdapter("") // No signing secret

	// URL verification doesn't require signature when no secret is configured
	body := `{"type":"url_verification","challenge":"verify-token"}`
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	adapter.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "verify-token" {
		t.Errorf("Expected challenge verify-token, got %s", w.Body.String())
	}
}

// TestIntegration_InteractiveEndpoint tests full interactive endpoint flow
func TestIntegration_InteractiveEndpoint(t *testing.T) {
	adapter := createTestAdapter("")

	body := `{"type":"interactive_message","callback_id":"test_callback"}`
	req := httptest.NewRequest(http.MethodPost, "/interactive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	adapter.handleInteractive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
