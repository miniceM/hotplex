/**
 * HotPlex Enterprise WebSocket Client
 *
 * Production-ready reference implementation with:
 * - Automatic reconnection with exponential backoff
 * - Comprehensive error handling and recovery
 * - Structured logging with configurable levels
 * - Connection health monitoring (heartbeat)
 * - Request timeout management
 * - Graceful shutdown support
 * - Full lifecycle management
 * - Metrics collection
 *
 * @example
 * const client = new HotPlexClient({ url: 'ws://localhost:8080/ws/v1/agent' });
 * await client.connect();
 * const result = await client.execute('List files in current directory');
 * console.log(result);
 * await client.disconnect();
 */

const WebSocket = require("ws");

// ============================================================================
// Configuration
// ============================================================================

const DEFAULT_CONFIG = {
  url: "ws://localhost:8080/ws/v1/agent",
  sessionId: `session-${Date.now()}`,
  workDir: process.cwd(),
  systemPrompt: null,
  timeout: { connect: 5000, request: 120000 },
  reconnect: {
    enabled: true,
    maxAttempts: 5,
    baseDelay: 1000,
    maxDelay: 30000,
  },
  heartbeat: { enabled: true, interval: 30000 },
  logLevel: "info", // debug, info, warn, error
};

// ============================================================================
// Logger
// ============================================================================

class Logger {
  constructor(level = "info") {
    this.levels = { debug: 0, info: 1, warn: 2, error: 3 };
    this.level = this.levels[level] ?? 1;
  }

  _log(level, prefix, ...args) {
    if (this.levels[level] >= this.level) {
      const timestamp = new Date().toISOString();
      console.log(`[${timestamp}] [${prefix}]`, ...args);
    }
  }

  debug(...args) {
    this._log("debug", "DEBUG", ...args);
  }
  info(...args) {
    this._log("info", "INFO", ...args);
  }
  warn(...args) {
    this._log("warn", "WARN", ...args);
  }
  error(...args) {
    this._log("error", "ERROR", ...args);
  }
}

// ============================================================================
// HotPlex Client
// ============================================================================

class HotPlexClient {
  constructor(options = {}) {
    this.config = { ...DEFAULT_CONFIG, ...options };
    this.logger = new Logger(this.config.logLevel);
    this.ws = null;
    this.connected = false;
    this.reconnectAttempts = 0;
    this.pendingRequests = new Map();
    this.requestId = 0;
    this.metrics = {
      requestsTotal: 0,
      requestsSuccess: 0,
      requestsFailed: 0,
      totalLatencyMs: 0,
      reconnectCount: 0,
    };
    this._heartbeatTimer = null;
    this._shutdown = false;
  }

  // --------------------------------------------------------------------------
  // Connection Management
  // --------------------------------------------------------------------------

