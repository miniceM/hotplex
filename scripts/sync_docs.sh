#!/usr/bin/env bash
#
# HotPlex Documentation SSOT Sync Script
# This script serves as the Single Source of Truth (SSOT) builder.
# It copies markdown files from their core repository locations into the VitePress docs-site structure.
#

set -euo pipefail

# Configuration
SOURCE_ROOT="."
TARGET_ROOT="docs-site"
DOCS_DIR="docs"

# Color output helpers
INFO="\033[0;34m"
SUCCESS="\033[0;32m"
WARN="\033[0;33m"
ERROR="\033[0;31m"
NC="\033[0m" # No Color

log_info() { echo -e "${INFO}ℹ️  $1${NC}"; }
log_success() { echo -e "${SUCCESS}✅ $1${NC}"; }
log_warn() { echo -e "${WARN}⚠️  $1${NC}"; }
log_error() { echo -e "${ERROR}❌ $1${NC}"; exit 1; }

# Safe copy function with source validation
safe_cp() {
    local src="$1"
    local dst="$2"
    if [[ -f "$src" ]]; then
        cp "$src" "$dst"
    else
        log_warn "Source file not found: $src (skipping)"
    fi
}

log_info "Synchronizing documentation sources to docs-site..."

# Ensure target directories exist
log_info "Creating target directories..."
mkdir -p "$TARGET_ROOT/guide"
mkdir -p "$TARGET_ROOT/sdks"
mkdir -p "$TARGET_ROOT/reference"
mkdir -p "$TARGET_ROOT/public/images"
mkdir -p "$TARGET_ROOT/public/assets/"
mkdir -p "$TARGET_ROOT/plan"
mkdir -p "$TARGET_ROOT/providers"

# --- Guides ---
log_info "Syncing guide files..."
safe_cp "README.md" "$TARGET_ROOT/guide/getting-started.md"
safe_cp "README_zh.md" "$TARGET_ROOT/guide/getting-started_zh.md"
safe_cp "$DOCS_DIR/quick-start.md" "$TARGET_ROOT/guide/quick-start.md"
safe_cp "$DOCS_DIR/quick-start_zh.md" "$TARGET_ROOT/guide/quick-start_zh.md"
safe_cp "$DOCS_DIR/architecture.md" "$TARGET_ROOT/guide/architecture.md"
safe_cp "$DOCS_DIR/architecture_zh.md" "$TARGET_ROOT/guide/architecture_zh.md"
safe_cp "SECURITY.md" "$TARGET_ROOT/guide/security.md"
safe_cp "$DOCS_DIR/server/api.md" "$TARGET_ROOT/guide/websocket.md"
safe_cp "$DOCS_DIR/hooks-architecture.md" "$TARGET_ROOT/guide/hooks.md"
safe_cp "$DOCS_DIR/hooks-architecture_zh.md" "$TARGET_ROOT/guide/hooks_zh.md"
safe_cp "$DOCS_DIR/observability-guide.md" "$TARGET_ROOT/guide/observability.md"
safe_cp "$DOCS_DIR/observability-guide_zh.md" "$TARGET_ROOT/guide/observability_zh.md"
safe_cp "$DOCS_DIR/docker-deployment.md" "$TARGET_ROOT/guide/docker.md"
safe_cp "$DOCS_DIR/docker-deployment_zh.md" "$TARGET_ROOT/guide/docker_zh.md"
safe_cp "$DOCS_DIR/production-guide.md" "$TARGET_ROOT/guide/deployment.md"
safe_cp "$DOCS_DIR/production-guide_zh.md" "$TARGET_ROOT/guide/deployment_zh.md"
safe_cp "$DOCS_DIR/benchmark-report.md" "$TARGET_ROOT/guide/performance.md"
safe_cp "$DOCS_DIR/benchmark-report_zh.md" "$TARGET_ROOT/guide/performance_zh.md"
safe_cp "$DOCS_DIR/chatapps/chatapps-guide.md" "$TARGET_ROOT/guide/chatapps.md"
safe_cp "$DOCS_DIR/chatapps/chatapps-slack.md" "$TARGET_ROOT/guide/chatapps-slack.md"
safe_cp "$DOCS_DIR/chatapps/slack-gap-analysis.md" "$TARGET_ROOT/guide/slack-gap-analysis.md"
safe_cp "$DOCS_DIR/chatapps/chatapps-dingtalk-analysis.md" "$TARGET_ROOT/guide/chatapps-dingtalk.md"

