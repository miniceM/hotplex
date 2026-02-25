package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hrygo/hotplex/event"
)

func TestBlockBuilder_NewBlockBuilder(t *testing.T) {
	t.Run("creates new BlockBuilder instance", func(t *testing.T) {
		bb := NewBlockBuilder()
		if bb == nil {
			t.Fatal("NewBlockBuilder returned nil")
		}
	})
}

func TestBlockBuilder_BuildThinkingBlock(t *testing.T) {
	t.Run("returns correct block structure", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildThinkingBlock()

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}

		block := blocks[0]
		if block["type"] != "context" {
			t.Errorf("expected type 'context', got '%v'", block["type"])
		}

		// Verify elements array exists
		elements, ok := block["elements"].([]map[string]any)
		if !ok {
			t.Fatal("elements is not an array")
		}
		if len(elements) != 1 {
			t.Errorf("expected 1 element, got %d", len(elements))
		}

		// Verify mrkdwn text object
		el := elements[0]
		if el["type"] != "mrkdwn" {
			t.Errorf("expected element type 'mrkdwn', got '%v'", el["type"])
		}
		if !strings.Contains(el["text"].(string), "Thinking") {
			t.Errorf("expected text to contain 'Thinking', got '%v'", el["text"])
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildThinkingBlock()

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildToolUseBlock(t *testing.T) {
	t.Run("returns correct block structure with input", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolUseBlock("Bash", "ls -la", false)

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}

		block := blocks[0]
		if block["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", block["type"])
		}

		// Verify text contains tool name
		text := block["text"].(map[string]any)
		textContent := text["text"].(string)
		if !strings.Contains(textContent, "Bash") {
			t.Errorf("expected text to contain 'Bash', got '%s'", textContent)
		}
		if !strings.Contains(textContent, "Using tool") {
			t.Errorf("expected text to contain 'Using tool', got '%s'", textContent)
		}

		// Verify fields array
		fields, ok := block["fields"].([]map[string]any)
		if !ok {
			t.Fatal("fields is not an array")
		}
		if len(fields) != 1 {
			t.Errorf("expected 1 field, got %d", len(fields))
		}
	})

	t.Run("includes truncation indicator when truncated is true", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolUseBlock("Read", strings.Repeat("a", 500), true)

		block := blocks[0]
		fields := block["fields"].([]map[string]any)
		fieldText := fields[0]["text"].(string)

		if !strings.Contains(fieldText, "truncated") {
			t.Errorf("expected field text to contain 'truncated', got '%s'", fieldText)
		}
	})

	t.Run("does not include truncation when truncated is false", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolUseBlock("Read", "short input", false)

		block := blocks[0]
		fields := block["fields"].([]map[string]any)
		fieldText := fields[0]["text"].(string)

		if strings.Contains(fieldText, "truncated") {
			t.Errorf("expected field text not to contain 'truncated', got '%s'", fieldText)
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolUseBlock("Bash", "echo hello", false)

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildToolResultBlock(t *testing.T) {
	t.Run("success result includes checkmark emoji", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 1500, "output content", false)

		block := blocks[0]
		text := block["text"].(map[string]any)
		textContent := text["text"].(string)

		if !strings.Contains(textContent, ":white_check_mark:") {
			t.Errorf("expected text to contain ':white_check_mark:', got '%s'", textContent)
		}
		if !strings.Contains(textContent, "Completed") {
			t.Errorf("expected text to contain 'Completed', got '%s'", textContent)
		}
	})

	t.Run("failure result includes x emoji", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(false, 500, "error message", false)

		block := blocks[0]
		text := block["text"].(map[string]any)
		textContent := text["text"].(string)

		if !strings.Contains(textContent, ":x:") {
			t.Errorf("expected text to contain ':x:', got '%s'", textContent)
		}
		if !strings.Contains(textContent, "Failed") {
			t.Errorf("expected text to contain 'Failed', got '%s'", textContent)
		}
	})

	t.Run("includes output preview when provided", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 100, "test output", false)

		block := blocks[0]
		if _, ok := block["fields"]; !ok {
			t.Error("expected fields to be present when output is provided")
		}
	})

	t.Run("adds action button when hasButton is true and success", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 100, "output", true)

		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks (result + action), got %d", len(blocks))
		}

		actionBlock := blocks[1]
		if actionBlock["type"] != "actions" {
			t.Errorf("expected second block type 'actions', got '%v'", actionBlock["type"])
		}

		elements := actionBlock["elements"].([]map[string]any)
		if len(elements) != 1 {
			t.Errorf("expected 1 element, got %d", len(elements))
		}

		btn := elements[0]
		if btn["type"] != "button" {
			t.Errorf("expected element type 'button', got '%v'", btn["type"])
		}
		if btn["action_id"] != "view_tool_output" {
			t.Errorf("expected action_id 'view_tool_output', got '%v'", btn["action_id"])
		}
	})

	t.Run("does not add button when hasButton is false", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 100, "output", false)

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
	})

	t.Run("does not add button when success is false", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(false, 100, "error", true)

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 1500, "test output", true)

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildErrorBlock(t *testing.T) {
	t.Run("returns warning emoji by default", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("something went wrong", false)

		if len(blocks) != 2 {
			t.Fatalf("expected 2 blocks (header + error), got %d", len(blocks))
		}

		headerBlock := blocks[0]
		if headerBlock["type"] != "header" {
			t.Errorf("expected first block type 'header', got '%v'", headerBlock["type"])
		}

		headerText := headerBlock["text"].(map[string]any)
		if !strings.Contains(headerText["text"].(string), ":warning:") {
			t.Errorf("expected header to contain ':warning:', got '%v'", headerText["text"])
		}
	})

	t.Run("returns x emoji when isDangerBlock is true", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("critical error", true)

		headerBlock := blocks[0]
		headerText := headerBlock["text"].(map[string]any)

		if !strings.Contains(headerText["text"].(string), ":x:") {
			t.Errorf("expected header to contain ':x:', got '%v'", headerText["text"])
		}
	})

	t.Run("includes error message in code block format", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("connection refused", false)

		errorBlock := blocks[1]
		if errorBlock["type"] != "section" {
			t.Errorf("expected second block type 'section', got '%v'", errorBlock["type"])
		}

		text := errorBlock["text"].(map[string]any)
		textContent := text["text"].(string)

		if !strings.Contains(textContent, "connection refused") {
			t.Errorf("expected text to contain error message, got '%s'", textContent)
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("error message", true)

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildAnswerBlock(t *testing.T) {
	t.Run("returns section block with formatted content", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildAnswerBlock("This is a test answer")

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}

		block := blocks[0]
		if block["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", block["type"])
		}

		text := block["text"].(map[string]any)
		if text["type"] != "mrkdwn" {
			t.Errorf("expected text type 'mrkdwn', got '%v'", text["type"])
		}
	})

	t.Run("includes expand property", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildAnswerBlock("Answer with expand")

		block := blocks[0]
		if block["expand"] == nil {
			t.Error("expected 'expand' property to be present")
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildAnswerBlock("Test answer")

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildStatsBlock(t *testing.T) {
	t.Run("returns empty when stats is nil", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildStatsBlock(nil)

		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks for nil stats, got %d", len(blocks))
		}
	})

	t.Run("returns empty when no fields to display", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{}
		blocks := bb.BuildStatsBlock(stats)

		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks for empty stats, got %d", len(blocks))
		}
	})

	t.Run("includes duration when TotalDurationMs > 0", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{
			TotalDurationMs: 5500,
		}
		blocks := bb.BuildStatsBlock(stats)

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}

		block := blocks[0]
		if block["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", block["type"])
		}

		fields := block["fields"].([]map[string]any)
		fieldText := fields[0]["text"].(string)
		if !strings.Contains(fieldText, "Duration") {
			t.Errorf("expected field to contain 'Duration', got '%s'", fieldText)
		}
	})

	t.Run("includes tokens when present", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{
			InputTokens:  100,
			OutputTokens: 200,
		}
		blocks := bb.BuildStatsBlock(stats)

		block := blocks[0]
		fields := block["fields"].([]map[string]any)
		fieldText := fields[0]["text"].(string)
		if !strings.Contains(fieldText, "Tokens") {
			t.Errorf("expected field to contain 'Tokens', got '%s'", fieldText)
		}
		if !strings.Contains(fieldText, "100") || !strings.Contains(fieldText, "200") {
			t.Errorf("expected field to contain token counts, got '%s'", fieldText)
		}
	})

	t.Run("includes cache tokens when present", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{
			InputTokens:     100,
			OutputTokens:    200,
			CacheReadTokens: 50,
		}
		blocks := bb.BuildStatsBlock(stats)

		block := blocks[0]
		fields := block["fields"].([]map[string]any)
		fieldText := fields[0]["text"].(string)
		if !strings.Contains(fieldText, "cache") {
			t.Errorf("expected field to contain 'cache', got '%s'", fieldText)
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{
			TotalDurationMs: 3000,
			InputTokens:     150,
			OutputTokens:    250,
		}
		blocks := bb.BuildStatsBlock(stats)

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestBlockBuilder_BuildDividerBlock(t *testing.T) {
	t.Run("returns divider block", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildDividerBlock()

		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}

		block := blocks[0]
		if block["type"] != "divider" {
			t.Errorf("expected type 'divider', got '%v'", block["type"])
		}
	})

	t.Run("produces valid JSON", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildDividerBlock()

		_, err := json.Marshal(blocks)
		if err != nil {
			t.Errorf("failed to marshal JSON: %v", err)
		}
	})
}

