package hooks

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func TestManager_RegisterAndEmit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()

	var receivedEvents []*Event
	var mu sync.Mutex

	testHook := &mockHook{
		name:   "test-hook",
		events: []EventType{EventSessionStart, EventSessionEnd},
		handler: func(ctx context.Context, event *Event) error {
			mu.Lock()
			defer mu.Unlock()
			receivedEvents = append(receivedEvents, event)
			return nil
		},
	}

	mgr.Register(testHook, HookConfig{Enabled: true})

	mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test-1"})
	mgr.Emit(&Event{Type: EventSessionEnd, SessionID: "test-1"})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(receivedEvents) != 2 {
		t.Errorf("Expected 2 events, got %d", len(receivedEvents))
	}
	mu.Unlock()
}

func TestManager_EmitSync(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()

	var receivedEvents []*Event
	var mu sync.Mutex

	testHook := &mockHook{
		name:   "sync-hook",
		events: []EventType{EventSessionStart},
		handler: func(ctx context.Context, event *Event) error {
			mu.Lock()
			defer mu.Unlock()
			receivedEvents = append(receivedEvents, event)
			return nil
		},
	}

	mgr.Register(testHook, HookConfig{Enabled: true, Async: false})

	ctx := context.Background()
	mgr.EmitSync(ctx, &Event{Type: EventSessionStart, SessionID: "sync-test"})

	mu.Lock()
	if len(receivedEvents) != 1 {
		t.Errorf("Expected 1 event, got %d", len(receivedEvents))
	}
	mu.Unlock()
}

func TestManager_Unregister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()

	var callCount int
	var mu sync.Mutex

	testHook := &mockHook{
		name:   "unregister-hook",
		events: []EventType{EventSessionStart},
		handler: func(ctx context.Context, event *Event) error {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			return nil
		},
	}

	mgr.Register(testHook, HookConfig{Enabled: true})
	mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test-1"})
	time.Sleep(50 * time.Millisecond)

	mgr.Unregister("unregister-hook")
	mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test-2"})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 call after unregister, got %d", callCount)
	}
	mu.Unlock()
}

func TestWebhookHook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	hook := NewWebhookHook("test-webhook", WebhookConfig{
		URL:     "http://example.com/webhook",
		Timeout: 1 * time.Second,
	}, logger)

	if hook.Name() != "test-webhook" {
		t.Errorf("Expected name 'test-webhook', got %s", hook.Name())
	}

	if len(hook.Events()) == 0 {
		t.Error("Expected default events to be set")
	}
}

func TestLoggingHook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	hook := NewLoggingHook("test-logging", logger, nil)

	if hook.Name() != "test-logging" {
		t.Errorf("Expected name 'test-logging', got %s", hook.Name())
	}

	ctx := context.Background()
	event := &Event{Type: EventSessionStart, SessionID: "test-1"}

	if err := hook.Handle(ctx, event); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

type mockHook struct {
	name    string
	events  []EventType
	handler func(ctx context.Context, event *Event) error
}

func (h *mockHook) Name() string {
	return h.name
}

func (h *mockHook) Events() []EventType {
	return h.events
}

func (h *mockHook) Handle(ctx context.Context, event *Event) error {
	if h.handler != nil {
		return h.handler(ctx, event)
	}
	return nil
}
