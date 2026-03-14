package command

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hrygo/hotplex/event"
)

// ========================================
// Mock Executor for testing
// ========================================

type mockExecutor struct {
	command     string
	description string
	executeFunc func(ctx context.Context, req *Request, callback event.Callback) (*Result, error)
}

func (m *mockExecutor) Command() string {
	return m.command
}

func (m *mockExecutor) Description() string {
	return m.description
}

func (m *mockExecutor) Execute(ctx context.Context, req *Request, callback event.Callback) (*Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req, callback)
	}
	return &Result{Success: true, Message: "OK"}, nil
}

// ========================================
// Registry Tests
// ========================================

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil || r.cmds == nil {
		t.Error("NewRegistry should not return nil or with uninitialized cmds")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	
	exec := &mockExecutor{
		command:     "/test",
		description: "Test command",
	}
	
	r.Register(exec)
	
	// Verify registration
	retrieved, ok := r.Get("/test")
	if !ok {
		t.Error("Failed to retrieve registered command")
	}
	if retrieved.Command() != "/test" {
		t.Errorf("Retrieved command = %s, want /test", retrieved.Command())
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()
	
	exec1 := &mockExecutor{command: "/test", description: "First"}
	exec2 := &mockExecutor{command: "/test", description: "Second"}
	
	r.Register(exec1)
	r.Register(exec2) // Should overwrite
	
	retrieved, _ := r.Get("/test")
	if retrieved.Description() != "Second" {
		t.Error("Second registration should overwrite first")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	
	// Test non-existent command
	_, ok := r.Get("/nonexistent")
	if ok {
		t.Error("Get should return false for non-existent command")
	}
	
	// Test existent command
	r.Register(&mockExecutor{command: "/test", description: "Test"})
	_, ok = r.Get("/test")
	if !ok {
		t.Error("Get should return true for existent command")
	}
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry()
	
	executed := false
	exec := &mockExecutor{
		command: "/test",
		description: "Test command",
		executeFunc: func(ctx context.Context, req *Request, callback event.Callback) (*Result, error) {
			executed = true
			return &Result{Success: true, Message: "Executed"}, nil
		},
	}
	r.Register(exec)
	
	ctx := context.Background()
	req := &Request{Command: "/test", Text: "arg1"}
	
	result, err := r.Execute(ctx, req, nil)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if !executed {
		t.Error("Executor should have been called")
	}
	if !result.Success {
		t.Error("Result should be successful")
	}
}

func TestRegistry_Execute_UnknownCommand(t *testing.T) {
	r := NewRegistry()
	
	ctx := context.Background()
	req := &Request{Command: "/unknown"}
	
	_, err := r.Execute(ctx, req, nil)
	if err == nil {
		t.Error("Execute should fail for unknown command")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	
	// Empty registry
	list := r.List()
	if len(list) != 0 {
		t.Errorf("Empty registry should return empty list, got %d", len(list))
	}
	
	// Add commands
	r.Register(&mockExecutor{command: "/reset"})
	r.Register(&mockExecutor{command: "/dc"})
	
	list = r.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(list))
	}
	
	// Check both commands present
	found := make(map[string]bool)
	for _, cmd := range list {
		found[cmd] = true
	}
	if !found["/reset"] || !found["/dc"] {
		t.Error("Expected both /reset and /dc in list")
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := NewRegistry()
	
	// Concurrent registration
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.Register(&mockExecutor{
				command:     "/cmd" + string(rune('0'+id%10)),
				description: "Command",
			})
		}(i)
	}
	wg.Wait()
	
	// Concurrent read
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.Get("/cmd" + string(rune('0'+id%10)))
			r.List()
		}(i)
	}
	wg.Wait()
}

// ========================================
// Request Tests
// ========================================

func TestRequest_Fields(t *testing.T) {
	req := &Request{
		Command:           "/reset",
		Text:              "hard",
		UserID:            "U123",
		ChannelID:         "C456",
		ThreadTS:          "T789",
		SessionID:         "S101112",
		ProviderSessionID: "P131415",
		Metadata:          map[string]any{"key": "value"},
	}
	
	if req.Command != "/reset" {
		t.Errorf("Command = %s, want /reset", req.Command)
	}
	if req.Text != "hard" {
		t.Errorf("Text = %s, want hard", req.Text)
	}
	if req.UserID != "U123" {
		t.Errorf("UserID = %s, want U123", req.UserID)
	}
}

// ========================================
// Result Tests
// ========================================

func TestResult_Success(t *testing.T) {
	result := &Result{
		Success:  true,
		Message:  "Operation completed",
		Metadata: map[string]any{"count": 5},
	}
	
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Message != "Operation completed" {
		t.Errorf("Message = %s", result.Message)
	}
}

func TestResult_Failure(t *testing.T) {
	result := &Result{
		Success:  false,
		Message:  "Operation failed",
		Metadata: map[string]any{"error": "not found"},
	}
	
	if result.Success {
		t.Error("Success should be false")
	}
}

// ========================================
// ProgressStep Tests
// ========================================

func TestProgressStep(t *testing.T) {
	step := &ProgressStep{
		Name:    "find_session",
		Message: "Finding session",
		Status:  "running",
	}
	
	if step.Name != "find_session" {
		t.Errorf("Name = %s", step.Name)
	}
	if step.Status != "running" {
		t.Errorf("Status = %s", step.Status)
	}
}

// ========================================
// Command Constants Tests
// ========================================

func TestCommandConstants(t *testing.T) {
	if CommandReset != "/reset" {
		t.Errorf("CommandReset = %s, want /reset", CommandReset)
	}
	if CommandDisconnect != "/dc" {
		t.Errorf("CommandDisconnect = %s, want /dc", CommandDisconnect)
	}
}

// ========================================
// Executor Interface Tests
// ========================================

func TestExecutorInterface(t *testing.T) {
	// Verify mockExecutor implements Executor
	var _ Executor = (*mockExecutor)(nil)
}

// ========================================
// Edge Cases
// ========================================

func TestRegistry_Execute_NilCallback(t *testing.T) {
	r := NewRegistry()
	
	exec := &mockExecutor{
		command: "/test",
		description: "Test",
		executeFunc: func(ctx context.Context, req *Request, callback event.Callback) (*Result, error) {
			// Should not panic with nil callback
			return &Result{Success: true}, nil
		},
	}
	r.Register(exec)
	
	ctx := context.Background()
	req := &Request{Command: "/test"}
	
	_, err := r.Execute(ctx, req, nil)
	if err != nil {
		t.Errorf("Execute with nil callback should not fail: %v", err)
	}
}

func TestRegistry_Execute_Error(t *testing.T) {
	r := NewRegistry()
	
	exec := &mockExecutor{
		command: "/test",
		description: "Test",
		executeFunc: func(ctx context.Context, req *Request, callback event.Callback) (*Result, error) {
			return nil, errors.New("execution error")
		},
	}
	r.Register(exec)
	
	ctx := context.Background()
	req := &Request{Command: "/test"}
	
	_, err := r.Execute(ctx, req, nil)
	if err == nil {
		t.Error("Execute should propagate error")
	}
}
