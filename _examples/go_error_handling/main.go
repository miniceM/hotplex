package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hrygo/hotplex"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	opts := hotplex.EngineOptions{
		Timeout:        30 * time.Second,
		Logger:         logger,
		Namespace:      "error-handling-demo",
		PermissionMode: "bypass-permissions",
	}

	engine, err := hotplex.NewEngine(opts)
	if err != nil {
		fmt.Printf("❌ Engine initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	cfg := &hotplex.Config{
		WorkDir:   "/tmp/hotplex-demo",
		SessionID: "error-demo",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = engine.Execute(ctx, cfg, "echo hello", func(eventType string, data any) error {
		switch eventType {
		case "answer":
			if evt, ok := data.(*hotplex.EventWithMeta); ok {
				fmt.Print(evt.EventData)
			}
		}
		return nil
	})

	if err != nil {
		handleError(err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Execution completed successfully")
}

func handleError(err error) {
	switch {
	case errors.Is(err, hotplex.ErrDangerBlocked):
		fmt.Printf("🚨 Security: Operation blocked by WAF\n")
		fmt.Printf("   The command was deemed dangerous and not executed.\n")

	case errors.Is(err, hotplex.ErrInvalidConfig):
		fmt.Printf("⚠️ Configuration Error: %v\n", err)
		fmt.Printf("   Check WorkDir and SessionID are valid.\n")

	case errors.Is(err, hotplex.ErrSessionNotFound):
		fmt.Printf("🔍 Session Error: Session not found\n")
		fmt.Printf("   The requested session does not exist.\n")

	case errors.Is(err, hotplex.ErrSessionDead):
		fmt.Printf("💀 Session Error: Session is dead\n")
		fmt.Printf("   The session process has terminated unexpectedly.\n")

	case errors.Is(err, hotplex.ErrTimeout):
		fmt.Printf("⏱️ Timeout: Operation exceeded time limit\n")
		fmt.Printf("   Consider increasing the timeout value.\n")

	case errors.Is(err, hotplex.ErrInputTooLarge):
		fmt.Printf("📦 Input Error: Input too large\n")
		fmt.Printf("   Reduce the size of your prompt.\n")

	case errors.Is(err, hotplex.ErrProcessStart):
		fmt.Printf("🚀 Process Error: Failed to start CLI process\n")
		fmt.Printf("   Ensure Claude Code or OpenCode CLI is installed.\n")

	case errors.Is(err, hotplex.ErrPipeClosed):
		fmt.Printf("🔌 Pipe Error: Communication pipe closed\n")
		fmt.Printf("   The session may have crashed.\n")

	case errors.Is(err, context.DeadlineExceeded):
		fmt.Printf("⏰ Context Timeout: Request cancelled\n")
		fmt.Printf("   The context deadline was exceeded.\n")

	case errors.Is(err, context.Canceled):
		fmt.Printf("🛑 Context Cancelled: Request cancelled by user\n")

	default:
		fmt.Printf("❓ Unknown Error: %v\n", err)
	}
}
