package slack

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SlackTextLimit is the maximum character limit for a single Slack message
const SlackTextLimit = 4000

// chunkMessage splits a text message into chunks that fit within Slack's limit.
// It attempts to split at word boundaries to avoid breaking words.
// Each chunk is prefixed with [chunkNum/totalChunks] for reference.
func chunkMessage(text string, limit int) []string {
	if text == "" || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	// Calculate approximate number of chunks
	runes := []rune(text)
	totalRunes := len(runes)

	var chunks []string
	chunkSize := limit - 15 // Reserve space for "[999/999]\n" prefix

	for i := 0; i < totalRunes; i += chunkSize {
		end := i + chunkSize
		if end > totalRunes {
			end = totalRunes
		}

		// Try to break at word boundary
		if end < totalRunes {
			chunk := string(runes[i:end])
			lastSpace := strings.LastIndex(chunk, "\n")
			if lastSpace > 0 {
				// Break at newline if possible
				end = i + lastSpace + 1
			} else {
				lastSpace = strings.LastIndex(chunk, " ")
				if lastSpace > chunkSize/2 {
					// Only break at space if more than half the chunk is used
					end = i + lastSpace
				}
			}
		}

		chunkStr := string(runes[i:end])
		chunkStr = strings.TrimRight(chunkStr, " \t")

		chunks = append(chunks, chunkStr)
	}

	// Add chunk numbering
	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		if len(chunks) > 1 {
			result[i] = fmt.Sprintf("[%d/%d]\n%s", i+1, len(chunks), chunk)
		} else {
			result[i] = chunk
		}
	}

	return result
}
