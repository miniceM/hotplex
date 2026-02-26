# hotplex Core Architecture Documentation

*Read this in other languages: [English](architecture.md), [简体中文](architecture_zh.md).*

hotplex is a high-performance **Agent Runtime** for AI CLI Agents, designed to transform one-off terminal-based AI tools (like Claude Code or OpenCode) into production-ready, long-lived interactive services. Its core philosophy is "Leverage vs Build"—by maintaining a persistent process pool with hardened security boundaries and a normalized full-duplex protocol layer, hotplex eliminates the spin-up overhead of headless CLI environments and enables millisecond-level responsiveness.

---

## 1. Physical Layout & Clean Architecture

hotplex follows a layered architecture with strict visibility rules, separating the public SDK from internal execution details and protocol adapters.

### 1.1 Directory Structure (Actual)
- **Root (`/`)**: Main entry point for the SDK. Contains `hotplex.go` (public aliases) and `client.go`.
- **`engine/`**: The public execution runner (`Engine`). Orchestrates prompt execution, security WAF, and event dispatching.
- **`provider/`**: The abstraction layer for diverse AI CLI agents. Contains the `Provider` interface and concrete implementations for `claude-code` and `opencode`.
- **`types/`**: Fundamental data structures (`Config`, `StreamMessage`, `UsageStats`).
- **`event/`**: Unified event protocol and callback definitions (`Callback`, `EventWithMeta`).
- **`internal/engine/`**: The core execution engine. Manages the `SessionPool` (process multiplexing) and `Session` (I/O piping and state management).
- **`internal/persistence/`**: Session durability markers and pool recovery logic.
- **`internal/security/`**: The Regex-based WAF (`Detector`) for command auditing.
- **`internal/config/`**: Hot-reloadable configuration watchers.
- **`internal/sys/`**: Low-level OS primitives for cross-platform process group management (PGID) and signal handling.
- **`internal/server/`**: Protocol adapters. Contains `hotplex_ws.go` (WebSocket) and `opencode_http.go` (REST/SSE).
- **`internal/strutil/`**: High-performance string manipulation and path cleaning.

### 1.2 Design Principles
1.  **Public Thin, Private Thick**: The root package `hotplex` provides a stable, minimal API surface.
2.  **Strategy Pattern (Provider)**: Decouples the engine from specific AI tools. `provider.Provider` allows switching backends without changing execution logic.
3.  **IO-Driven State Machine**: `internal/engine` manages process states (Starting, Ready, Busy, Dead) using IO markers rather than fixed sleeps.

---

## 2. Core System Components

### 2.1 Engine & Runner (`engine/runner.go`)
*   **Engine Singleton**: The primary interface for users (`NewEngine`, `Execute`).
*   **Security Injection**: Dynamically injects global `EngineOptions` (like `AllowedTools`) into downstream sessions.
*   **Deterministic Session ID**: Uses UUID v5 to map conversation IDs to persistent sessions, ensuring high context cache hits.

### 2.2 Provider Adapter (`provider/`)
Standardizes diverse CLI protocols into a unified "HotPlex Event Stream":
*   **Provider Interface**: Handles CLI argument construction, input payload formatting, and event parsing.
*   **Factory & Registry**: `ProviderFactory` manages provider instantiation, while `ProviderRegistry` caches active instances for reuse.

### 2.3 Session Manager (`internal/engine/pool.go`)
*   **Hot-Multiplexing**: The `SessionPool` maintains a registry of active processes. Repeat requests to the same session skip the "Cold Start" (fork) and perform a "Hot Execution" (stdin injection).
*   **Graceful GC**: Uses a `cleanupLoop` to sweep idle processes based on `IdleTimeout`.

### 2.4 Security & Process Isolation (`internal/security/`, `internal/sys/`)
*   **Regex WAF**: The `Detector` scans all input prompts for malicious intent (e.g., `rm -rf /`) as a final line of defense before reached the agent.
*   **PGID Hard Isolation**: Ensures that the agent and any of its child processes (e.g., a build script) are assigned a unique Process Group ID. Termination kills the entire group via `SIGKILL` to prevent orphan processes.

### 2.5 Event Hooks & Observability (`hooks/`, `telemetry/`)
*   **Webhooks & Audit**: Asynchronously broadcasts payload events to external systems (Slack, Webhooks) without blocking the hot-execution path.
*   **Tracing & Metrics**: Pushes native OpenTelemetry spans and exposes `/metrics` for Prometheus scraping.

---

## 3. Session Lifecycle & Data Flow

```mermaid
sequenceDiagram
    participant Client as "Client (WebSocket/SDK)"
    participant Server as "internal/server"
    participant Engine as "engine.Engine"
    participant Hooks as "Event Hooks & OTel"
    participant Pool as "internal/engine.SessionPool"
    participant Provider as "provider.Provider"
    participant Proc as "CLI Process (OS)"
    
    Client->>Server: Request (WebSocket Message / POST)
    Server->>Engine: Execute(Config, Prompt)
    Engine->>Engine: Check WAF (Detector)
    Engine->>Hooks: Start Trace Span
    Engine->>Pool: GetOrCreateSession(ID)
    
    alt Cold Start (Not in Pool)
        Pool->>Provider: Build CLI Args/Env
        Pool->>Proc: fork() with PGID
        Pool->>Proc: Inject Context/SystemPrompt
    end
    
    Engine->>Proc: Write stdin (JSON payload)
    
    loop Stream Event Normalization
        Proc-->>Provider: Raw tool specific output
        Provider-->>Engine: Normalized ProviderEvent
        Engine-->>Hooks: Emit Event (Webhook/Log)
        Engine-->>Server: Public EventWithMeta
        Server-->>Client: WebSocket/SSE Event
    end
    
    Engine->>Pool: Touch(ID) to refresh idle timer
    Engine->>Hooks: End Trace & Record Metrics
```

---

## 4. Feature Matrix

### Core Capabilities
- [x] Clean Architecture with `internal/` isolation
- [x] Strategy-based Provider pattern (Claude Code, OpenCode)
- [x] Resilient Session Hot-Multiplexing
- [x] Multi-platform PGID management (Windows Job Objects / Unix PGID)
- [x] Regex-based Security WAF
- [x] **Dual Protocol Gateway**: Native WebSocket and OpenCode-compatible REST/SSE API
- [x] **Event Hooks**: Plugin system for Webhooks, Slack, and custom audit sinks
- [x] **Observability**: OpenTelemetry native tracing and Prometheus metrics (`/metrics`)

### Planned Enhancements
- **L2/L3 Isolation**: Integrating Linux Namespaces (PID/Net) and WASM sandboxing
- **Multi-Agent Bus**: Orchestrating multiple specialized agents behind a single namespace
