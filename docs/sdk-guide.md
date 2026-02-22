# hotplex SDK Developer Guide (Go)

*Read this in other languages: [English](sdk-guide.md), [简体中文](sdk-guide_zh.md).*

Welcome to the hotplex SDK! This guide is designed to help developers integrate the powerful hotplex AI Agent Runtime into their Go applications.

---

## 1. Core Philosophy

hotplex follows the **"Leverage vs Build"** philosophy. Instead of reinventing AI agents, our SDK transforms elite terminal-based AI tools (like Claude Code, OpenCode) into production-ready backend services:
- **Hot-Multiplexing**: Eliminates cold-start latency, achieving millisecond response times.
- **Security Hardening**: Provides Process Group (PGID) isolation and instruction-level WAF auditing.
- **Protocol Normalization**: Standardizes diverse CLI outputs into unified streaming events.

---

## 2. Quick Start

### 2.1 Installation

```bash
go get github.com/hrygo/hotplex
```

### 2.2 Basic Usage Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/hrygo/hotplex"
)

func main() {
    // 1. Initialize Engine
    engine, _ := hotplex.NewEngine(hotplex.EngineOptions{
        Namespace: "my_app",
        Timeout:   5 * time.Minute,
    })
    defer engine.Close()

    // 2. Configure Execution
    cfg := &hotplex.Config{
        WorkDir:   "/tmp/project",
        SessionID: "user_session_123",
    }

    // 3. Define Event Callback
    callback := func(eventType string, data any) error {
        if eventType == "answer" {
            if evt, ok := data.(*hotplex.EventWithMeta); ok {
                fmt.Print(evt.EventData)
            }
        }
        return nil
    }

    // 4. Execute Prompt
    ctx := context.Background()
    engine.Execute(ctx, cfg, "Write a quicksort algorithm in Go", callback)
}
```

---

## 3. Core API Reference

### 3.1 `EngineOptions` (Engine Initialization)
Used in `hotplex.NewEngine(opts)` to define global behavior boundaries.

| Field              | Type            | Description                                                                                                           |
| :----------------- | :-------------- | :-------------------------------------------------------------------------------------------------------------------- |
| `Namespace`        | `string`        | **Namespace**. Used to generate deterministic UUID v5 SessionIDs for physical isolation in multi-tenant environments. |
| `Timeout`          | `time.Duration` | **Execution Timeout**. Maximum allowed time for a single `Execute` call (default: 5m).                                |
| `IdleTimeout`      | `time.Duration` | **Idle Reclaim Time**. Background processes inactive for this duration are automatically cleaned up (default: 30m).   |
| `BaseSystemPrompt` | `string`        | **Engine-level System Prompt**. Injected at process startup as foundational rules for all sessions.                   |
| `PermissionMode`   | `string`        | **Permission Mode**. e.g., `"bypass-permissions"` (auto-authorize) or `"default"`.                                    |
| `AllowedTools`     | `[]string`      | **Tool Whitelist**. Explicit list of allowed tools (e.g., `["Bash", "Edit"]`).                                        |
| `DisallowedTools`  | `[]string`      | **Tool Blacklist**. Explicit list of forbidden tools.                                                                 |
| `AdminToken`       | `string`        | **Admin Token**. Credentials required for privileged security bypass or policy adjustments.                           |
| `Logger`           | `*slog.Logger`  | **Structured Logger**. Injected instance to maintain observability consistency.                                       |
| `Provider`         | `Provider`      | **Driver**. Optional interface to specify the underlying agent implementation (defaults to Claude Code).              |

### 3.2 `Config` (Per-Task Config)
Used in `engine.Execute(ctx, cfg, prompt, cb)` to define the context for a specific task.

| Field              | Type     | Description                                                                                                     |
| :----------------- | :------- | :-------------------------------------------------------------------------------------------------------------- |
| `WorkDir`          | `string` | **Working Directory**. Root directory where the agent performs file operations, searches, and script execution. |
| `SessionID`        | `string` | **Session ID**. Business-level ID that HotPlex maps to a unique background hot process.                         |
| `TaskInstructions` | `string` | **Task Instructions**. Persistent instructions defining the session objective.                                  |

### 3.3 Event Callbacks & Data Models (`Callback`)
Defined as `func(eventType string, data any) error`.

#### Event Types (`eventType`)
- `thinking`: Agent is performing logical reasoning.
- `tool_use`: Starting a local tool invocation (e.g., `bash`, `editor_write`).
- `tool_result`: Tool execution finished with results.
- `answer`: Textual response chunks generated by the agent.
- `session_stats`: Final session statistics (triggered once upon successful completion).
- `danger_block`: Security firewall interception alert.
- `runner_exit`: Underlying process exited unexpectedly.

#### Detailed Metadata (`EventMeta`)
For most events (except `session_stats`), `data` is `*hotplex.EventWithMeta`. Its `Meta` contains:
- `DurationMs`: Time spent on the current step.
- `TotalDurationMs`: Cumulative time spent since the turn started.
- `ToolName` / `ToolID`: Invoked tool name and unique call ID.
- `Status`: Execution status (`running`, `success`, `error`).
- `InputSummary` / `OutputSummary`: Summary of tool input parameters and a truncated preview of output results.
- `FilePath` / `LineCount`: The affected file path and number of lines involved.
- `Progress`: Progress percentage for long-running tasks.
- `InputTokens` / `OutputTokens`: Token consumption for the current step.

#### Final Statistics (`SessionStatsData`)
For the `session_stats` event, `data` is `*hotplex.SessionStatsData`:
- `InputTokens` / `OutputTokens`: Cumulative tokens for the entire turn.
- `CacheReadTokens` / `CacheWriteTokens`: Tokens hit or written to the prompt cache.
- `TotalDurationMs`: Total milliseconds from request start to finish.
- `ToolCallCount`: Total number of tool invocations.
- `ToolsUsed`: Unique list of tool names invoked.
- `FilesModified`: Number of files actually modified during the turn.
- `TotalCostUSD`: Real-time estimated cost for the turn in USD.
- `IsError`: Boolean indicating if the turn ended in failure.

### 3.4 Administrative & Safety Control (`HotPlexClient`)

`HotPlexClient` provides specialized control through functional sub-interfaces. Since `hotplex.NewEngine` returns a concrete `*Engine` that implements all these interfaces, you can use them directly or via type assertion when receiving it as a generic client.

#### Usage Example

```go
// 1. Basic execution (Executor interface)
client.Execute(ctx, cfg, prompt, cb)

