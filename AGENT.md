# 🤖 AI Agent Guidelines for hotplex (AGENT.md)

Welcome, AI Developer! This document serves as the top-level context and operational boundary for any AI Agent (like Claude Code, OpenCode, or Antigravity) working on the **hotplex** codebase. 

Please read and strictly adhere to the following rules when analyzing, modifying, or creating code in this project.

---

## 🏗 1. Project Overview & Identity

**hotplex** is a high-performance **AI Agent Control Plane**.
- **First Principle**: Instead of reinventing the wheel, we leverage existing, powerful AI CLI agents (like Claude Code, OpenCode) and bridge them into production-ready systems by converting them into long-lived, interactive services (**Cli-as-a-Service**).
- **Core Role**: It provides a production-ready execution environment for AI agents, eliminating the continuous spin-up overhead of headless CLI mode and providing a unified control layer for security, state, and streaming.
- **Primary Language**: Go (Golang) 1.24
- **Architecture**: A lightweight Gateway (WebSocket) wrapping a Core Engine (`hotplex.Engine`), a persistence layer (`internal/engine/pool.go`), and a strict Regex WAF (`internal/security/detector.go`).

### 📍 Repository

- **GitHub**: https://github.com/hrygo/hotplex
- **Owner**: `hrygo`
- **Repo**: `hotplex`

---

## 📜 2. Strict Coding Rules (SOLID & DRY)

When writing or refactoring Go code in HotPlex, you must enforce the following:

1.  **Single Responsibility Principle (SRP)**:
    - Never dump mixed responsibilities into one struct or file.
    - Example: `runner.go` should only bridge I/O and OS processes. Session lifecycle belongs in `internal/engine/pool.go`. Danger detection belongs in `internal/security/detector.go`.
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

1.  **Do Not Bypass `Detector`**: Never write code that allows user prompts or AI commands to reach `Stdin` without first passing through `CheckInput()` in `internal/security/detector.go`.
2.  **Native Capability Governance**: Prioritize native tool restrictions (`AllowedTools` in `EngineOptions`) over file path interception. This leverages the CLI's internal sandbox for more reliable enforcement.
3.  **Filesystem Isolation**: The agent's `WorkDir` is holy. Ensure the CLI is initialized with the correct working directory to leverage its native path restrictions.
4.  **No Eval/Shell Hacks**: Do not use `sh -c` or `bash -c` unless strictly necessary and sanitized. Stick to direct binary execution via `os/exec` where possible.

---

## 📁 4. Architecture Map

When looking for where to make changes, follow this map:

- **Public SDK (`/`)**:
  - `hotplex.go`: Main entry point with public aliases.
  - `client.go`: Client interface definitions.
- **Engine Layer (`engine/`)**:
  - `runner.go`: The `Engine` singleton. High-level API orchestration.
- **Provider Layer (`provider/`)**:
  - `provider.go`: `Provider` interface.
  - `claude_provider.go` / `opencode_provider.go`: Individual tool adapters.
  - `factory.go` / `registry.go`: Provider instantiation and caching.
- **Internal Core (`internal/engine/`)**:
  - `pool.go`: `SessionPool` (process multiplexing) and GC loops.
  - `session.go`: Individual `Session` I/O piping and state machine.
- **Internal Security (`internal/security/`)**:
  - `detector.go`: The Regex WAF. (Add new scary commands to reject here).
- **Internal Systems (`internal/sys/`)**:
  - `proc_unix.go` / `proc_windows.go`: PGID and signal handling.
- **Adapters (`internal/server/`)**:
  - `hotplex_ws.go`: Native WebSocket protocol implementation.
  - `opencode_http.go`: OpenCode HTTP/SSE compatibility layer.
  - `security.go`: CORS and API key security configuration.
- **Types & Events (`types/`, `event/`)**:
  - Core data structures and streaming protocols.

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
5.  **GitHub Operations**: Prioritize using the `gh` command or MCP tools for any GitHub-related actions (Releases, PRs, Runs, Issues).
