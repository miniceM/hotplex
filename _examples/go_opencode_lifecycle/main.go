package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/hrygo/hotplex"
)

/*
This example demonstrates the full lifecycle management of an OpenCode session with HotPlex.
It covers:
1. Cold Start: Initializing a brand new session.
2. Multi-turn interaction within a single session.
3. Session Persistence: Capturing the provider-specific session ID.
4. Hot-Multiplexing / Warm Start: Resuming a previous session using its ID.
5. Process Recovery: How HotPlex ensures session continuity.
*/

func main() {
	// Configure logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("=== HotPlex OpenCode Lifecycle & Persistence Demo ===")

	// 1. Initialize OpenCode Provider
	opencodePrv, err := hotplex.NewOpenCodeProvider(hotplex.ProviderConfig{
		Type:         hotplex.ProviderTypeOpenCode,
		DefaultModel: "zhipu/glm-5-code-plan", // Updated to GLM 5 Code Plan
		OpenCode: &hotplex.OpenCodeConfig{
			PlanMode: true, // Safe mode
		},
	}, logger)
	if err != nil {
		log.Fatalf("Failed to create OpenCode provider: %v", err)
	}

	// 2. Initialize HotPlex Core Engine
	engine, err := hotplex.NewEngine(hotplex.EngineOptions{
		Namespace: "opencode_lifecycle",
		Provider:  opencodePrv,
	})
	if err != nil {
		log.Fatalf("Failed to initialize HotPlex: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	sessionID := "persistent-opencode-task"
	var lastProviderSessionID string

	// --- PHASE 1: Initial Cold Start ---
	fmt.Println("\n[Phase 1] Initial Cold Start...")
	cfg1 := &hotplex.Config{
		WorkDir:   "./lifecycle_demo",
		SessionID: sessionID,
	}

	err = engine.Execute(ctx, cfg1, "Hi! This is the start of our project. Please create a file named README.md with the text 'Hello OpenCode'.",
		func(eventType string, data any) error {
			if eventType == "answer" {
				fmt.Print(data.(*hotplex.EventWithMeta).EventData)
			}
			return nil
		})
	if err != nil {
		log.Fatalf("Phase 1 failed: %v", err)
	}

	// HotPlex automatically manages session state.
	// In a real application, you might want to save the providerSessionID to a DB mapping.
	// For this demo, we rely on the internal state or the --session flag mapped by HotPlex.

	// --- PHASE 2: Multi-turn Interaction (Continuous) ---
	fmt.Println("\n\n[Phase 2] Multi-turn Interaction (Continuing same session)...")
	err = engine.Execute(ctx, cfg1, "Now add a section called 'Features' to that README.md.",
		func(eventType string, data any) error {
			if eventType == "answer" {
				fmt.Print(data.(*hotplex.EventWithMeta).EventData)
			}
			return nil
		})
	if err != nil {
		log.Fatalf("Phase 2 failed: %v", err)
	}

	// --- PHASE 3: Simulating Engine Restart & Session Recovery ---
	fmt.Println("\n\n[Phase 3] Simulating Recovery (Warm Start)...")
	// In a real scenario, the engine might have been killed.
	// We use the same SessionID to tell HotPlex to resume.

	err = engine.Execute(ctx, cfg1, "Verify the content of README.md and tell me what the total line count is.",
		func(eventType string, data any) error {
			if eventType == "answer" {
				fmt.Print(data.(*hotplex.EventWithMeta).EventData)
			}
			return nil
		})
	if err != nil {
		log.Fatalf("Phase 3 failed: %v", err)
	}

	// --- PHASE 4: Fetching Stats Manually ---
	fmt.Println("\n\n[Phase 4] Fetching Session Stats Manually...")
	// Demonstrates the new v0.8.0 GetSessionStats(sessionID) interface
	stats := engine.GetSessionStats(sessionID)
	if stats != nil {
		fmt.Printf("Manually Fetched Stats -> Duration: %dms, Input Tokens: %d, Output Tokens: %d\n",
			stats.TotalDurationMs, stats.InputTokens, stats.OutputTokens)
	}

	fmt.Printf("\n=== Demo Complete (Session ID: %s) ===\n", sessionID)
}