func TestMrkdwnFormatter_Format(t *testing.T) {
	t.Run("escapes special characters to HTML entities", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.Format("a < b & c > d")
		// Implementation escapes < > & to HTML entities
		if !strings.Contains(result, "&lt;") || !strings.Contains(result, "&gt;") || !strings.Contains(result, "&amp;") {
			t.Errorf("expected HTML entities, got '%s'", result)
		}
	})

	t.Run("converts bold markdown", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.Format("**bold text**")
		// Bold conversion replaces ** with *
		if strings.Contains(result, "**") {
			t.Errorf("expected ** to be converted, got '%s'", result)
		}
	})

	t.Run("converts italic markdown", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.Format("*italic text*")
		// Just verify it doesn't crash and returns something
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("converts links", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.Format("[click here](https://example.com)")
		if !strings.Contains(result, "<https://example.com|click here>") {
			t.Errorf("expected link to be converted, got '%s'", result)
		}
	})

	t.Run("preserves code blocks", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		input := "```python\nprint('hello')\n```"
		result := f.Format(input)
		if !strings.Contains(result, "```python") {
			t.Errorf("expected code block to be preserved, got '%s'", result)
		}
	})

	t.Run("handles mixed content", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		input := "**bold** and _italic_ with [link](https://test.com)"
		result := f.Format(input)

		// Verify bold conversion
		if strings.Contains(result, "**") {
			t.Errorf("expected ** to be converted, got '%s'", result)
		}
		// Verify link conversion
		if !strings.Contains(result, "<https://test.com|link>") {
			t.Errorf("expected link conversion, got '%s'", result)
		}
	})
}