# --- AI Providers ---
log_info "Syncing AI provider guides..."
safe_cp "$DOCS_DIR/providers/claudecode.md" "$TARGET_ROOT/providers/claude.md"
safe_cp "$DOCS_DIR/providers/claudecode_zh.md" "$TARGET_ROOT/providers/claude_zh.md"
safe_cp "$DOCS_DIR/providers/opencode.md" "$TARGET_ROOT/providers/opencode.md"
safe_cp "$DOCS_DIR/providers/opencode_zh.md" "$TARGET_ROOT/providers/opencode_zh.md"

# --- SDKs ---
log_info "Syncing SDK files..."
safe_cp "$DOCS_DIR/sdk-guide.md" "$TARGET_ROOT/sdks/go-sdk.md"
safe_cp "$DOCS_DIR/sdk-guide_zh.md" "$TARGET_ROOT/sdks/go-sdk_zh.md"
safe_cp "sdks/python/README.md" "$TARGET_ROOT/sdks/python-sdk.md"
safe_cp "sdks/typescript/README.md" "$TARGET_ROOT/sdks/typescript-sdk.md"

# --- Reference ---
log_info "Syncing reference files..."
safe_cp "$DOCS_DIR/server/api.md" "$TARGET_ROOT/reference/api.md"
safe_cp "$DOCS_DIR/server/api_zh.md" "$TARGET_ROOT/reference/api_zh.md"
safe_cp "$DOCS_DIR/README.md" "$TARGET_ROOT/reference/index.md"
safe_cp "$DOCS_DIR/README_zh.md" "$TARGET_ROOT/reference/index_zh.md"

# --- Plan ---
log_info "Syncing plan files..."
safe_cp "$DOCS_DIR/plan/technical-plan-draft.md" "$TARGET_ROOT/plan/technical-plan.md"

# --- Assets ---
log_info "Syncing assets..."
if [[ -d "$DOCS_DIR/images" ]]; then
    cp -r "$DOCS_DIR/images/"* "$TARGET_ROOT/public/images/"
fi

if [[ -d ".github/assets" ]]; then
    cp -r ".github/assets/"* "$TARGET_ROOT/public/assets/"
fi

# --- Path Fixes for VitePress ---
log_info "Fixing paths and links for VitePress..."

# Fix image paths
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -i.bak \
    -e 's|docs/images|/images|g' \
    -e 's|\.\./images|/images|g' \
    -e 's|\./images|/images|g' \
    -e 's|\.github/assets|/assets|g' \
    {} +

# Fix Bilingual Cross-links & Internal VitePress Links using regex (sed -E)
# AI Provider Guides rewrites
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(providers/)?claudecode(_zh)?(\.md)?\)|](/providers/claude\3.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(providers/)?claudecode_(zh)?(\.md)?\)|](/providers/claude_\4.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(providers/)?opencode(_zh)?(\.md)?\)|](/providers/opencode\3.md)|g' {} +

# Self-references in providers
find "$TARGET_ROOT/providers" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(claudecode(_zh)?\.md\)|](/providers/claude\1.md)|g' {} +
find "$TARGET_ROOT/providers" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(opencode(_zh)?\.md\)|](/providers/opencode\1.md)|g' {} +

# Remove legacy migration sections from index files
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak '/Developer Migration/d' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak '/migration-guide-v0/d' {} +