  async connect() {
    if (this.connected) return;

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error("Connection timeout"));
      }, this.config.timeout.connect);

      this.logger.info("Connecting to", this.config.url);

      this.ws = new WebSocket(this.config.url);

      this.ws.on("open", () => {
        clearTimeout(timeout);
        this.connected = true;
        this.reconnectAttempts = 0;
        this.logger.info("Connected successfully");
        this._startHeartbeat();
        resolve();
      });

      this.ws.on("message", (data) => this._handleMessage(data));
      this.ws.on("error", (err) => this._handleError(err));
      this.ws.on("close", (code, reason) => this._handleClose(code, reason));
    });
  }

  async disconnect() {
    this._shutdown = true;
    this._stopHeartbeat();

    if (this.ws && this.connected) {
      this.logger.info("Disconnecting...");
      this.ws.close();
      this.connected = false;
    }

    // Reject all pending requests
    for (const [id, { reject }] of this.pendingRequests) {
      reject(new Error("Connection closed"));
      this.pendingRequests.delete(id);
    }
  }

  async _reconnect() {
    if (this._shutdown || !this.config.reconnect.enabled) return;

    const { maxAttempts, baseDelay, maxDelay } = this.config.reconnect;

    if (this.reconnectAttempts >= maxAttempts) {
      this.logger.error("Max reconnect attempts reached");
      return;
    }

    this.reconnectAttempts++;
    this.metrics.reconnectCount++;

    const delay = Math.min(
      baseDelay * Math.pow(2, this.reconnectAttempts - 1),
      maxDelay,
    );
    this.logger.warn(
      `Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${maxAttempts})`,
    );

    await new Promise((r) => setTimeout(r, delay));

    try {
      await this.connect();
    } catch (err) {
      this.logger.error("Reconnect failed:", err.message);
    }
  }

  // --------------------------------------------------------------------------
  // Heartbeat
  // --------------------------------------------------------------------------

  _startHeartbeat() {
    if (!this.config.heartbeat.enabled) return;

    this._heartbeatTimer = setInterval(() => {
      if (this.connected && this.ws.readyState === WebSocket.OPEN) {
        this.ws.ping();
      }
    }, this.config.heartbeat.interval);
  }

  _stopHeartbeat() {
    if (this._heartbeatTimer) {
      clearInterval(this._heartbeatTimer);
      this._heartbeatTimer = null;
    }
  }

  // --------------------------------------------------------------------------
  // Event Handlers
  // --------------------------------------------------------------------------

  _handleMessage(data) {
    try {
      const msg = JSON.parse(data);
      const { event, data: payload, request_id } = msg;

      // Find pending request by request_id (server now echoes it back)
      const pending = request_id
        ? this.pendingRequests.get(request_id)
        : this.pendingRequests.values().next().value;

      switch (event) {
        case "thinking":
          if (pending)
            pending.onProgress?.({ type: "thinking", data: payload });
          break;

        case "tool_use":
          if (pending) pending.onProgress?.({ type: "tool", data: payload });
          break;

        case "tool_result":
          if (pending)
            pending.onProgress?.({ type: "tool_result", data: payload });
          break;

        case "answer":
          if (pending) {
            pending.answer =
              (pending.answer || "") + (payload.event_data || "");
            pending.onProgress?.({
              type: "answer",
              data: payload.event_data,
            });
          }
          break;

        case "completed":
          if (pending) {
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(pending.requestId);
            this.metrics.requestsSuccess++;
            pending.resolve({
              success: true,
              answer: pending.answer || "",
              stats: pending.sessionStats || payload,
            });
          }
          break;

        case "session_stats":
          // Store stats for later use in completed event
          if (pending) {
            pending.sessionStats = payload;
          }
          break;

        case "stopped":
          if (pending) {
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(pending.requestId);
            this.metrics.requestsSuccess++;
            pending.resolve({
              success: true,
              stopped: true,
              data: payload,
            });
          }
          break;

        case "error":
          if (pending) {
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(pending.requestId);
            this.metrics.requestsFailed++;
            pending.reject(new Error(payload.message || "Server error"));
          }
          break;

        case "version":
          this.logger.debug("Server version:", payload);
          break;

        case "stats":
          if (pending) {
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(pending.requestId);
            this.metrics.requestsSuccess++;
            pending.resolve({ success: true, stats: payload });
          }
          break;

        default:
          this.logger.debug("Unknown event:", event);
      }
    } catch (err) {
      this.logger.error("Failed to parse message:", err.message);
    }
  }

  _handleError(err) {
    this.logger.error("WebSocket error:", err.message);
  }

  _handleClose(code, reason) {
    this.logger.warn("Connection closed:", code, reason.toString());
    this.connected = false;
    this._stopHeartbeat();

    if (!this._shutdown) {
      this._reconnect();
    }
  }

  // --------------------------------------------------------------------------
  // Request Methods
  // --------------------------------------------------------------------------

  async _sendRequest(payload, options = {}) {
    if (!this.connected) {
      throw new Error("Not connected");
    }

    const requestId = ++this.requestId;
    const requestPayload = {
      ...payload,
      request_id: requestId,
      session_id: payload.session_id || this.config.sessionId,
    };

    return new Promise((resolve, reject) => {
      // Setup timeout
      const timeout = setTimeout(() => {
        this.pendingRequests.delete(requestId);
        this.metrics.requestsFailed++;
        reject(new Error("Request timeout"));
      }, options.timeout || this.config.timeout.request);

      // Track pending request
      this.pendingRequests.set(requestId, {
        requestId,
        resolve,
        reject,
        timeout,
        answer: "",
        sessionStats: null,
        onProgress: options.onProgress,
      });

      // Send request
      this.metrics.requestsTotal++;
      this.logger.debug("Sending request:", payload.type);
      this.ws.send(JSON.stringify(requestPayload));
    });
  }

  /**
   * Execute a prompt on the AI agent
   * @param {string} prompt - The prompt to execute
   * @param {object} options - Execution options
   * @returns {Promise<{success: boolean, answer?: string, stats?: object}>}
   */
  async execute(prompt, options = {}) {
    const payload = {
      type: "execute",
      prompt,
      work_dir: options.workDir || this.config.workDir,
      system_prompt: options.systemPrompt || this.config.systemPrompt,
    };

    const startTime = Date.now();
    const result = await this._sendRequest(payload, options);
    this.metrics.totalLatencyMs += Date.now() - startTime;

    return result;
  }

  /**
   * Query server version
   * Note: version response doesn't include request_id, so we handle it separately
   */
  async getVersion() {
    if (!this.connected) {
      throw new Error("Not connected");
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error("Version request timeout"));
      }, 5000);

      const handler = (data) => {
        try {
          const msg = JSON.parse(data);
          if (msg.event === "version") {
            clearTimeout(timeout);
            this.ws.off("message", handler);
            resolve(msg.data);
          }
        } catch (err) {
          // Ignore parse errors for other messages
        }
      };

      this.ws.on("message", handler);
      this.ws.send(JSON.stringify({ type: "version" }));
    });
  }

  /**
   * Get session statistics
   */
  async getStats(sessionId) {
    return this._sendRequest({
      type: "stats",
      session_id: sessionId || this.config.sessionId,
    });
  }

  /**
   * Stop a running session
   * Note: If session is already completed, this may timeout or return error
   */
  async stop(sessionId, reason = "client_request") {
    try {
      return await this._sendRequest(
        {
          type: "stop",
          session_id: sessionId || this.config.sessionId,
          reason,
        },
        { timeout: 5000 }, // Short timeout for stop command
      );
    } catch (err) {
      // Session may already be completed, log and continue
      this.logger.debug("Stop command completed:", err.message);
      return { success: true, note: "Session may have already completed" };
    }
  }

  // --------------------------------------------------------------------------
  // Metrics
  // --------------------------------------------------------------------------

  getMetrics() {
    return {
      ...this.metrics,
      avgLatencyMs:
        this.metrics.requestsSuccess > 0
          ? Math.round(
              this.metrics.totalLatencyMs / this.metrics.requestsSuccess,
            )
          : 0,
    };
  }
}