// 2. Advanced Control (SessionController)
if controller, ok := client.(hotplex.SessionController); ok {
    stats := controller.GetSessionStats("user_session_123")
    fmt.Printf("Input Tokens: %d\n", stats.InputTokens)
    
    // Stop a hung session
    controller.StopSession("session_123", "user cancel")
}

// 3. Security Management (SafetyManager)
if safety, ok := client.(hotplex.SafetyManager); ok {
    safety.SetDangerAllowPaths([]string{"/home/user/project"})
    safety.SetDangerBypassEnabled("my-admin-token", true)
}
```

#### `SessionController` (Lifecycle & Observability)
| Method                              | Description                                                                 |
| :---------------------------------- | :-------------------------------------------------------------------------- |
| `GetSessionStats(id) *SessionStats` | Returns the latest telemetry/tokens for the specified session.              |
| `StopSession(id, reason) error`     | Forcibly terminates a specific session (useful for Web UIs "Stop" buttons). |
| `GetCLIVersion() (string, error)`   | Returns the version of the underlying agent binary.                         |

#### `SafetyManager` (Security Policy)
| Method                                      | Description                                              |
| :------------------------------------------ | :------------------------------------------------------- |
| `SetDangerAllowPaths([]string)`             | Dynamic whitelist for file operations.                   |
| `SetDangerBypassEnabled(token, bool) error` | Privileged override for the WAF (requires `AdminToken`). |

#### `Executor` (Logic Validation)
| Method                          | Description                                                 |
| :------------------------------ | :---------------------------------------------------------- |
| `ValidateConfig(*Config) error` | Pre-flight security and integrity check for session config. |

---

## 4. Error Handling

HotPlex exports the following core error variables for business logic handling:

- `hotplex.ErrDangerBlocked`: The user's prompt or the agent's action was intercepted by the security WAF.
- `hotplex.ErrInvalidConfig`: Validation failed for the provided `Config` or `EngineOptions` (e.g., non-existent Path).

---

## 5. Security & Isolation

HotPlex provides out-of-the-box security:
1. **Instruction WAF**: Automatically blocks high-risk commands (e.g., `rm -rf /`).
2. **Process Group Isolation**: Ensures any child processes spawned by the agent are properly cleaned up.
3. **Capability Constraints**: Restricts tool usage at the semantic level using `AllowedTools`.

---

## 6. Advanced Features

### 5.1 Multi-Provider Support
HotPlex supports various underlying agents. You can register providers dynamically:
- **Claude Code**: Industry-leading code editing and execution (Default). See the [Claude Code Integration Guide](providers/claudecode.md) for details.
- **OpenCode**: Flexible support for open-source agents. See the [OpenCode Integration Guide](providers/opencode.md) for details.

### 5.2 Observability & Telemetry
At the end of each session, the `session_stats` event returns `SessionStatsData`:
- `TotalDurationMs`: Execution time.
- `InputTokens` / `OutputTokens`: Consumption metrics.
- `TotalCostUSD`: Real-time cost estimation.
- `FilesModified`: Count of modified files.

---

## 7. Best Practices

1. **Lifecycle Management**: Always call `engine.Close()` on application exit to prevent zombie processes.
2. **Cancellation**: Use `context.Context` with timeouts for `Execute` to prevent hanging sessions.
3. **Concurrency**: Callbacks are triggered from stream goroutines; ensure thread safety if accessing shared resources.
4. **Namespace Isolation**: Use unique `Namespace` values in multi-tenant environments to ensure isolation.

---

*For more details, check the full examples in the `_examples/` directory.*
