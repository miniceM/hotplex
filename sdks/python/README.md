# HotPlex Python SDK

The **HotPlex Python SDK** is a production-ready client that transforms elite AI CLI tools (like Claude Code or OpenCode) into long-lived, interactive services (Cli-as-a-Service).

Instead of dealing with the multi-second spin-up latency of starting CLIs in headless mode, this SDK allows your Python applications to communicate with a persistent HotPlex session via high-performance WebSocket streams.

## 🛠 Prerequisites

This SDK is a client-side library. To use it, you must have a running **HotPlex Server** (`hotplexd`) which acts as the execution bridge for the AI agents.

1.  **Install the HotPlex Server**: Follow the [HotPlex Main Repository](https://github.com/hrygo/hotplex) instructions to build and run `hotplexd`.
2.  **Start the Server**:
    ```bash
    PORT=8080 ./dist/hotplexd
    ```

## 📦 Installation

```bash
pip install hotplex
```

## Quick Start

```python
from hotplex import HotPlexClient, Config

client = HotPlexClient(url="ws://localhost:8080/ws/v1/agent")

config = Config(
    work_dir="/tmp/ai-sandbox",
    session_id="my-session",
    task_instructions="You are a helpful coding assistant."
)

for event in client.execute_stream(
    prompt="Write a Python function to calculate fibonacci",
    config=config,
):
    if event.type == "answer":
        print(event.data, end="")
    elif event.type == "thinking":
        print(f"\n[Thinking: {event.data}]")
    elif event.type == "tool_use":
        print(f"\n[Tool: {event.meta.tool_name}]")

client.close()
```

## Context Manager

```python
from hotplex import HotPlexClient, Config

with HotPlexClient() as client:
    events = client.execute(
        prompt="List files in current directory",
        config=Config(work_dir="/tmp", session_id="test")
    )
    
    for event in events:
        print(f"{event.type}: {event.data}")
```

## Error Handling

```python
from hotplex import (
    HotPlexClient,
    Config,
    DangerBlockedError,
    TimeoutError,
    ExecutionError,
)

client = HotPlexClient()

try:
    events = client.execute(
        prompt="rm -rf /",
        config=Config(work_dir="/tmp", session_id="test")
    )
except DangerBlockedError as e:
    print(f"Blocked by WAF: {e}")
except TimeoutError as e:
    print(f"Request timed out: {e}")
except ExecutionError as e:
    print(f"Execution failed: {e}")
```

## Configuration Options

```python
from hotplex import HotPlexClient, ClientConfig

config = ClientConfig(
    url="ws://localhost:8080/ws/v1/agent",
    timeout=300.0,
    reconnect=True,
    reconnect_attempts=5,
    reconnect_delay=1.0,
    log_level="DEBUG",
    api_key="your-api-key",
)

client = HotPlexClient(config=config)
```

## Event Types

| Event           | Description              |
| --------------- | ------------------------ |
| `thinking`      | Agent is thinking        |
| `answer`        | Streaming text response  |
| `tool_use`      | Tool invocation started  |
| `tool_result`   | Tool execution result    |
| `session_stats` | Final session statistics |
| `error`         | Error occurred           |
| `danger_block`  | Blocked by WAF           |

## 🌐 OpenCode (HTTP/SSE) Support

If you prefer using the OpenCode-compatible HTTP/SSE protocol instead of WebSockets:

```python
from hotplex import OpenCodeClient, Config

client = OpenCodeClient(url="http://localhost:8080")

for event in client.execute_stream(
    prompt="Hello!",
    config=Config(session_id="my-sse-session")
):
    print(f"{event.type}: {event.data}")
```

Note: Requires `sseclient-py` and `requests`.

## License

MIT
