package feishu

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEncodeActionValue(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		sessionID string
		messageID string
		wantErr   bool
	}{
		{
			name:      "Permission request",
			action:    "permission_request",
			sessionID: "test-session-123",
			messageID: "msg-456",
			wantErr:   false,
		},
		{
			name:      "Empty message ID",
			action:    "permission_request",
			sessionID: "test-session-789",
			messageID: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeActionValue(tt.action, tt.sessionID, tt.messageID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EncodeActionValue() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify it can be decoded back
				var decoded ActionValue
				if err := json.Unmarshal([]byte(got), &decoded); err != nil {
					t.Fatalf("Failed to unmarshal encoded value: %v", err)
				}

				if decoded.Action != tt.action {
					t.Errorf("Action mismatch: got %s, want %s", decoded.Action, tt.action)
				}
				if decoded.SessionID != tt.sessionID {
					t.Errorf("SessionID mismatch: got %s, want %s", decoded.SessionID, tt.sessionID)
				}
				if decoded.MessageID != tt.messageID {
					t.Errorf("MessageID mismatch: got %s, want %s", decoded.MessageID, tt.messageID)
				}
			}
		})
	}
}

func TestDecodeActionValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    *ActionValue
		wantErr bool
	}{
		{
			name:  "Valid permission request",
			value: `{"action":"permission_request","session_id":"test-123","message_id":"msg-456"}`,
			want: &ActionValue{
				Action:    "permission_request",
				SessionID: "test-123",
				MessageID: "msg-456",
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			value:   `{invalid json}`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty value",
			value:   "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeActionValue(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeActionValue() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if got == nil {
					t.Fatal("Expected non-nil result")
				}
				if got.Action != tt.want.Action {
					t.Errorf("Action mismatch: got %s, want %s", got.Action, tt.want.Action)
				}
				if got.SessionID != tt.want.SessionID {
					t.Errorf("SessionID mismatch: got %s, want %s", got.SessionID, tt.want.SessionID)
				}
				if got.MessageID != tt.want.MessageID {
					t.Errorf("MessageID mismatch: got %s, want %s", got.MessageID, tt.want.MessageID)
				}
			}
		})
	}
}

func TestHandleInteractive_URLVerification(t *testing.T) {
	logger := slog.Default()
	config := &Config{
		AppID:             "test_app_id",
		AppSecret:         "test_app_secret",
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key",
		ServerAddr:        ":0",
		SystemPrompt:      "test",
	}

	adapter, err := NewAdapter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() { _ = adapter.Stop() }()

	handler := NewInteractiveHandler(adapter)

	// Create URL verification request
	event := InteractiveEvent{
		Header: &InteractiveHeader{
			EventType: "url_verification",
		},
		Token: "test_challenge_token",
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/feishu/interactive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Add signature headers
	req.Header.Set("X-Timestamp", "1234567890")
	req.Header.Set("X-Signature", calculateHMACSHA256("1234567890"+"test_encrypt_key"+string(body), "test_encrypt_key"))

	rr := httptest.NewRecorder()
	handler.HandleInteractive(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify challenge response
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["challenge"] != "test_challenge_token" {
		t.Errorf("Expected challenge %s, got %s", "test_challenge_token", response["challenge"])
	}
}

func TestActionValueRoundTrip(t *testing.T) {
	original := &ActionValue{
		Action:    "permission_request",
		SessionID: "session-abc-123",
		MessageID: "msg-xyz-789",
	}

	// Encode
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode
	var decoded ActionValue
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	// Verify
	if decoded.Action != original.Action {
		t.Errorf("Action mismatch: got %s, want %s", decoded.Action, original.Action)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", decoded.SessionID, original.SessionID)
	}
	if decoded.MessageID != original.MessageID {
		t.Errorf("MessageID mismatch: got %s, want %s", decoded.MessageID, original.MessageID)
	}
}

func TestInteractiveEventParsing(t *testing.T) {
	// Test full event parsing
	jsonStr := `{
		"header": {
			"event_id": "evt-123",
			"event_type": "im.message.reply",
			"create_time": "2026-03-03T08:00:00Z",
			"token": "token-xyz",
			"app_id": "app-123",
			"tenant_key": "tenant-456"
		},
		"event": {
			"message": {
				"message_id": "msg-789",
				"chat_id": "chat-abc",
				"message_type": "interactive"
			},
			"user": {
				"user_id": "user-def"
			},
			"action": {
				"value": "{\"action\":\"permission_request\",\"session_id\":\"sess-123\"}",
				"tag": "button"
			}
		},
		"token": "challenge-token"
	}`

	var event InteractiveEvent
	if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	// Verify parsed values
	if event.Header.EventID != "evt-123" {
		t.Errorf("EventID mismatch: got %s", event.Header.EventID)
	}
	if event.Header.EventType != "im.message.reply" {
		t.Errorf("EventType mismatch: got %s", event.Header.EventType)
	}
	if event.Event.Message.MessageID != "msg-789" {
		t.Errorf("MessageID mismatch: got %s", event.Event.Message.MessageID)
	}
	if event.Event.User.UserID != "user-def" {
		t.Errorf("UserID mismatch: got %s", event.Event.User.UserID)
	}

	// Verify action value can be decoded
	var actionValue ActionValue
	if err := json.Unmarshal([]byte(event.Event.Action.Value), &actionValue); err != nil {
		t.Fatalf("Failed to decode action value: %v", err)
	}
	if actionValue.SessionID != "sess-123" {
		t.Errorf("SessionID mismatch: got %s", actionValue.SessionID)
	}
}
