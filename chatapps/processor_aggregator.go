package chatapps

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/hrygo/hotplex/chatapps/base"
)

// MessageAggregatorProcessor aggregates multiple rapid messages into one
type MessageAggregatorProcessor struct {
	logger *slog.Logger

	// Buffer for aggregating messages
	buffers map[string]*messageBuffer
	mu      sync.Mutex

	// Configuration
	window     time.Duration // Time window for aggregation
	minContent int           // Minimum content difference to trigger send

	// Sender for flushing aggregated messages
	sender AggregatedMessageSender
}

// AggregatedMessageSender sends aggregated messages
type AggregatedMessageSender interface {
	SendAggregatedMessage(ctx context.Context, msg *base.ChatMessage) error
}

// messageBuffer holds buffered messages for aggregation
type messageBuffer struct {
	messages  []*base.ChatMessage
	createdAt time.Time
	timer     *time.Timer
	done      chan *base.ChatMessage
}

// MessageAggregatorProcessorOptions configures the aggregator
type MessageAggregatorProcessorOptions struct {
	Window     time.Duration // Time window to wait for more messages
	MinContent int           // Minimum characters before sending immediately
}

// NewMessageAggregatorProcessor creates a new MessageAggregatorProcessor
func NewMessageAggregatorProcessor(logger *slog.Logger, opts MessageAggregatorProcessorOptions) *MessageAggregatorProcessor {
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if opts.Window == 0 {
		opts.Window = 100 * time.Millisecond
	}
	if opts.MinContent == 0 {
		opts.MinContent = 200
	}

	return &MessageAggregatorProcessor{
		logger:     logger,
		buffers:    make(map[string]*messageBuffer),
		window:     opts.Window,
		minContent: opts.MinContent,
	}
}

// SetSender sets the sender for flushing aggregated messages
func (p *MessageAggregatorProcessor) SetSender(sender AggregatedMessageSender) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sender = sender
}

// Name returns the processor name
func (p *MessageAggregatorProcessor) Name() string {
	return "MessageAggregatorProcessor"
}

// Order returns the processor order
func (p *MessageAggregatorProcessor) Order() int {
	return int(OrderAggregation)
}

// Process aggregates messages
func (p *MessageAggregatorProcessor) Process(ctx context.Context, msg *base.ChatMessage) (*base.ChatMessage, error) {
	if msg == nil || msg.Metadata == nil {
		return msg, nil
	}

	// Check if this is a stream message
	isStream, _ := msg.Metadata["stream"].(bool)
	if !isStream {
		return msg, nil
	}

	// Check if this is the final message
	isFinal, _ := msg.Metadata["is_final"].(bool)
	if isFinal {
		return p.flushBuffer(msg)
	}

	// Check content length - send immediately if long enough
	if len(msg.Content) >= p.minContent {
		return msg, nil
	}

	// Buffer the message
	return p.bufferMessage(ctx, msg)
}

// bufferMessage adds message to buffer and returns nil (will be sent later)
func (p *MessageAggregatorProcessor) bufferMessage(ctx context.Context, msg *base.ChatMessage) (*base.ChatMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sessionKey := msg.Platform + ":" + msg.SessionID

	buf, exists := p.buffers[sessionKey]
	if !exists {
		buf = &messageBuffer{
			messages:  make([]*base.ChatMessage, 0, 10),
			createdAt: time.Now(),
			done:      make(chan *base.ChatMessage, 1),
		}

		// Set timer to flush buffer after window
		buf.timer = time.AfterFunc(p.window, func() {
			p.flushBufferByTimer(sessionKey)
		})

		p.buffers[sessionKey] = buf
	}

	buf.messages = append(buf.messages, msg)

	p.logger.Debug("Message buffered for aggregation",
		"session_key", sessionKey,
		"buffer_size", len(buf.messages),
		"content_len", len(msg.Content))

	// Return nil to indicate message is buffered (not sent yet)
	return nil, nil
}

// flushBufferByTimer flushes buffer when timer expires
func (p *MessageAggregatorProcessor) flushBufferByTimer(sessionKey string) {
	p.mu.Lock()
	buf, exists := p.buffers[sessionKey]
	sender := p.sender
	if !exists {
		p.mu.Unlock()
		return
	}

	// Remove buffer
	delete(p.buffers, sessionKey)
	p.mu.Unlock()

	// Aggregate messages
	aggregated := p.aggregateMessages(buf.messages)
	if aggregated == nil {
		return
	}

	// Send via sender if available
	if sender != nil {
		p.logger.Info("Flushing aggregated message via sender",
			"session_key", sessionKey,
			"messages_count", len(buf.messages),
			"content_len", len(aggregated.Content))

		if err := sender.SendAggregatedMessage(context.Background(), aggregated); err != nil {
			p.logger.Error("Failed to send aggregated message",
				"session_key", sessionKey,
				"error", err)
		}
	} else {
		p.logger.Warn("No sender configured, aggregated message dropped",
			"session_key", sessionKey,
			"messages_count", len(buf.messages))
	}
}

