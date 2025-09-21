#!/usr/bin/env bash
# scripts/update-embedded-web.sh
# Build the embedded web UI and then install the Go binary.
#
# This script will:
#  - Build the React app in app/web to app/web/dist using Bun (preferred),
#    or fall back to npm/yarn/pnpm if Bun isn't available.
#  - Run `go install` from the repository root so the binary picks up the new
#    embedded assets.
#
# Usage:
#   ./scripts/update-embedded-web.sh
#
# Requirements:
#  - Go 1.18+ installed (for `go install` and go:embed support)
#  - Bun (https://bun.sh) recommended; alternatively Node.js with npm/yarn/pnpm
#
set -euo pipefail

# Resolve repo root to allow running from anywhere
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
WEB_DIR="${REPO_ROOT}/app/web"

log() { printf "[update-embedded-web] %s\n" "$*"; }
err() { printf "[update-embedded-web][ERROR] %s\n" "$*" >&2; }

if [[ ! -d "${WEB_DIR}" ]]; then
  err "Expected web directory at ${WEB_DIR} not found."
  exit 1
fi

# Choose a JS package runner
JS_RUNNER=""
if command -v bun >/dev/null 2>&1; then
  JS_RUNNER="bun"
elif command -v npm >/dev/null 2>&1; then
  JS_RUNNER="npm"
elif command -v yarn >/dev/null 2>&1; then
  JS_RUNNER="yarn"
elif command -v pnpm >/dev/null 2>&1; then
  JS_RUNNER="pnpm"
else
  err "No JavaScript package manager found. Install Bun (recommended) or npm/yarn/pnpm."
  exit 1
fi

log "Using JavaScript runner: ${JS_RUNNER}"

# Install dependencies and build the web app
log "Installing frontend dependencies..."
pushd "${WEB_DIR}" >/dev/null
case "${JS_RUNNER}" in
  bun)
    bun install
    log "Building web app with Bun..."
    bun run build
    ;;
  npm)
    npm ci || npm install
    log "Building web app with npm..."
    npm run build
    ;;
  yarn)
    yarn install --frozen-lockfile || yarn install
    log "Building web app with yarn..."
    yarn build
    ;;
  pnpm)
    pnpm install --frozen-lockfile || pnpm install
    log "Building web app with pnpm..."
    pnpm build
    ;;
  *)
    err "Unsupported JS runner: ${JS_RUNNER}"
    exit 1
    ;;

esac
popd >/dev/null

# Verify the output directory expected by go:embed exists
DIST_DIR="${WEB_DIR}/dist"
if [[ ! -d "${DIST_DIR}" ]]; then
  err "Build did not produce ${DIST_DIR}. Check your frontend build configuration."
  exit 1
fi

log "Frontend build complete at ${DIST_DIR}."

# Install the Go binary so it embeds the latest files
log "Running 'go install' from repo root..."
pushd "${REPO_ROOT}" >/dev/null
GO111MODULE=on go install ./...
popd >/dev/null

log "Done. Your installed binary now includes the updated embedded web UI."