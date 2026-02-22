//go:build benchmark
// +build benchmark

package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	intengine "github.com/hrygo/hotplex/internal/engine"
	"github.com/hrygo/hotplex/internal/security"
	"github.com/hrygo/hotplex/provider"
	"github.com/hrygo/hotplex/types"
)

// =============================================================================
// HotPlex Performance Benchmarks
// =============================================================================
// Run with: go test -tags=benchmark -bench=. -benchmem ./engine/
// =============================================================================

// setupBenchmarkEngine creates a test engine with a mock CLI provider
func setupBenchmarkEngine(b *testing.B) (*Engine, string, func()) {
	tmpDir, err := os.MkdirTemp("", "hotplex-bench-*")
	if err != nil {
		b.Fatal(err)
	}

	// Create a fast mock CLI that simulates Claude Code behavior
	dummyPath := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
# Signal readiness
echo '{"type":"ready"}'
# Process turns
while read line; do
  echo '{"type":"thinking","thinking":"Processing..."}'
  echo '{"type":"result","result":"Done","status":"success","duration_ms":10,"tokens":{"input":100,"output":50}}'
done
`
	if err := os.WriteFile(dummyPath, []byte(script), 0755); err != nil {
		b.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	opts := EngineOptions{
		Namespace:      "benchmark",
		Logger:         logger,
		IdleTimeout:    30 * time.Minute,
		Timeout:        5 * time.Minute,
		PermissionMode: "bypassPermissions",
	}

	prv, err := provider.NewClaudeCodeProvider(provider.ProviderConfig{}, logger)
	if err != nil {
		b.Fatal(err)
	}

	eng := &Engine{
		opts:           opts,
		cliPath:        dummyPath,
		logger:         logger,
		provider:       prv,
		manager:        intengine.NewSessionPool(logger, opts.IdleTimeout, intengine.EngineOptions(opts), dummyPath, prv),
		dangerDetector: security.NewDetector(logger),
	}

	cleanup := func() {
		eng.Close()
		os.RemoveAll(tmpDir)
	}

	return eng, tmpDir, cleanup
}

// -----------------------------------------------------------------------------
// BENCHMARK: Cold Start Latency
// Measures time to create a new session (first request for a session ID)
// -----------------------------------------------------------------------------
func BenchmarkColdStartLatency(b *testing.B) {
	eng, tmpDir, cleanup := setupBenchmarkEngine(b)
	defer cleanup()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sessionID := fmt.Sprintf("cold-start-%d", i)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			err := eng.Execute(ctx, &types.Config{
				SessionID: sessionID,
				WorkDir:   tmpDir,
			}, "test prompt", func(eventType string, data any) error {
				return nil
			})

			cancel()
			if err != nil {
				b.Errorf("Cold start failed: %v", err)
			}
			i++
		}
	})
}

// -----------------------------------------------------------------------------
// BENCHMARK: Hot Multiplex Latency
// Measures time for subsequent requests to an existing session
// -----------------------------------------------------------------------------
func BenchmarkHotMultiplexLatency(b *testing.B) {
	eng, tmpDir, cleanup := setupBenchmarkEngine(b)
	defer cleanup()

	// Pre-warm a session
	sessionID := "hot-multiplex-session"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := eng.Execute(ctx, &types.Config{
		SessionID: sessionID,
		WorkDir:   tmpDir,
	}, "warmup", func(eventType string, data any) error { return nil })
	cancel()
	if err != nil {
		b.Fatalf("Warmup failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := eng.Execute(ctx, &types.Config{
			SessionID: sessionID,
			WorkDir:   tmpDir,
		}, "test prompt", func(eventType string, data any) error {
			return nil
		})
		cancel()
		if err != nil {
			b.Errorf("Hot multiplex failed: %v", err)
		}
	}
}

// -----------------------------------------------------------------------------
// BENCHMARK: Session Pool Throughput
// Measures how many concurrent sessions can be handled
// -----------------------------------------------------------------------------
func BenchmarkSessionPoolThroughput(b *testing.B) {
	eng, tmpDir, cleanup := setupBenchmarkEngine(b)
	defer cleanup()

	b.ResetTimer()

	var wg sync.WaitGroup
	workers := 10
	requestsPerWorker := b.N / workers
	if requestsPerWorker < 1 {
		requestsPerWorker = 1
	}

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("throughput-worker-%d", workerID)

			// First request: cold start
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = eng.Execute(ctx, &types.Config{
				SessionID: sessionID,
				WorkDir:   tmpDir,
			}, "init", func(eventType string, data any) error { return nil })
			cancel()

			// Subsequent requests: hot multiplex
			for i := 0; i < requestsPerWorker-1; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = eng.Execute(ctx, &types.Config{
					SessionID: sessionID,
					WorkDir:   tmpDir,
				}, fmt.Sprintf("request-%d", i), func(eventType string, data any) error { return nil })
				cancel()
			}
		}(w)
	}
	wg.Wait()
}

// -----------------------------------------------------------------------------
// BENCHMARK: Security WAF Performance
// Measures time to check for dangerous commands
// -----------------------------------------------------------------------------
func BenchmarkDangerDetection(b *testing.B) {
	detector := security.NewDetector(slog.Default())

	testInputs := []string{
		"Please read the file and summarize it",
		"Run npm install to add dependencies",
		"Execute the test suite",
		"rm -rf /",    // dangerous
		"sudo reboot", // dangerous
		"Normal coding task description",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := testInputs[i%len(testInputs)]
		_ = detector.CheckInput(input)
	}
}

// -----------------------------------------------------------------------------
// BENCHMARK: Event Callback Overhead
// Measures overhead of event dispatch
// -----------------------------------------------------------------------------
func BenchmarkEventCallbackOverhead(b *testing.B) {
	callback := func(eventType string, data any) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = callback("answer", "test response")
		_ = callback("tool_use", "Bash")
		_ = callback("thinking", "processing...")
	}
}

// -----------------------------------------------------------------------------
// BENCHMARK: Concurrent Session Creation
// Measures parallel cold start performance
// -----------------------------------------------------------------------------
func BenchmarkConcurrentSessionCreation(b *testing.B) {
	eng, tmpDir, cleanup := setupBenchmarkEngine(b)
	defer cleanup()

	b.ResetTimer()

	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("concurrent-%d", idx)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_ = eng.Execute(ctx, &types.Config{
				SessionID: sessionID,
				WorkDir:   tmpDir,
			}, "test", func(eventType string, data any) error { return nil })
		}(i)
	}
	wg.Wait()
}

// -----------------------------------------------------------------------------
// BENCHMARK: Memory Allocation Per Session
// Measures memory overhead of session creation
// -----------------------------------------------------------------------------
func BenchmarkMemoryPerSession(b *testing.B) {
	eng, tmpDir, cleanup := setupBenchmarkEngine(b)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sessionID := fmt.Sprintf("memory-%d", i)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = eng.Execute(ctx, &types.Config{
			SessionID: sessionID,
			WorkDir:   tmpDir,
		}, "test", func(eventType string, data any) error { return nil })
		cancel()
	}
}