// flushBuffer flushes buffer for final message
func (p *MessageAggregatorProcessor) flushBuffer(finalMsg *base.ChatMessage) (*base.ChatMessage, error) {
	sessionKey := finalMsg.Platform + ":" + finalMsg.SessionID

	p.mu.Lock()
	buf, exists := p.buffers[sessionKey]
	if !exists {
		p.mu.Unlock()
		return finalMsg, nil
	}

	// Stop timer
	if buf.timer != nil {
		buf.timer.Stop()
	}

	// Add final message
	buf.messages = append(buf.messages, finalMsg)

	// Remove buffer
	delete(p.buffers, sessionKey)
	p.mu.Unlock()

	// Aggregate all messages
	aggregated := p.aggregateMessages(buf.messages)

	p.logger.Debug("Buffer flushed",
		"session_key", sessionKey,
		"messages_count", len(buf.messages),
		"aggregated_len", len(aggregated.Content))

	return aggregated, nil
}

// aggregateMessages combines multiple messages into one
func (p *MessageAggregatorProcessor) aggregateMessages(messages []*base.ChatMessage) *base.ChatMessage {
	if len(messages) == 0 {
		return nil
	}

	if len(messages) == 1 {
		return messages[0]
	}

	// Use first message as base
	first := messages[0]

	// Combine content
	var combined strings.Builder
	combined.Grow(len(first.Content) * len(messages))

	for i, msg := range messages {
		if i > 0 {
			combined.WriteString("\n")
		}
		combined.WriteString(msg.Content)
	}

	// Create aggregated message
	aggregated := &base.ChatMessage{
		Platform:    first.Platform,
		SessionID:   first.SessionID,
		UserID:      first.UserID,
		Content:     combined.String(),
		MessageID:   first.MessageID,
		Timestamp:   first.Timestamp,
		Metadata:    first.Metadata,
		RichContent: first.RichContent,
	}

	// Merge RichContent from all messages
	if len(messages) > 1 {
		aggregated.RichContent = p.mergeRichContent(messages)
	}

	return aggregated
}

// mergeRichContent merges RichContent from multiple messages
func (p *MessageAggregatorProcessor) mergeRichContent(messages []*base.ChatMessage) *base.RichContent {
	// Get first non-nil RichContent for default values
	var firstRichContent *base.RichContent
	for _, msg := range messages {
		if msg.RichContent != nil {
			firstRichContent = msg.RichContent
			break
		}
	}

	// If no RichContent found, return a default one
	if firstRichContent == nil {
		return &base.RichContent{
			Attachments: make([]base.Attachment, 0),
			Reactions:   make([]base.Reaction, 0),
			Blocks:      make([]any, 0),
			Embeds:      make([]any, 0),
		}
	}

	merged := &base.RichContent{
		ParseMode:      firstRichContent.ParseMode,
		Attachments:    make([]base.Attachment, 0),
		Reactions:      make([]base.Reaction, 0),
		Blocks:         make([]any, 0),
		Embeds:         make([]any, 0),
		InlineKeyboard: firstRichContent.InlineKeyboard,
	}

	seenReactions := make(map[string]bool)

	for _, msg := range messages {
		if msg.RichContent == nil {
			continue
		}

		// Merge attachments
		merged.Attachments = append(merged.Attachments, msg.RichContent.Attachments...)

		// Merge reactions (deduplicate)
		for _, reaction := range msg.RichContent.Reactions {
			key := reaction.Name
			if !seenReactions[key] {
				merged.Reactions = append(merged.Reactions, reaction)
				seenReactions[key] = true
			}
		}

		// Merge blocks
		merged.Blocks = append(merged.Blocks, msg.RichContent.Blocks...)

		// Merge embeds
		merged.Embeds = append(merged.Embeds, msg.RichContent.Embeds...)
	}

	return merged
}

// Stop stops the aggregator and cleans up buffers
func (p *MessageAggregatorProcessor) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, buf := range p.buffers {
		if buf.timer != nil {
			buf.timer.Stop()
		}
	}

	p.buffers = make(map[string]*messageBuffer)
	p.logger.Info("Message aggregator stopped")
}
