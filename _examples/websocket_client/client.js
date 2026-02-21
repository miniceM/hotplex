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

  // Send a simple task
  ws.send(JSON.stringify({
    type: "execute",
    session_id: "quick-start-demo",
    prompt: "Say 'Hello from HotPlex!' and nothing else.",
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