// ============================================================================
// Demo / Main Entry
// ============================================================================

async function main() {
  console.log("=== HotPlex Enterprise Client Demo ===\n");

  const client = new HotPlexClient({
    sessionId: "enterprise-demo",
    logLevel: "debug",
    reconnect: { enabled: true, maxAttempts: 3 },
  });

  // Graceful shutdown handler
  const shutdown = async () => {
    console.log("\n\nShutting down gracefully...");
    await client.disconnect();
    console.log("Metrics:", client.getMetrics());
    process.exit(0);
  };

  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);

  try {
    // Step 1: Connect
    await client.connect();

    // Step 2: Version check
    const version = await client.getVersion();
    console.log("Server Version:", version);

    // Step 3: Execute task with progress callback
    console.log("\n--- Executing Task ---\n");

    const result = await client.execute(
      "List files in current directory and give a brief summary.",
      {
        systemPrompt: "You are a helpful DevOps assistant. Be concise.",
        onProgress: (event) => {
          switch (event.type) {
            case "thinking":
              process.stdout.write(".");
              break;
            case "tool":
              console.log("\n[Tool]", event.data.event_data);
              break;
            case "answer":
              process.stdout.write(event.data);
              break;
          }
        },
      },
    );

    console.log("\n\n--- Task Result ---");
    console.log("Success:", result.success);
    if (result.stats) {
      console.log(
        "Tokens:",
        result.stats.input_tokens,
        "/",
        result.stats.output_tokens,
      );
      console.log("Duration:", result.stats.total_duration_ms, "ms");
    }

    // Step 4: Get final stats
    const stats = await client.getStats();
    console.log("\n--- Session Stats ---");
    console.log(JSON.stringify(stats, null, 2));

    // Step 5: Cleanup
    await client.stop();
    await client.disconnect();

    console.log("\n=== Demo Complete ===");
    console.log("Final Metrics:", client.getMetrics());
  } catch (err) {
    console.error("\nDemo failed:", err.message);
    console.error("\nEnsure 'hotplexd' is running on localhost:8080");
    await client.disconnect();
    process.exit(1);
  }
}

// Export for use as module
module.exports = { HotPlexClient, Logger };

// Run demo if executed directly
if (require.main === module) {
  main();
}
