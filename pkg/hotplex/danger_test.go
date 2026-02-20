package hotplex

import (
	"log/slog"
	"os"
	"testing"
)

func TestDetector_CheckInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	detector := NewDetector(logger)

	tests := []struct {
		name     string
		input    string
		isDanger bool
	}{
		{
			name:     "Safe command",
			input:    "ls -la",
			isDanger: false,
		},
		{
			name:     "Safe git command",
			input:    "git status",
			isDanger: false,
		},
		{
			name:     "Critical: rm -rf /",
			input:    "rm -rf /",
			isDanger: true,
		},
		{
			name:     "High: dd wiping disk",
			input:    "dd if=/dev/zero of=/dev/sda",
			isDanger: true,
		},
		{
			name:     "High: Fork bomb",
			input:    ":(){}|",
			isDanger: true,
		},
		{
			name:     "Moderate: git reset hard",
			input:    "git reset --hard HEAD",
			isDanger: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := detector.CheckInput(tt.input)
			if tt.isDanger && event == nil {
				t.Errorf("Expected danger detected for input %q, but got nil", tt.input)
			}
			if !tt.isDanger && event != nil {
				t.Errorf("Expected no danger for input %q, but got %v", tt.input, event.Reason)
			}
		})
	}
}

func TestDetector_Bypass(t *testing.T) {
	detector := NewDetector(nil)
	detector.SetBypassEnabled(true)

	input := "rm -rf /"
	event := detector.CheckInput(input)
	if event != nil {
		t.Error("Danger detected even when bypass is enabled")
	}

	detector.SetBypassEnabled(false)
	event = detector.CheckInput(input)
	if event == nil {
		t.Error("Danger NOT detected when bypass is disabled")
	}
}
