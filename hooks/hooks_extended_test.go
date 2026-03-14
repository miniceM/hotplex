package hooks

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// ========================================
// Additional Manager Tests
// ========================================

func TestManager_NewManager_Defaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Test with zero buffer size
	mgr := NewManager(logger, 0)
	defer mgr.Close()
	
	// Should use default buffer size
	if mgr == nil {
		t.Error("NewManager should not return nil")
	}
}

func TestManager_NewManager_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	mgr := NewManager(nil, 100)
	defer mgr.Close()
	
	if mgr == nil {
		t.Error("NewManager should handle nil logger")
	}
}

func TestManager_RegisteredHooks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	// Initially should be empty
	hooks := mgr.RegisteredHooks()
	if len(hooks) != 0 {
		t.Errorf("Expected empty hooks map, got %d", len(hooks))
	}
	
	// Register a hook
	testHook := &testHookImpl{name: "test-hook", events: []EventType{EventSessionStart}}
	mgr.Register(testHook, HookConfig{Enabled: true})
	
	hooks = mgr.RegisteredHooks()
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
	
	if _, ok := hooks["test-hook"]; !ok {
		t.Error("Expected test-hook to be registered")
	}
}

func TestManager_Emit_ChannelFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Small buffer to trigger full channel
	mgr := NewManager(logger, 1)
	defer mgr.Close()
	
	// Register a synchronous hook (blocks)
	blockerHook := &blockingHook{}
	mgr.Register(blockerHook, HookConfig{Enabled: true, Async: false})
	
	// Fill the channel by emitting many events
	for i := 0; i < 10; i++ {
		mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test"})
		// Give time for events to be processed
		time.Sleep(10 * time.Millisecond)
	}
	// This should not panic - Emit has a default case
}

func TestManager_ExecuteHookAsync(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	var executed bool
	var mu sync.Mutex
	
	asyncHook := &testHookImpl{
		name: "async-hook",
		events: []EventType{EventSessionStart},
		handler: func(ctx context.Context, event *Event) error {
			mu.Lock()
			executed = true
			mu.Unlock()
			return nil
		},
	}
	
	mgr.Register(asyncHook, HookConfig{Enabled: true, Async: true})
	
	mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test"})
	
	// Wait for async execution
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	if !executed {
		t.Error("Async hook should have been executed")
	}
	mu.Unlock()
}

func TestManager_HookRetry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	var attemptCount int
	
	retryHook := &testHookImpl{
		name: "retry-hook",
		events: []EventType{EventSessionStart},
		handler: func(ctx context.Context, event *Event) error {
			attemptCount++
			if attemptCount < 3 {
				return &testError{"not ready"}
			}
			return nil
		},
	}
	
	mgr.Register(retryHook, HookConfig{Enabled: true, Async: false, Retry: 3, Timeout: time.Second})
	
	mgr.EmitSync(context.Background(), &Event{Type: EventSessionStart, SessionID: "test"})
	
	if attemptCount < 3 {
		t.Errorf("Expected at least 3 attempts, got %d", attemptCount)
	}
}

func TestManager_ConcurrentRegister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			hook := &testHookImpl{
				name: "hook-" + string(rune('0'+id%10)),
				events: []EventType{EventSessionStart},
			}
			mgr.Register(hook, HookConfig{Enabled: true})
		}(i)
	}
	wg.Wait()
	
	hooks := mgr.RegisteredHooks()
	if len(hooks) == 0 {
		t.Error("Should have registered hooks")
	}
}

// ========================================
// Test Hook Implementations
// ========================================

type testHookImpl struct {
	name    string
	events  []EventType
	handler func(ctx context.Context, event *Event) error
}

func (h *testHookImpl) Name() string {
	return h.name
}

func (h *testHookImpl) Events() []EventType {
	return h.events
}

func (h *testHookImpl) Handle(ctx context.Context, event *Event) error {
	if h.handler != nil {
		return h.handler(ctx, event)
	}
	return nil
}

type blockingHook struct{}

