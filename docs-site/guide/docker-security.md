# Docker Security Isolation

## Container-Level Security for Multi-Bot Deployments

HotPlex implements multiple layers of security isolation for containerized multi-bot deployments. This document describes the security mechanisms and best practices.

---

## Security Architecture Overview

HotPlex uses a defense-in-depth approach with multiple isolation layers:

```
┌─────────────────────────────────────────────────────────┐
│                   Host Machine                          │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│  │  Bot 01     │  │  Bot 02     │  │  Bot 03     │   │
│  │  (PGID: X)  │  │  (PGID: Y)  │  │  (PGID: Z)  │   │
│  └─────────────┘  └─────────────┘  └─────────────┘   │
├─────────────────────────────────────────────────────────┤
│              Volume Isolation (per-instance)            │
├─────────────────────────────────────────────────────────┤
│              Network Isolation (localhost only)        │
└─────────────────────────────────────────────────────────┘
```

---

## Process Group Isolation (PGID)

### How It Works

Each HotPlex session spawns a dedicated process group with a unique **Process Group ID (PGID)**. This ensures:

1. **Process Tree Termination**: When a session fails or times out, the entire process tree is killed via `kill(-pgid, signal)`, preventing orphaned processes.
2. **Resource Cleanup**: All child processes (CLI tools, spawned agents) are terminated together.
3. **Zombie Prevention**: Proper signal handling prevents zombie processes.

### Implementation

```go
// From engine/runner.go
procAttr := &syscall.SysProcAttr{
    Setpgid: true,  // Create new process group
    Pgid: 0,        // Use PID as PGID
}

// Kill entire process group on termination
syscall.Kill(-pgid, syscall.SIGKILL)
```

### Container Environment

In Docker containers, PGID isolation works as follows:

| Scenario | Behavior |
| :------- | :-------- |
| Normal termination | All processes in group receive SIGTERM, then SIGKILL |
| Timeout | Engine sends SIGKILL to entire process group |
| Crash | Process group becomes orphaned, container runtime cleans up |

---

## Volume Isolation

### Per-Instance Storage

Each bot instance has its own isolated directory structure:

```
~/.hotplex/instances/
├── bot-01/
│   ├── storage/       # SQLite/PostgreSQL data
│   ├── projects/      # Code repositories
│   └── claude/        # Agent configuration
├── bot-02/
│   ├── storage/
│   ├── projects/
│   └── claude/
```

### Mount Configuration

In `docker-compose.yml`:

```yaml
services:
  hotplex-01:
    volumes:
      - hotplex-01-data:/home/hotplex/instances/bot-01
  hotplex-02:
    volumes:
      - hotplex-02-data:/home/hotplex/instances/bot-02

volumes:
  hotplex-01-data:
  hotplex-02-data:
```

### Best Practices

1. **Never share storage volumes** between bots
2. **Use named volumes** instead of host binds for production
3. **Backup regularly** using volume snapshot tools

---

## Network Isolation

### Localhost Binding

All bot services bind to `127.0.0.1` (localhost only), preventing external access:

```yaml
services:
  hotplex-01:
    ports:
      - "127.0.0.1:18080:8080"  # Only accessible from host
```

### Network Segmentation

For multi-tenant deployments, create separate networks:

```yaml
networks:
  bot-01-internal:
    driver: bridge
  bot-02-internal:
    driver: bridge
```

---

## Environment Variable Isolation

### Bot-Specific Configuration

Each bot instance loads its own `.env-XX` file:

| File | Purpose | Example Variables |
| :--- | :--- | :--- |
| `.env` | Global secrets | `HOTPLEX_API_KEY`, `ANTHROPIC_API_KEY` |
| `.env-01` | Bot 01 identity | `HOTPLEX_BOT_ID=bot-01`, `HOTPLEX_SLACK_BOT_USER_ID` |
| `.env-02` | Bot 02 identity | `HOTPLEX_BOT_ID=bot-02`, `HOTPLEX_SLACK_BOT_USER_ID` |

### Security Considerations

- **Never** put bot-specific tokens in the global `.env`
- Use **unique `bot_user_id`** for each instance to prevent session ID collisions
- Rotate credentials regularly

---

## Container Runtime Security

### Non-Root User

All HotPlex containers run as non-root user `hotplex`:

```dockerfile
FROM debian:bookworm-slim
RUN groupadd -g 1000 hotplex && useradd -r -u 1000 -g hotplex hotplex
USER hotplex
```

### Capabilities

Minimal Linux capabilities are granted:

| Capability | Purpose |
| :--------- | :------- |
| `NET_BIND_SERVICE` | Bind to privileged ports |
| `SYS_CHROOT` | Chroot filesystem |

### Read-Only Filesystem

For production, use read-only root filesystem:

```yaml
services:
  hotplex-01:
    read_only: true
    tmpfs:
      - /tmp
      - /run
```

---

## Label-Based Isolation

Docker labels provide metadata for security policies:

```yaml
services:
  hotplex-01:
    labels:
      hotplex.bot.id: "bot-01"
      hotplex.bot.role: "primary"
      hotplex.team: "engineering"
```

### Use Cases

- **Network policies**: Filter traffic by label
- **Monitoring**: Aggregate metrics by bot role
- **Access control**: Restrict admin operations

---

## Best Practices Checklist

### Deployment

- [ ] Use unique `bot_user_id` for each instance
- [ ] Bind ports to `127.0.0.1` only
- [ ] Use separate volumes per bot
- [ ] Enable network segmentation for multi-tenant

### Operations

- [ ] Monitor process group activity
- [ ] Set appropriate timeout values
- [ ] Implement logging aggregation
- [ ] Regular security scans with `trivy`

### Network

- [ ] Never expose bot ports publicly
- [ ] Use reverse proxy for external access
- [ ] Enable TLS in production
- [ ] Implement rate limiting

---

## Related Documentation

- [Docker Matrix](/guide/deployment) - Multi-bot orchestration
- [Security Overview](/guide/security) - General security model
- [Deployment Guide](/guide/deployment) - Production deployment
