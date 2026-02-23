package chatapps

import (
	"context"
	"time"
)

type ChatMessage struct {
	Platform  string
	SessionID string
	UserID    string
	Content   string
	MessageID string
	Timestamp time.Time
	Metadata  map[string]any
}

type ChatAdapter interface {
	Platform() string
	Start(ctx context.Context) error
	Stop() error
	SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error
	HandleMessage(ctx context.Context, msg *ChatMessage) error
}

type MessageHandler func(ctx context.Context, msg *ChatMessage) error