func (h *blockingHook) Name() string        { return "blocking-hook" }
func (h *blockingHook) Events() []EventType { return []EventType{EventSessionStart} }
func (h *blockingHook) Handle(ctx context.Context, event *Event) error {
	// Block for a long time
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return nil
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// ========================================
// Event Tests
// ========================================

func TestEvent_Type(t *testing.T) {
	event := &Event{
		Type:      EventSessionStart,
		Timestamp: time.Now(),
		SessionID: "test-session",
		Data:      map[string]string{"key": "value"},
	}
	
	if event.Type != EventSessionStart {
		t.Errorf("Type = %s, want %s", event.Type, EventSessionStart)
	}
	
	if event.SessionID != "test-session" {
		t.Errorf("SessionID = %s, want test-session", event.SessionID)
	}
}

func TestEvent_DefaultTimestamp(t *testing.T) {
	event := &Event{Type: EventSessionStart}
	
	// Emit should set timestamp if zero
	mgr := NewManager(nil, 100)
	defer mgr.Close()
	
	mgr.Emit(event)
	
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set by Emit")
	}
}

// ========================================
// HookConfig Tests
// ========================================

func TestHookConfig_Defaults(t *testing.T) {
	cfg := HookConfig{}
	
	if cfg.Enabled {
		t.Error("Enabled should default to false")
	}
	if cfg.Async {
		t.Error("Async should default to false")
	}
	// Note: Timeout default is set in Manager.Register, not in HookConfig struct
	if cfg.Timeout != 0 {
		t.Error("Timeout should default to 0 (set in Register)")
	}
	if cfg.Retry != 0 {
		t.Error("Retry should default to 0")
	}
}

func TestHookConfig_Custom(t *testing.T) {
	cfg := HookConfig{
		Enabled:  true,
		Async:    true,
		Timeout:  5 * time.Second,
		Retry:    3,
	}
	
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if !cfg.Async {
		t.Error("Async should be true")
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.Retry != 3 {
		t.Errorf("Retry = %d, want 3", cfg.Retry)
	}
}

// ========================================
// EventType Constants Tests
// ========================================

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		constant EventType
		expected string
	}{
		{EventSessionStart, "session.start"},
		{EventSessionEnd, "session.end"},
		{EventSessionError, "session.error"},
		{EventToolUse, "tool.use"},
		{EventToolResult, "tool.result"},
		{EventDangerBlocked, "danger.blocked"},
		{EventStreamStart, "stream.start"},
		{EventStreamEnd, "stream.end"},
		{EventTurnStart, "turn.start"},
		{EventTurnEnd, "turn.end"},
	}
	
	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("%v = %s, want %s", tt.constant, tt.constant, tt.expected)
		}
	}
}

// ========================================
// Edge Cases
// ========================================

func TestManager_Unregister_Multiple(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	// Register same hook to multiple events
	hook := &testHookImpl{name: "multi-event-hook", events: []EventType{EventSessionStart, EventSessionEnd}}
	mgr.Register(hook, HookConfig{Enabled: true})
	
	hooks := mgr.RegisteredHooks()
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(hooks))
	}
	
	// Unregister
	mgr.Unregister("multi-event-hook")
	
	hooks = mgr.RegisteredHooks()
	if len(hooks) != 0 {
		t.Errorf("Expected 0 hooks after unregister, got %d", len(hooks))
	}
}

func TestManager_Unregister_NonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	// Should not panic
	mgr.Unregister("non-existent-hook")
}

func TestManager_DisabledHook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	var executed bool
	
	hook := &testHookImpl{
		name:    "disabled-hook",
		events:  []EventType{EventSessionStart},
		handler: func(ctx context.Context, event *Event) error {
			executed = true
			return nil
		},
	}
	
	// Register with disabled
	mgr.Register(hook, HookConfig{Enabled: false})
	
	mgr.Emit(&Event{Type: EventSessionStart, SessionID: "test"})
	time.Sleep(50 * time.Millisecond)
	
	if executed {
		t.Error("Disabled hook should not be executed")
	}
}

func TestManager_EmitSync_Context(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := NewManager(logger, 100)
	defer mgr.Close()
	
	var ctx context.Context
	
	hook := &testHookImpl{
		name:    "ctx-hook",
		events:  []EventType{EventSessionStart},
		handler: func(c context.Context, event *Event) error {
			ctx = c
			return nil
		},
	}
	
	mgr.Register(hook, HookConfig{Enabled: true})
	
	testCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	
	mgr.EmitSync(testCtx, &Event{Type: EventSessionStart, SessionID: "test"})
	
	if ctx == nil {
		t.Error("Context should be passed to handler")
	}
}
