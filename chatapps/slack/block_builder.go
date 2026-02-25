package slack

import (
	"fmt"
	"strings"

	"github.com/hrygo/hotplex/event"
)

// BlockBuilder builds Slack Block Kit messages for various event types
type BlockBuilder struct{}

// NewBlockBuilder creates a new BlockBuilder instance
func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{}
}

// =============================================================================
// Text Object Helpers
// =============================================================================

// mrkdwnText creates a mrkdwn text object
// Used for section, context blocks that support formatting
func mrkdwnText(text string) map[string]any {
	return map[string]any{
		"type": "mrkdwn",
		"text": text,
	}
}

// plainText creates a plain_text text object
// Used for header, button text that doesn't support formatting
func plainText(text string) map[string]any {
	return map[string]any{
		"type":  "plain_text",
		"text":  text,
		"emoji": true,
	}
}

// =============================================================================
// Mrkdwn Formatting Utilities
// =============================================================================

// MrkdwnFormatter provides utilities for converting Markdown to Slack mrkdwn format
type MrkdwnFormatter struct{}

// NewMrkdwnFormatter creates a new MrkdwnFormatter
func NewMrkdwnFormatter() *MrkdwnFormatter {
	return &MrkdwnFormatter{}
}

// Format converts Markdown text to Slack mrkdwn format
// Handles: bold, italic, code blocks, links
func (f *MrkdwnFormatter) Format(text string) string {
	result := text

	// First, escape special characters
	result = f.escapeSpecialChars(result)

	// Convert bold: **text** -> *text*
	result = f.convertBold(result)

	// Convert italic: *text* -> _text_ (but not already converted bold markers)
	result = f.convertItalic(result)

	// Convert links: [text](url) -> <url|text>
	result = f.convertLinks(result)

	return result
}

