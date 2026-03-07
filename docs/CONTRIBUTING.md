# HotPlex Contributing Guide

> Detailed guide for contributors. For quick start, see [DEVELOPMENT.md](DEVELOPMENT.md).
> Also see the root [CONTRIBUTING.md](../CONTRIBUTING.md) for the basics.

## Table of Contents

- [Development Philosophy](#development-philosophy)
- [Code Standards](#code-standards)
- [PR Workflow](#pr-workflow)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Review Process](#review-process)

---

## Development Philosophy

### First Principles

1. **Leverage vs Build**: Bridge existing AI CLI tools into production, don't reinvent
2. **Cli-as-a-Service**: Transform one-off CLI into long-lived services
3. **Security First**: Every execution is isolated; WAF is mandatory

### Architectural Rules

| Rule | Description |
|------|-------------|
| Public Thin, Private Thick | Root package provides minimal API surface |
| Strategy Pattern | Provider interface decouples engine from AI tools |
| PGID-First Security | Every process in a dedicated process group |
| IO-Driven State Machine | No fixed sleeps, use IO markers |
| SDK-First | Use official platform SDKs (slack-go, telegram-bot-api) |

---

## Code Standards

### Go Style

Follow [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md):

```go
// ✅ Good: Named mutex field
type SessionPool struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}

// ❌ Bad: Embedded mutex
type SessionPool struct {
    sync.Mutex  // Leaks implementation
    sessions map[string]*Session
}
```

### Interface Compliance

**MANDATORY**: All interface implementations must have compile-time verification:

```go
// Every implementation must have this:
var _ ChatAdapter = (*SlackAdapter)(nil)
var _ EngineSupport = (*SlackAdapter)(nil)
```

### Error Handling

- Never use `panic()` in core engine
- Return explicit errors, wrap with `%w`
- Use `log/slog` with context

```go
// ✅ Good
if err != nil {
    return fmt.Errorf("session start: %w", err)
}

// ❌ Bad
if err != nil {
    panic(err)
}
```

### Concurrency

- Use `sync.RWMutex` for `SessionPool`
- Always `defer mu.Unlock()` immediately after Lock
- Zero deadlock tolerance

```go
// ✅ Good
p.mu.Lock()
defer p.mu.Unlock()

// ❌ Bad - easy to miss unlock
p.mu.Lock()
if condition {
    p.Unlock()
    return
}
p.Unlock()
```

---

## PR Workflow

### Branch Naming

```
<type>/<issue-id>-short-description

Examples:
feat/123-add-slack-streaming
fix/456-session-memory-leak
docs/789-update-api-reference
refactor/101-simplify-pool-logic
```

### Commit Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(scope): description

Types:
- feat: New feature
- fix: Bug fix
- refactor: Code refactoring
- docs: Documentation
- test: Tests
- chore: Maintenance
- wip: Work in progress

Examples:
feat(slack): add native streaming support
fix(pool): resolve memory leak on cleanup
docs(sdk): update examples
wip: checkpoint for feature X
```

### Atomic Commits

- One commit per independent logic unit
- Commit frequently (every 50 lines or logical unit)
- Use `wip:` prefix for checkpoints

### PR Description

**MANDATORY**: Include issue reference in PR body:

```markdown
## Summary
Brief description of changes.

## Changes
- Change 1
- Change 2

## Test Plan
- [ ] Unit tests pass
- [ ] Manual testing done

Resolves #123  <!-- REQUIRED for auto-close -->
```

### Pre-commit Checklist

```bash
# Run before every commit
make lint test

# Or individually
make fmt        # Format code
make vet        # Vet code
make lint       # Run linter
make test       # Run tests
```

---

## Testing Guidelines

### Test Requirements

- All features require unit tests
- Mock heavy I/O (use echo/cat for CLI)
- `go test -race` must pass

### Test Structure

```go
// Use table-driven tests
func TestSessionPool(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "test", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Test Commands

```bash
# Fast unit tests
make test

# With race detection
make test-race

# Integration tests
make test-integration

# CI-optimized
make test-ci
```

### Mocking

For CLI mocking, use simple echo/cat:

```go
// Mock provider
type MockProvider struct {
    responses []string
}

func (m *MockProvider) BuildArgs(ctx context.Context, cfg *types.Config) ([]string, error) {
    return []string{"echo", "mock response"}, nil
}
```

---

## Documentation

### Docs-First Policy

Any PR modifying public APIs must update documentation:

| Change Type | Update Location |
|-------------|-----------------|
| API changes | `docs/server/api.md`, SDK docs |
| New features | Appropriate manual in `docs/` |
| User-facing | `README.md` |
| Architecture | `docs/ARCHITECTURE.md` |

### Documentation Structure

```
docs/
├── ARCHITECTURE.md   # This file - system design
├── DEVELOPMENT.md    # Local development
├── CONFIGURATION.md  # Config reference
├── CONTRIBUTING.md   # This file
├── architecture.md   # Detailed architecture
├── sdk-guide.md      # SDK documentation
├── quick-start.md    # Quick start guide
└── chatapps/         # Platform-specific guides
```

### Writing Style

- Use clear, concise language
- Include code examples
- Link to related documentation
- Keep tables and lists formatted

---

## Review Process

### CI Checks

All PRs must pass:

1. **Code Format**: `go fmt ./...`
2. **Linting**: `golangci-lint run`
3. **Tests**: `go test -race ./...`
4. **Build**: `go build ./...`

### Review Criteria

Reviewers will check:

- [ ] Code follows style guide
- [ ] Interface compliance verified at compile-time
- [ ] No panics in core engine
- [ ] Proper error handling with `%w` wrapping
- [ ] Thread-safe session pool access
- [ ] Tests for new functionality
- [ ] Documentation updated

### Response Time

- Initial review: Within 2-3 business days
- Follow-up reviews: Within 1-2 business days

---

## Git Safety Protocol

### Forbidden Actions

In shared development, these commands destroy others' work:

- `git checkout -- .`
- `git reset --hard`
- `git restore .`
- `git clean -fd`

### Required Checks

1. Run `git status` before any git operation
2. Review all files before destructive actions
3. Use `git stash` for temporary saves
4. Commit frequently to "claim" progress

### Recovery

If mistakes happen:
1. Check IDE **Timeline** for local changes
2. Use `git fsck --lost-found` for lost commits
3. Contact team before force operations

---

## Related Documentation

- [DEVELOPMENT.md](DEVELOPMENT.md) - Local development setup
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture overview
- [CONFIGURATION.md](CONFIGURATION.md) - Configuration reference
- [uber-go-style-guide.md](../.agent/rules/uber-go-style-guide.md) - Go style reference
