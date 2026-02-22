package hotplex

import (
	"context"

	"github.com/hrygo/hotplex/event"
	"github.com/hrygo/hotplex/types"
)

// HotPlexClient defines the comprehensive public API for the HotPlex engine.
// It integrates execution, session management, and safety configuration.
type HotPlexClient interface {
	Executor
	SessionController
	SafetyManager

	// Close gracefully terminates all managed sessions and releases resources.
	Close() error
}

// Executor handles the core execution logic and configuration validation.
type Executor interface {
	// Execute runs a command or prompt and streams normalized events.
	Execute(ctx context.Context, cfg *types.Config, prompt string, callback event.Callback) error

	// ValidateConfig checks if the session configuration is secure and valid.
	ValidateConfig(cfg *types.Config) error
}

// SessionController provides administrative control over persistent sessions.
type SessionController interface {
	// GetSessionStats returns telemetry and token usage for the given sessionID.
	// Note: Use the business-side sessionID provided during execution, not the internal
	// CLI-level session identifier. This sessionID maps to a specific background process.
	GetSessionStats(sessionID string) *SessionStats

	// StopSession forcibly terminates a persistent session and its underlying OS process group.
	// Note: Use the business-side sessionID (provided by the user) to identify which
	// specific agent instance to terminate.
	StopSession(sessionID string, reason string) error

	// GetCLIVersion returns the version string of the underlying AI CLI tool.
	GetCLIVersion() (string, error)
}

// SafetyManager controls the security boundaries and WAF settings.
type SafetyManager interface {
	// SetDangerAllowPaths configures the whitelist of safe directories for file I/O.
	SetDangerAllowPaths(paths []string)

	// SetDangerBypassEnabled toggles the regex WAF (requires valid admin token).
	SetDangerBypassEnabled(token string, enabled bool) error
}