# Go SDK Links
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?sdk-guide(_zh)?(\.md)?\)|](/sdks/go-sdk\2.md)|g' {} +

# API Reference Links
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(server/)?api(_zh)?(\.md)?\)|](/reference/api\3.md)|g' {} +

# Getting Started / README Links
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?README(_zh)?\.md\)|](/guide/getting-started\1.md)|g' {} +

# Other Internal Guides rewritten for# Architecture Links (Standardized)
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?quick-start(_zh)?(\.md)?\)|](/guide/quick-start\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?architecture(_zh)?(\.md)?\)|](/guide/architecture\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(architecture(_zh)?\.md\)|](/guide/architecture\1.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?observability-guide(_zh)?(\.md)?\)|](/guide/observability\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?docker-deployment(_zh)?(\.md)?\)|](/guide/docker\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?production-guide(_zh)?(\.md)?\)|](/guide/deployment\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?benchmark-report(_zh)?(\.md)?\)|](/guide/performance\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(chatapps/)?chatapps-guide(_zh)?(\.md)?\)|](/guide/chatapps.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(chatapps/)?chatapps-dingtalk-analysis(_zh)?(\.md)?\)|](/guide/chatapps-dingtalk.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(chatapps/)?chatapps-slack(_zh)?(\.md)?\)|](/guide/chatapps-slack.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(chatapps/)?slack-gap-analysis(_zh)?(\.md)?\)|](/guide/slack-gap-analysis.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(plan/)?technical-plan-draft(_zh)?(\.md)?\)|](/plan/technical-plan.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?chatapps-design(_zh)?(\.md)?\)|](/guide/chatapps.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?hooks-architecture(_zh)?(\.md)?\)|](/guide/hooks\2.md)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's!\]\(\.?/?SECURITY(\.md)?\)!](/guide/security.md)!g' {} +

# Redirect GitHub-only URLs (Examples, CONTRIBUTING, LICENSE, Roadmap, ClaudeCode)
# 1. Transform specific folders to GitHub tree links (only if not already a URL)
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.\./(_examples\|cmd\|internal\|pkg\|scripts\|sdks\|chatapps)/([^)]*)\)|](https://github.com/hrygo/hotplex/tree/main/\1/\2)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\((_examples\|cmd\|internal\|pkg\|scripts\|sdks\|chatapps)/([^)]*)\)|](https://github.com/hrygo/hotplex/tree/main/\1/\2)|g' {} +

# 2. Transform specific file types to GitHub blob links (only for relative paths)
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(([^)]*\.(go\|sh\|yml\|yaml\|json\|mod\|sum\|proto))\)|](https://github.com/hrygo/hotplex/blob/main/\1)|g' {} +
# Prevent double wrapping if it happened
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -i.bak 's|https://github.com/hrygo/hotplex/blob/main/https://github.com/|https://github.com/|g' {} +

# 3. Handle pointers to root files (e.g., CONTRIBUTING.md, LICENSE)
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(CONTRIBUTING\|LICENSE\|SECURITY\|AGENT\|CLAUDE)(\.md)?\)|](https://github.com/hrygo/hotplex/blob/main/\1\2)|g' {} +
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.\./(CONTRIBUTING\|LICENSE\|SECURITY\|AGENT\|CLAUDE)(\.md)?\)|](https://github.com/hrygo/hotplex/blob/main/\1\2)|g' {} +

# 4. Specific known links
find "$TARGET_ROOT" -name "*.md" -type f -exec sed -E -i.bak 's|\]\(\.?/?(docs/)?(archive/)?roadmap-2026(\.md)?\)|](https://github.com/hrygo/hotplex/blob/main/docs/archive/roadmap-2026.md)|g' {} +

# Clean up sed backups immediately after each find command
find "$TARGET_ROOT" -name "*.bak" -type f -delete

log_success "Documentation successfully synchronized."

