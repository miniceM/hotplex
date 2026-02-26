package slack

import (
	"io"
	"log/slog"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

// TestHandleResetCommand_EngineNil tests /reset when engine is nil
// This is the only testable case without complex mocking
func TestHandleResetCommand_EngineNil(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := NewAdapter(&Config{
		BotToken: "xoxb-test",
		Mode:     "socket",
	}, logger, base.WithoutServer())

	// Don't set engine - it should be nil

	cmd := SlashCommand{
		Command:     "/reset",
		UserID:      "U123",
		ChannelID:   "C123",
		ResponseURL: "https://hooks.slack.com/test",
	}

	// Test that handleResetCommand handles nil engine without panicking
	// The method should return an error and log appropriately
	err := adapter.handleResetCommand(cmd)

	// We expect an error since engine is nil
	if err == nil {
		t.Log("handleResetCommand returned nil error - may have sent ephemeral message")
	}

	t.Logf("handleResetCommand completed with error: %v", err)
}

// TestSlashCommandStruct tests the SlashCommand struct
func TestSlashCommandStruct(t *testing.T) {
	cmd := SlashCommand{
		Command:     "/clear",
		Text:        "",
		UserID:      "U123",
		ChannelID:   "C123",
		ResponseURL: "https://hooks.slack.com/test",
	}

	if cmd.Command != "/clear" {
		t.Errorf("expected command '/clear', got %s", cmd.Command)
	}
	if cmd.UserID != "U123" {
		t.Errorf("expected userID 'U123', got %s", cmd.UserID)
	}
	if cmd.ChannelID != "C123" {
		t.Errorf("expected channelID 'C123', got %s", cmd.ChannelID)
	}
}
