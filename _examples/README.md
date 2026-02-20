# HotPlex Examples

This directory contains examples of how to use the HotPlex SDK and Proxy Server.

## 📁 Examples Structure

### 1. [Basic SDK (Go)](./basic_sdk)
A simple Go application demonstrating the basic usage of the `HotPlexClient` to execute a prompt and handle streaming events.

### 2. [Full SDK (Go)](./full_sdk)
A comprehensive Go demo showing the end-to-end lifecycle of a session:
- **Cold Start**: Initializing a new persistent process.
- **Hot-Multiplexing**: Reusing an existing process for sub-second latency.
- **Process Recovery**: How HotPlex resumes sessions after a "crash" or restart using marker files.
- **Manual Termination**: Explicitly stopping a session.

### 3. [Basic WebSocket (Node.js)](./websocket_client)
A minimal Node.js client using the `ws` library to interact with the HotPlex Proxy Server (`hotplexd`).

### 4. [Full WebSocket (Node.js)](./full_websocket)
An advanced Node.js demo that mirrors the Full SDK features over the WebSocket protocol, including manual session termination via the `stop` command.

---

## 🚀 How to Run

### Prerequisite: Claude Code CLI
Ensure you have the `claude` CLI installed and authenticated. 

#### Recommended (Native):
```bash
# macOS / Linux / WSL
curl -fsSL https://claude.ai/install.sh | bash

# Windows (PowerShell)
irm https://claude.ai/install.ps1 | iex
```

#### Alternatives:
```bash
brew install claude-code
# OR
npm install -g @anthropic-ai/claude-code
```

Run authentication:
```bash
claude auth
```

### Running the Go Examples
```bash
# Full SDK Demo
go run _examples/full_sdk/main.go
```

### Running the WebSocket Examples
1. Start the HotPlex Proxy Server:
   ```bash
   go run cmd/hotplexd/main.go
   ```

2. Run the Node.js client (in another terminal):
   ```bash
   cd _examples/full_websocket
   npm install
   node client.js
   ```

## ⚙️ Configuration Hints
- **`IDLE_TIMEOUT`**: Set this env var when running `hotplexd` to change how long idle processes stay alive (e.g., `IDLE_TIMEOUT=5m`).
- **`PORT`**: Change the default `8080` port.
