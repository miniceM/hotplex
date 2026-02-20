# 🤖 AI Agent Guidelines for HotPlex (AGENT.md)

Welcome, AI Developer! This document serves as the top-level context and operational boundary for any AI Agent (like Claude Code, OpenCode, Aider, or Antigravity) working on the **HotPlex** codebase. 

Please read and strictly adhere to the following rules when analyzing, modifying, or creating code in this project.

---

## 🏗 1. Project Overview & Identity

**HotPlex** is a high-performance **Process Multiplexer**.
- **Core Role**: It solves the "cold start" problem for heavyweight Node.js CLI agents (specifically Claude Code initially) by keeping a persistent OS process alive in the background and multiplexing user requests via `Stdin/Stdout`.
- **Primary Language**: Go (Golang) 1.21+
- **Architecture**: A lightweight Gateway (WebSocket) wrapping a Core Engine (`hotplex.Engine`), a persistence layer (`SessionPool`), and a strict Regex WAF (`Detector`).

---

## 📜 2. Strict Coding Rules (SOLID & DRY)

When writing or refactoring Go code in HotPlex, you must enforce the following:

1.  **Single Responsibility Principle (SRP)**:
    - Never dump mixed responsibilities into one struct or file.
    - Example: `runner.go` should only bridge I/O and OS processes. Session lifecycle belongs in `session_manager.go`. Danger detection belongs in `danger.go`.
2.  **Concurrency Safety First**:
    - **Never** read/write to `SessionPool` maps without holding the appropriate `sync.RWMutex`.
    - Always use `defer mu.Unlock()` immediately after acquiring a lock.
    - Be hyper-aware of Deadlocks. Do not trigger callbacks that might re-enter a locked section.
3.  **Process Lifecycle & Zombie Prevention**:
    - Any OS Process created *must* be assigned a Process Group ID (PGID) via `SysProcAttr{Setpgid: true}`.
    - We kill processes by sending `SIGKILL` to `-PGID`, never just the PID, to ensure no orphan Node.js/Python processes leak.
4.  **Error Handling**:
    - Do not use `panic()` in the core engine. Return `error` explicitly.
    - Wrap errors with context: `fmt.Errorf("failed to start session %s: %w", sessionID, err)`.
5.  **Logging**:
    - Use `log/slog`.
    - Always include structured context: `logger.Info("session started", "session_id", sessionID)`.

---

## 🔒 3. Security Boundaries (Zero-Trust)

HotPlex executes LLM-generated Shell commands on the host machine. **Security is the top priority.**

1.  **Do Not Bypass `Detector`**: Never write code that allows user prompts or AI commands to reach `Stdin` without first passing through `CheckInput()` in `danger.go`.
2.  **Native Capability Governance**: As of v0.2.0, prioritize native tool restrictions (`AllowedTools` in `EngineOptions`) over file path interception. This leverages the CLI's internal sandbox for more reliable enforcement.
3.  **Filesystem Isolation**: The agent's `WorkDir` is holy. Ensure the CLI is initialized with the correct working directory to leverage its native path restrictions.
4.  **No Eval/Shell Hacks**: Do not use `sh -c` or `bash -c` unless strictly necessary and sanitized. Stick to direct binary execution via `os/exec` where possible.

---

## 📁 4. Architecture Map

When looking for where to make changes, follow this map:

- `pkg/hotplex/`: **The Core SDK.** (Do not put HTTP/CLI logic here).
  - `runner.go`: The `Engine` singleton. High-level API.
  - `session_manager.go`: `SessionPool` and GC loops. State management.
  - `danger.go`: The Regex WAF. (Add new scary commands to reject here).
  - `events.go`: Data structures for JSON STDOUT parsing.
- `cmd/hotplexd/`: **The Executable.**
  - `main.go`: Application entrypoint. Wires dependency injection.
- `internal/server/`: **The Adapters.**
  - `websocket.go`: HTTP to WebSocket upgrade and JSON RPC framing over WS.
- `docs/`: **Knowledge Base.**
  - `architecture.md`: `@docs/architecture.md` (Design & Diagrams)

---

## 🛠 5. Testing Requirements

- **No Code Without Tests**: If you add a feature, you must add a unit test in the corresponding `_test.go` file.
- **Mock Heavy I/O**: Do not write tests that actually spawn `npx @anthropic-ai/claude-code` unless it is explicitly an E2E test. Use echo/cat dummy shell scripts to mock the CLI in unit tests.
- **Race Detector**: All Go code must pass `go test -race ./...`.

---

## 🚀 6. Action Mode Trigger (For AI)

If the USER asks you to `[Implement]`, `[Extend]`, or `[Fix]` something in HotPlex:
1.  **Acknowledge**: Briefly state the plan.
2.  **Verify**: Check this `AGENT.md` for architectural constraints.
3.  **Execute**: Write the code, ensuring concurrency locks and PGID rules are respected.
4.  **Validate**: Ensure it builds (`go build ./...`).
