---
layout: home

hero:
  name: "HotPlex"
  text: "The AI Agent Control Plane"
  tagline: "Turn your AI CLI Agents into high-performance, production-ready interactive services."
  image:
    src: /logo.svg
    alt: HotPlex
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/hrygo/hotplex

features:
  - icon: 🔄
    title: Cli-as-a-Service
    details: Upgrade "run-and-exit" CLI tools into persistent, stateful interactive services with full context preservation.
  
  - icon: 🚀
    title: Zero Startup Latency
    details: Hot-multiplexing eliminates Node.js/Python spin-up costs, delivering sub-second AI response times.
  
  - icon: 🛡️
    title: Enterprise Security
    details: Multi-layer protection with PGID isolation, command-level WAF, and workspace boundary locking.
  
  - icon: 💬
    title: ChatApps Native
    details: Built-in adapters for Telegram, Slack, and DingTalk, bringing Agent capabilities to your favorite communication tools.
  
  - icon: 🔌
    title: Unified Integration
    details: Seamlessly integrate via native Go SDK, WebSocket protocol, or OpenCode-compatible HTTP/SSE endpoints.
  
  - icon: 📊
    title: Full Observability
    details: Production-ready with OpenTelemetry tracing, Prometheus metrics, and automated health monitoring.
---

## ⚡ Why HotPlex?

HotPlex is the **Strategic Bridge** for AI agent engineering. It decouples the access layer from the execution engine, allowing you to build professional AI products without worry about the overhead of managing long-lived agent processes.

### sub-second Interaction
Forget the 3-5 second wait for agent runtimes. HotPlex maintains a "hot" pool of sessions, making your AI interactions feel like a native chat experience.

### Secure by Design
Every agent runs in a sandbox. Our **Danger WAF** intercepts destructive shell commands, while **PGID isolation** ensures no orphan processes are left behind on your server.

---

## 🚀 Quick Start

### Install

```bash
go get github.com/hrygo/hotplex
```

### Basic Usage (Go SDK)

```go
package main

import (
    "context"
    "fmt"
    "github.com/hrygo/hotplex"
)

func main() {
    // 1. Initialize the Engine
    opts := hotplex.EngineOptions{
        PermissionMode:  "bypassPermissions",
        AllowedTools:    []string{"Bash", "Edit", "Read"},
    }
    engine, err := hotplex.NewEngine(opts)
    if err != nil {
        fmt.Printf("Failed to initialize engine: %v\n", err)
        return
    }
    defer engine.Close()

    // 2. Configure a persistent session
    cfg := &hotplex.Config{
        WorkDir:   "/tmp/ai-sandbox",
        SessionID: "user-unique-session",
    }

    // 3. Execute with real-time streaming
    err = engine.Execute(context.Background(), cfg, "List files in current directory", 
        func(eventType string, data any) error {
            if eventType == "answer" {
                fmt.Printf("🤖: %v\n", data)
            }
            return nil
        })
    if err != nil {
        fmt.Printf("Execution failed: %v\n", err)
    }
}
```

---

## 🌐 Connectivity & Ecosystem

HotPlex is designed to live at the center of your AI infrastructure.

| Access Layer   | Role          | Target Use Case                               |
| -------------- | ------------- | --------------------------------------------- |
| **Go SDK**     | Native Logic  | High-performance backend orchestration.       |
| **ChatApps**   | UI Gateway    | Integration with Slack, Feishu, and DingTalk. |
| **Proxy Mode** | Real-time I/O | Full-duplex WebSocket UIs for AI agents.      |
| **Meta Layer** | Compatibility | OpenCode HTTP/SSE standard integration.       |

---

## 🗺️ Roadmap

### Completed
- [x] **v0.9.0**: Multi-language SDKs (Python, TS) & Docker Execution.
- [x] **v0.10.0**: ChatApps Platform Integration (Telegram/DingTalk).
- [x] **v0.11.0**: Documentation Site & Observability Stack.

### Planned (H2 2026)
- [ ] **L2/L3 Isolation**: Kernel-level PID/Net namespaces.
- [ ] **WASM Runtime**: Fully isolated tool execution via WebAssembly.
---

<p align="center">
  <i>Built with ❤️ for the AI Engineering community.</i><br/>
  <i>Copyright © 2026 HotPlex Team</i>
</p>