// escapeSpecialChars escapes & < > for mrkdwn
func (f *MrkdwnFormatter) escapeSpecialChars(text string) string {
	result := strings.ReplaceAll(text, "&", "&amp;")
	result = strings.ReplaceAll(result, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")
	return result
}

// convertBold converts **text** to *text*
func (f *MrkdwnFormatter) convertBold(text string) string {
	// Replace ** with * (but not ``` code blocks)
	var result strings.Builder
	inCodeBlock := false
	i := 0
	for i < len(text) {
		// Check for code block markers
		if i+3 <= len(text) && text[i:i+3] == "```" {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 3
			continue
		}

		if inCodeBlock {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Check for **
		if i+1 < len(text) && text[i:i+2] == "**" {
			result.WriteByte('*')
			i += 2
			continue
		}

		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// convertItalic converts *text* to _text_ (single asterisks only)
func (f *MrkdwnFormatter) convertItalic(text string) string {
	var result strings.Builder
	inCodeBlock := false
	inBold := false
	i := 0
	for i < len(text) {
		// Check for code block markers
		if i+3 <= len(text) && text[i:i+3] == "```" {
			inCodeBlock = !inCodeBlock
			result.WriteString("```")
			i += 3
			continue
		}

		if inCodeBlock {
			result.WriteByte(text[i])
			i++
			continue
		}

		// Track bold state (already converted to single *)
		if text[i] == '*' {
			if i+1 < len(text) && text[i+1] == '*' {
				// Double asterisk, skip (shouldn't happen after bold conversion)
				result.WriteString("**")
				i += 2
				continue
			}
			// Single asterisk - check if it's bold marker or italic
			if !inBold {
				// Check if this is likely a bold marker (non-whitespace before/after)
				isBoldMarker := (i > 0 && text[i-1] != ' ' && text[i-1] != '\n') ||
					(i+1 < len(text) && text[i+1] != ' ' && text[i+1] != '\n')
				if isBoldMarker {
					inBold = !inBold
					result.WriteByte('*')
					i++
					continue
				}
			}
			// Convert to italic
			result.WriteByte('_')
			i++
			continue
		}

		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// convertLinks converts [text](url) to <url|text>
func (f *MrkdwnFormatter) convertLinks(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Look for [
		if text[i] == '[' {
			// Find closing ]
			textEnd := strings.Index(text[i:], "]")
			if textEnd != -1 && i+textEnd+1 < len(text) && text[i+textEnd+1] == '(' {
				// Find closing )
				urlStart := i + textEnd + 2
				urlEnd := strings.Index(text[urlStart:], ")")
				if urlEnd != -1 {
					linkText := text[i+1 : i+textEnd]
					linkURL := text[urlStart : urlStart+urlEnd]
					// Write <url|text>
					result.WriteString("<")
					result.WriteString(linkURL)
					result.WriteString("|")
					result.WriteString(linkText)
					result.WriteString(">")
					i = urlStart + urlEnd + 1
					continue
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// FormatCodeBlock formats code with optional language hint
func (f *MrkdwnFormatter) FormatCodeBlock(code, language string) string {
	if language == "" {
		return fmt.Sprintf("```\n%s\n```", code)
	}
	return fmt.Sprintf("```%s\n%s\n```", language, code)
}

// =============================================================================
// Block Builders - Event Type Mappings
// =============================================================================

// BuildThinkingBlock builds a context block for thinking status
// Used for: provider.EventTypeThinking
// Strategy: Send immediately (not aggregated) for instant feedback
func (b *BlockBuilder) BuildThinkingBlock() []map[string]any {
	return []map[string]any{
		{
			"type": "context",
			"elements": []map[string]any{
				mrkdwnText(":brain: _Thinking..._"),
			},
		},
	}
}

// BuildToolUseBlock builds a section block for tool invocation
// Used for: provider.EventTypeToolUse
// Strategy: Can be aggregated with similar tool events
func (b *BlockBuilder) BuildToolUseBlock(toolName, input string, truncated bool) []map[string]any {
	// Format input as code block
	formattedInput := fmt.Sprintf("```%s```", input)

	// Add truncation indicator if needed
	if truncated {
		formattedInput += "\n*_Output truncated..._*"
	}

	return []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(fmt.Sprintf(":hammer_and_wrench: *Using tool:* `%s`", toolName)),
			"fields": []map[string]any{
				mrkdwnText("*Input:*\n" + formattedInput),
			},
		},
	}
}

// BuildToolResultBlock builds a section block for tool execution result
// Used for: provider.EventTypeToolResult
// Strategy: Can be aggregated, includes optional button to expand output
func (b *BlockBuilder) BuildToolResultBlock(success bool, durationMs int64, output string, hasButton bool) []map[string]any {
	var blocks []map[string]any

	// Build status text
	statusEmoji := ":white_check_mark:"
	statusText := "*Completed*"
	if !success {
		statusEmoji = ":x:"
		statusText = "*Failed*"
	}

	// Format duration
	durationStr := formatDuration(durationMs)

	// Main result block
	resultBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText(fmt.Sprintf("%s %s (%s)", statusEmoji, statusText, durationStr)),
	}

	// Add output preview if available (truncated to 200 chars)
	if output != "" {
		previewLen := 200
		preview := output
		if len(output) > previewLen {
			preview = output[:previewLen] + "..."
		}
		resultBlock["fields"] = []map[string]any{
			mrkdwnText("*Output:*\n```\n" + preview + "\n```"),
		}
	}

	blocks = append(blocks, resultBlock)

	// Add action button if requested
	if hasButton && success {
		actionBlock := map[string]any{
			"type": "actions",
			"elements": []map[string]any{
				{
					"type":      "button",
					"text":      plainText("View Full Output"),
					"action_id": "view_tool_output",
					"value":     "expand_output",
				},
			},
		}
		blocks = append(blocks, actionBlock)
	}

	return blocks
}

// BuildErrorBlock builds blocks for error messages
// Used for: provider.EventTypeError, danger_block
// Strategy: Send immediately (not aggregated) for critical feedback
func (b *BlockBuilder) BuildErrorBlock(message string, isDangerBlock bool) []map[string]any {
	var blocks []map[string]any

	// Header block with emoji (Slack doesn't support style: danger for headers)
	headerEmoji := ":warning:"
	if isDangerBlock {
		headerEmoji = ":x:"
	}

	headerBlock := map[string]any{
		"type": "header",
		"text": plainText(fmt.Sprintf("%s Error", headerEmoji)),
	}
	blocks = append(blocks, headerBlock)

	// Error message as section with mrkdwn
	errorBlock := map[string]any{
		"type": "section",
		"text": mrkdwnText(fmt.Sprintf("```\n%s\n```", message)),
	}
	blocks = append(blocks, errorBlock)

	return blocks
}

// BuildAnswerBlock builds a section block for AI answer text
// Used for: provider.EventTypeAnswer
// Strategy: Stream updates via chat.update, supports mrkdwn formatting
func (b *BlockBuilder) BuildAnswerBlock(content string) []map[string]any {
	// Format content with mrkdwn
	formatter := NewMrkdwnFormatter()
	formattedContent := formatter.Format(content)

	return []map[string]any{
		{
			"type": "section",
			"text": mrkdwnText(formattedContent),
			// Enable expand for AI Assistant apps
			"expand": true,
		},
	}
}

// BuildStatsBlock builds a section block with statistics
// Used for: provider.EventTypeResult (end of turn)
// Strategy: Send as final summary
func (b *BlockBuilder) BuildStatsBlock(stats *event.EventMeta) []map[string]any {
	if stats == nil {
		return []map[string]any{}
	}

	var fields []map[string]any

	// Duration field
	if stats.TotalDurationMs > 0 {
		fields = append(fields, mrkdwnText(fmt.Sprintf("*Duration:*\n%s", formatDuration(stats.TotalDurationMs))))
	}

	// Token usage field
	if stats.InputTokens > 0 || stats.OutputTokens > 0 {
		tokenStr := fmt.Sprintf("%d in / %d out", stats.InputTokens, stats.OutputTokens)
		if stats.CacheReadTokens > 0 {
			tokenStr += fmt.Sprintf(" (cache: %d)", stats.CacheReadTokens)
		}
		fields = append(fields, mrkdwnText(fmt.Sprintf("*Tokens:*\n%s", tokenStr)))
	}

	// Cost field (if available)
	// Note: Cost tracking depends on provider implementation

	if len(fields) == 0 {
		return []map[string]any{}
	}

	return []map[string]any{
		{
			"type":   "section",
			"fields": fields,
		},
	}
}

// BuildDividerBlock creates a simple divider
func (b *BlockBuilder) BuildDividerBlock() []map[string]any {
	return []map[string]any{
		{
			"type": "divider",
		},
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// formatDuration converts milliseconds to human-readable duration
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.1fs", seconds)
}

// TruncateText truncates text to max length with ellipsis
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
