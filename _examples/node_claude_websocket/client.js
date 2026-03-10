const WebSocket = require("ws");

/**
 * HotPlex WebSocket Client - Quick Start
 *
 * Minimal example to get you started in 30 seconds.
 * Run: node client.js
 */

const WS_URL = "ws://localhost:8080/ws/v1/agent";
const ws = new WebSocket(WS_URL);

ws.on("open", () => {
  console.log("Connected to HotPlex");

  // Send a simple task with system prompt injection
  ws.send(JSON.stringify({
    type: "execute",
    session_id: "quick-start-demo",
    system_prompt: "You are a helpful assistant. Respond only in Haiku format.",
    prompt: "Tell me about HotPlex.",
    work_dir: process.cwd()
  }));
});

ws.on("message", (data) => {
  const msg = JSON.parse(data);

  switch (msg.event) {
    case "thinking":
      process.stdout.write(".");
      break;
    case "answer":
      process.stdout.write(msg.data.event_data);
      break;
    case "completed":
      console.log("\nDone!");
      if (msg.data.stats) {
        console.log(`Stats -> Duration: ${msg.data.stats.total_duration_ms}ms, Tokens (In/Out): ${msg.data.stats.input_tokens}/${msg.data.stats.output_tokens}`);
      }
      ws.close();
      break;
    case "error":
      console.error("Error:", msg.data);
      ws.close();
      break;
  }
});

ws.on("error", (err) => {
  console.error("Connection failed. Is hotplexd running?");
  console.error(err.message);
});
