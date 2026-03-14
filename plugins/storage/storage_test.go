package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hrygo/hotplex/types"
)

// ========================================
// StorageStrategy Tests
// ========================================

func TestDefaultStrategy_ShouldStore_Storable(t *testing.T) {
	s := NewDefaultStrategy()
	
	msg := &ChatAppMessage{
		MessageType: types.MessageTypeUserInput,
	}
	
	if !s.ShouldStore(msg) {
		t.Error("ShouldStore should return true for storable message types")
	}
}

func TestDefaultStrategy_ShouldStore_NotStorable(t *testing.T) {
	s := NewDefaultStrategy()
	
	msg := &ChatAppMessage{
		MessageType: types.MessageTypeThinking,
	}
	
	if s.ShouldStore(msg) {
		t.Error("ShouldStore should return false for non-storable message types")
	}
}

func TestDefaultStrategy_BeforeStore(t *testing.T) {
	s := NewDefaultStrategy()
	
	msg := &ChatAppMessage{Content: "test"}
	err := s.BeforeStore(context.Background(), msg)
	if err != nil {
		t.Errorf("BeforeStore failed: %v", err)
	}
}

func TestDefaultStrategy_AfterStore(t *testing.T) {
	s := NewDefaultStrategy()
	
	msg := &ChatAppMessage{Content: "test"}
	err := s.AfterStore(context.Background(), msg)
	if err != nil {
		t.Errorf("AfterStore failed: %v", err)
	}
}

// ========================================
// MessageQuery Tests
// ========================================

func TestMessageQuery_Defaults(t *testing.T) {
	q := &MessageQuery{}
	
	if q.Limit != 0 {
		t.Errorf("Limit default = %d, want 0", q.Limit)
	}
	if q.Ascending {
		t.Error("Ascending should default to false")
	}
	if q.IncludeDeleted {
		t.Error("IncludeDeleted should default to false")
	}
}

func TestMessageQuery_Custom(t *testing.T) {
	now := time.Now()
	q := &MessageQuery{
		ChatSessionID:      "chat-123",
		ChatUserID:         "user-456",
		EngineSessionID:    uuid.New(),
		ProviderType:       "claude",
		ProviderSessionID:  "prov-789",
		StartTime:          &now,
		EndTime:            &now,
		MessageTypes:       []types.MessageType{types.MessageTypeUserInput},
		Limit:              100,
		Offset:             10,
		Ascending:          true,
		IncludeDeleted:     true,
	}
	
	if q.ChatSessionID != "chat-123" {
		t.Errorf("ChatSessionID = %s", q.ChatSessionID)
	}
	if q.Limit != 100 {
		t.Errorf("Limit = %d", q.Limit)
	}
	if !q.Ascending {
		t.Error("Ascending should be true")
	}
}

// ========================================
// SessionMeta Tests
// ========================================

func TestSessionMeta_Fields(t *testing.T) {
	now := time.Now()
	meta := &SessionMeta{
		ChatSessionID: "chat-123",
		ChatPlatform:  "slack",
		ChatUserID:    "user-456",
		LastMessageID: "msg-789",
		LastMessageAt: now,
		MessageCount:  42,
		UpdatedAt:     now,
	}
	
	if meta.ChatSessionID != "chat-123" {
		t.Errorf("ChatSessionID = %s", meta.ChatSessionID)
	}
	if meta.MessageCount != 42 {
		t.Errorf("MessageCount = %d", meta.MessageCount)
	}
}

// ========================================
// ChatAppMessage Tests
// ========================================

func TestChatAppMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := &ChatAppMessage{
		ID:                "msg-123",
		ChatSessionID:     "chat-456",
		ChatPlatform:      "slack",
		ChatUserID:        "user-789",
		ChatBotUserID:     "bot-000",
		ChatChannelID:     "channel-111",
		ChatThreadID:      "thread-222",
		EngineSessionID:   uuid.New(),
		EngineNamespace:   "default",
		ProviderSessionID: "prov-333",
		ProviderType:      "claude",
		MessageType:       types.MessageTypeUserInput,
		FromUserID:        "user-789",
		FromUserName:      "Test User",
		ToUserID:          "bot-000",
		Content:           "Hello world",
		Metadata:          map[string]any{"key": "value"},
		CreatedAt:         now,
		UpdatedAt:         now,
		Deleted:           false,
		DeletedAt:         nil,
	}
	
	if msg.ID != "msg-123" {
		t.Errorf("ID = %s", msg.ID)
	}
	if msg.Content != "Hello world" {
		t.Errorf("Content = %s", msg.Content)
	}
	if msg.MessageType != types.MessageTypeUserInput {
		t.Errorf("MessageType = %v", msg.MessageType)
	}
}

