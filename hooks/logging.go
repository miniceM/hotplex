package hooks

import (
	"context"
	"log/slog"
)

type LoggingHook struct {
	name   string
	logger *slog.Logger
	events []EventType
}

func NewLoggingHook(name string, logger *slog.Logger, events []EventType) *LoggingHook {
	if len(events) == 0 {
		events = []EventType{
			EventSessionStart,
			EventSessionEnd,
			EventSessionError,
			EventDangerBlocked,
		}
	}

	return &LoggingHook{
		name:   name,
		logger: logger,
		events: events,
	}
}

func (h *LoggingHook) Name() string {
	return h.name
}

func (h *LoggingHook) Events() []EventType {
	return h.events
}

func (h *LoggingHook) Handle(ctx context.Context, event *Event) error {
	h.logger.Info("Event received",
		"event_type", event.Type,
		"timestamp", event.Timestamp,
		"namespace", event.Namespace,
		"session_id", event.SessionID,
		"error", event.Error)
	return nil
}
