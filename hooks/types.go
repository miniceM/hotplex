package hooks

import (
	"context"
	"time"
)

type EventType string

const (
	EventSessionStart  EventType = "session.start"
	EventSessionEnd    EventType = "session.end"
	EventSessionError  EventType = "session.error"
	EventToolUse       EventType = "tool.use"
	EventToolResult    EventType = "tool.result"
	EventDangerBlocked EventType = "danger.blocked"
	EventStreamStart   EventType = "stream.start"
	EventStreamEnd     EventType = "stream.end"
	EventTurnStart     EventType = "turn.start"
	EventTurnEnd       EventType = "turn.end"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Namespace string      `json:"namespace,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type Hook interface {
	Name() string
	Handle(ctx context.Context, event *Event) error
	Events() []EventType
}

type HookConfig struct {
	Async   bool          `json:"async"`
	Timeout time.Duration `json:"timeout"`
	Retry   int           `json:"retry"`
	Enabled bool          `json:"enabled"`
}

type HookRegistration struct {
	Hook   Hook
	Config HookConfig
}