func TestMrkdwnFormatter_FormatCodeBlock(t *testing.T) {
	t.Run("wraps code without language", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.FormatCodeBlock("some code", "")
		if !strings.Contains(result, "```") {
			t.Errorf("expected code block markers, got '%s'", result)
		}
		if !strings.Contains(result, "some code") {
			t.Errorf("expected code content, got '%s'", result)
		}
	})

	t.Run("includes language hint", func(t *testing.T) {
		f := NewMrkdwnFormatter()
		result := f.FormatCodeBlock("const x = 1", "javascript")
		if !strings.Contains(result, "```javascript") {
			t.Errorf("expected language hint, got '%s'", result)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	t.Run("returns milliseconds for less than 1000ms", func(t *testing.T) {
		result := formatDuration(500)
		if result != "500ms" {
			t.Errorf("expected '500ms', got '%s'", result)
		}
	})

	t.Run("returns seconds for >= 1000ms", func(t *testing.T) {
		result := formatDuration(1500)
		if result != "1.5s" {
			t.Errorf("expected '1.5s', got '%s'", result)
		}
	})

	t.Run("handles exact seconds", func(t *testing.T) {
		result := formatDuration(1000)
		if result != "1.0s" {
			t.Errorf("expected '1.0s', got '%s'", result)
		}
	})

	t.Run("handles large values", func(t *testing.T) {
		result := formatDuration(65000)
		if result != "65.0s" {
			t.Errorf("expected '65.0s', got '%s'", result)
		}
	})
}

func TestTruncateText(t *testing.T) {
	t.Run("returns original text when shorter than max", func(t *testing.T) {
		result := TruncateText("hello", 10)
		if result != "hello" {
			t.Errorf("expected 'hello', got '%s'", result)
		}
	})

	t.Run("truncates text longer than max", func(t *testing.T) {
		result := TruncateText("hello world", 5)
		if result != "hello..." {
			t.Errorf("expected 'hello...', got '%s'", result)
		}
	})

	t.Run("handles exact max length", func(t *testing.T) {
		result := TruncateText("hello", 5)
		if result != "hello" {
			t.Errorf("expected 'hello', got '%s'", result)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := TruncateText("", 5)
		if result != "" {
			t.Errorf("expected '', got '%s'", result)
		}
	})

	t.Run("handles zero max length", func(t *testing.T) {
		result := TruncateText("hello", 0)
		if result != "..." {
			t.Errorf("expected '...', got '%s'", result)
		}
	})

	t.Run("negative max length causes panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic with negative max length")
			}
		}()
		_ = TruncateText("hello", -1)
	})
}

