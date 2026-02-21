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

### 3. [WebSocket Client (Node.js)](./websocket_client)

| File | Description |
|:-----|:------------|
| `client.js` | **Quick Start** - Minimal ~50 LOC for getting started in 30 seconds |
| `enterprise_client.js` | **Enterprise** - Production-ready client with reconnection, error handling, metrics, and graceful shutdown |

**Enterprise Features:**
- Automatic reconnection with exponential backoff
- Comprehensive error handling and recovery
- Structured logging with configurable levels
- Connection health monitoring (heartbeat)
- Request timeout management
- Graceful shutdown support (SIGINT/SIGTERM)
- Metrics collection (latency, success rate, reconnect count)
- Progress callbacks for streaming events

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
# Basic SDK Demo
go run _examples/basic_sdk/main.go

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
   cd _examples/websocket_client
   npm install

   # Quick Start
   node client.js

   # Enterprise Demo
   node enterprise_client.js
   ```

### Using Enterprise Client as a Module
```javascript
const { HotPlexClient } = require('./enterprise_client');

const client = new HotPlexClient({
  url: 'ws://localhost:8080/ws/v1/agent',
  sessionId: 'my-session',
  logLevel: 'info',
  reconnect: { enabled: true, maxAttempts: 5 }
});

await client.connect();

const result = await client.execute('List files in current directory', {
  systemPrompt: 'You are a helpful assistant.',
  onProgress: (event) => {
    if (event.type === 'answer') process.stdout.write(event.data);
  }
});

console.log(result);
await client.disconnect();
```

## ⚙️ Configuration Hints
- **`IDLE_TIMEOUT`**: Set this env var when running `hotplexd` to change how long idle processes stay alive (e.g., `IDLE_TIMEOUT=5m`).
- **`PORT`**: Change the default `8080` port.