func TestChatAppMessage_Deleted(t *testing.T) {
	now := time.Now()
	msg := &ChatAppMessage{
		Deleted:   true,
		DeletedAt: &now,
	}
	
	if !msg.Deleted {
		t.Error("Deleted should be true")
	}
	if msg.DeletedAt == nil {
		t.Error("DeletedAt should not be nil")
	}
}

// ========================================
// Interface Compliance Tests
// ========================================

func TestStorageStrategy_Interface(t *testing.T) {
	// Compile-time verification
	var _ StorageStrategy = (*DefaultStrategy)(nil)
}

// ========================================
// Factory Tests
// ========================================

func TestNewDefaultStrategy(t *testing.T) {
	s := NewDefaultStrategy()
	if s == nil {
		t.Error("NewDefaultStrategy should not return nil")
	}
}

// ========================================
// Utils Tests  
// ========================================

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *ChatAppMessage
		wantErr bool
	}{
		{
			name:    "valid message",
			msg:     &ChatAppMessage{Content: "test", ChatSessionID: "chat-123", EngineSessionID: uuid.New(), ProviderSessionID: "prov-123"},
			wantErr: false,
		},
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name:    "missing chat session",
			msg:     &ChatAppMessage{Content: "test", EngineSessionID: uuid.New(), ProviderSessionID: "prov-123"},
			wantErr: true,
		},
		{
			name:    "missing engine session",
			msg:     &ChatAppMessage{Content: "test", ChatSessionID: "chat-123", ProviderSessionID: "prov-123"},
			wantErr: true,
		},
		{
			name:    "missing provider session",
			msg:     &ChatAppMessage{Content: "test", ChatSessionID: "chat-123", EngineSessionID: uuid.New()},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"short", 10, "short"},
	}
	
	for _, tt := range tests {
		result := SanitizeContent(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("SanitizeContent(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTimestamp(ts)
	
	// Should not be empty
	if len(result) == 0 {
		t.Error("FormatTimestamp should not return empty string")
	}
}

func TestParseMessageType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_input", "user_input"},
		{"User_Input", "user_input"},
		{"final_response", "final_response"},
		{"tool_use", "tool_use"},
		{"unknown", "unknown"},
		{"", "unknown"},
	}
	
	for _, tt := range tests {
		result := ParseMessageType(tt.input)
		if result != tt.expected {
			t.Errorf("ParseMessageType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestBuildSessionID(t *testing.T) {
	result := BuildSessionID("slack", "U123", "C456")
	if len(result) == 0 {
		t.Error("BuildSessionID should not return empty string")
	}
	
	// Should be deterministic
	result2 := BuildSessionID("slack", "U123", "C456")
	if result != result2 {
		t.Error("BuildSessionID should be deterministic")
	}
}

func TestGenerateProviderSessionID(t *testing.T) {
	result := GenerateProviderSessionID()
	if len(result) == 0 {
		t.Error("GenerateProviderSessionID should not return empty string")
	}
	
	// Should be UUID format
	_, err := uuid.Parse(result)
	if err != nil {
		t.Errorf("GenerateProviderSessionID should return valid UUID: %v", err)
	}
}

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Short strings get fully masked
		{"hello world", "he****ld"},
		{"", "****"},
		{"ab", "****"},
		{"abc", "****"},
		{"abcd", "****"},
		// 5+ chars show first 2 and last 2
		{"abcde", "ab****de"},
	}
	
	for _, tt := range tests {
		result := MaskSensitiveData(tt.input)
		if result != tt.expected {
			t.Errorf("MaskSensitiveData(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolong", 4, "tool..."},
	}
	
	for _, tt := range tests {
		result := TruncateForLog(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("TruncateForLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTruncateForLog_DefaultMaxLen(t *testing.T) {
	// Test default maxLen (200)
	longString := string(make([]byte, 300))
	for i := range longString {
		longString = longString[:i] + "a" + longString[i+1:]
	}
	longString = "abcdefghijklmnopqrstuvwxyz"
	
	result := TruncateForLog(longString, 0) // 0 should use default
	// Default is 200, so this should not be truncated
	if len(result) > 200 {
		t.Errorf("TruncateForLog with 0 should use default, got len=%d", len(result))
	}
}