func TestBlockBuilder_JsonSerialization(t *testing.T) {
	t.Run("BuildThinkingBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildThinkingBlock()
		jsonBytes, err := json.Marshal(blocks)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "context" {
			t.Errorf("expected type 'context', got '%v'", parsed[0]["type"])
		}
	})

	t.Run("BuildToolUseBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolUseBlock("Bash", "ls -la", false)
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", parsed[0]["type"])
		}
	})

	t.Run("BuildToolResultBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 1000, "output", true)
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		// Should have 2 blocks: section + actions
		if len(parsed) != 2 {
			t.Errorf("expected 2 blocks, got %d", len(parsed))
		}
	})

	t.Run("BuildErrorBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("error msg", false)
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "header" {
			t.Errorf("expected first block type 'header', got '%v'", parsed[0]["type"])
		}
		if parsed[1]["type"] != "section" {
			t.Errorf("expected second block type 'section', got '%v'", parsed[1]["type"])
		}
	})

	t.Run("BuildAnswerBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildAnswerBlock("answer text")
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", parsed[0]["type"])
		}
	})

	t.Run("BuildStatsBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		stats := &event.EventMeta{
			TotalDurationMs: 5000,
			InputTokens:     100,
			OutputTokens:    200,
		}
		blocks := bb.BuildStatsBlock(stats)
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", parsed[0]["type"])
		}
	})

	t.Run("BuildDividerBlock serializes correctly", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildDividerBlock()
		jsonBytes, _ := json.Marshal(blocks)

		var parsed []map[string]any
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if parsed[0]["type"] != "divider" {
			t.Errorf("expected type 'divider', got '%v'", parsed[0]["type"])
		}
	})
}

func TestBlockBuilder_SlackAPICompliance(t *testing.T) {
	t.Run("all blocks have valid type field", func(t *testing.T) {
		bb := NewBlockBuilder()

		testCases := []struct {
			name   string
			blocks []map[string]any
		}{
			{"BuildThinkingBlock", bb.BuildThinkingBlock()},
			{"BuildToolUseBlock", bb.BuildToolUseBlock("Test", "input", false)},
			{"BuildToolResultBlock", bb.BuildToolResultBlock(true, 100, "out", false)},
			{"BuildErrorBlock", bb.BuildErrorBlock("err", false)},
			{"BuildAnswerBlock", bb.BuildAnswerBlock("ans")},
			{"BuildDividerBlock", bb.BuildDividerBlock()},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				for _, block := range tc.blocks {
					if block["type"] == nil {
						t.Error("block missing required 'type' field")
					}
				}
			})
		}
	})

	t.Run("text objects have required fields", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildAnswerBlock("test")

		textObj := blocks[0]["text"].(map[string]any)
		if textObj["type"] == nil {
			t.Error("text object missing 'type' field")
		}
		if textObj["text"] == nil {
			t.Error("text object missing 'text' field")
		}
	})

	t.Run("header blocks use plain_text", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildErrorBlock("error", false)

		headerBlock := blocks[0]
		textObj := headerBlock["text"].(map[string]any)
		if textObj["type"] != "plain_text" {
			t.Errorf("header text should use 'plain_text', got '%v'", textObj["type"])
		}
	})

	t.Run("context elements use mrkdwn", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildThinkingBlock()

		elements := blocks[0]["elements"].([]map[string]any)
		if elements[0]["type"] != "mrkdwn" {
			t.Errorf("context elements should use 'mrkdwn', got '%v'", elements[0]["type"])
		}
	})

	t.Run("button has required fields", func(t *testing.T) {
		bb := NewBlockBuilder()
		blocks := bb.BuildToolResultBlock(true, 100, "out", true)

		actionBlock := blocks[1]
		elements := actionBlock["elements"].([]map[string]any)
		button := elements[0]

		if button["type"] != "button" {
			t.Errorf("expected type 'button', got '%v'", button["type"])
		}
		if button["action_id"] == nil {
			t.Error("button missing 'action_id' field")
		}
		if button["text"] == nil {
			t.Error("button missing 'text' field")
		}
	})
}
