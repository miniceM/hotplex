package hooks

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.RWMutex
	hooks    map[EventType][]HookRegistration
	logger   *slog.Logger
	notifyCh chan *Event
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewManager(logger *slog.Logger, bufferSize int) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	m := &Manager{
		hooks:    make(map[EventType][]HookRegistration),
		logger:   logger,
		notifyCh: make(chan *Event, bufferSize),
		done:     make(chan struct{}),
	}

	m.wg.Add(1)
	go m.eventLoop()

	return m
}

func (m *Manager) Register(hook Hook, config HookConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Enabled {
		config.Enabled = true
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	reg := HookRegistration{
		Hook:   hook,
		Config: config,
	}

	for _, eventType := range hook.Events() {
		m.hooks[eventType] = append(m.hooks[eventType], reg)
	}

	m.logger.Info("Hook registered",
		"hook", hook.Name(),
		"events", hook.Events(),
		"async", config.Async)
}

func (m *Manager) Unregister(hookName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for eventType, regs := range m.hooks {
		filtered := make([]HookRegistration, 0, len(regs))
		for _, reg := range regs {
			if reg.Hook.Name() != hookName {
				filtered = append(filtered, reg)
			}
		}
		m.hooks[eventType] = filtered
	}
}

func (m *Manager) Emit(event *Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case m.notifyCh <- event:
	default:
		m.logger.Warn("Event channel full, dropping event",
			"type", event.Type,
			"session_id", event.SessionID)
	}
}

func (m *Manager) EmitSync(ctx context.Context, event *Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.executeHooks(ctx, event)
}

func (m *Manager) eventLoop() {
	defer m.wg.Done()

	for {
		select {
		case event := <-m.notifyCh:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			m.executeHooks(ctx, event)
			cancel()
		case <-m.done:
			return
		}
	}
}

func (m *Manager) executeHooks(ctx context.Context, event *Event) {
	m.mu.RLock()
	regs := m.hooks[event.Type]
	hooksCopy := make([]HookRegistration, len(regs))
	copy(hooksCopy, regs)
	m.mu.RUnlock()

	for _, reg := range hooksCopy {
		if !reg.Config.Enabled {
			continue
		}

		if reg.Config.Async {
			m.wg.Add(1)
			go m.executeHookAsync(ctx, reg, event)
		} else {
			m.executeHookSync(ctx, reg, event)
		}
	}
}

func (m *Manager) executeHookSync(ctx context.Context, reg HookRegistration, event *Event) {
	hookCtx, cancel := context.WithTimeout(ctx, reg.Config.Timeout)
	defer cancel()

	var lastErr error
	for i := 0; i <= reg.Config.Retry; i++ {
		if err := reg.Hook.Handle(hookCtx, event); err != nil {
			lastErr = err
			m.logger.Warn("Hook execution failed, retrying",
				"hook", reg.Hook.Name(),
				"event_type", event.Type,
				"attempt", i+1,
				"error", err)
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return
	}

	m.logger.Error("Hook execution failed after retries",
		"hook", reg.Hook.Name(),
		"event_type", event.Type,
		"error", lastErr)
}

func (m *Manager) executeHookAsync(ctx context.Context, reg HookRegistration, event *Event) {
	defer m.wg.Done()
	m.executeHookSync(ctx, reg, event)
}

func (m *Manager) Close() {
	close(m.done)
	m.wg.Wait()
	close(m.notifyCh)
}

func (m *Manager) RegisteredHooks() map[string][]EventType {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]EventType)
	for eventType, regs := range m.hooks {
		for _, reg := range regs {
			result[reg.Hook.Name()] = append(result[reg.Hook.Name()], eventType)
		}
	}
	return result
}
