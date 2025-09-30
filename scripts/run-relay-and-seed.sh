#!/usr/bin/env bash
set -euo pipefail

# run-relay-and-seed.sh
# Starts the ORLY relay with specified settings, then runs `bun dev:seed` in a
# provided Market repository to observe how the app interacts with the relay.
#
# Usage:
#   scripts/run-relay-and-seed.sh /path/to/market
#   MARKET_DIR=/path/to/market scripts/run-relay-and-seed.sh
#
# Notes:
# - This script removes /tmp/plebeian before starting the relay.
# - The relay listens on 0.0.0.0:3334
# - ORLY_ADMINS is intentionally empty and ACL is set to 'none'.
# - Requires: go, bun, curl

# ---------- Config ----------
RELAY_HOST="127.0.0.1"
RELAY_PORT="10547"
RELAY_DATA_DIR="/tmp/plebeian"
LOG_PREFIX="[relay]"
WAIT_TIMEOUT="45"  # seconds

# ---------- Resolve repo root ----------
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# ---------- Resolve Market directory ----------
MARKET_DIR="${1:-${MARKET_DIR:-}}"
if [[ -z "${MARKET_DIR}" ]]; then
  echo "ERROR: Market repository directory not provided. Set MARKET_DIR env or pass as first arg." >&2
  echo "Example: MARKET_DIR=$HOME/src/market scripts/run-relay-and-seed.sh" >&2
  exit 1
fi
if [[ ! -d "${MARKET_DIR}" ]]; then
  echo "ERROR: MARKET_DIR does not exist: ${MARKET_DIR}" >&2
  exit 1
fi

# ---------- Prerequisites ----------
command -v go >/dev/null 2>&1 || { echo "ERROR: 'go' not found in PATH" >&2; exit 1; }
command -v bun >/dev/null 2>&1 || { echo "ERROR: 'bun' not found in PATH. Install Bun: https://bun.sh" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "ERROR: 'curl' not found in PATH" >&2; exit 1; }

# ---------- Cleanup handler ----------
RELAY_PID=""
cleanup() {
  set +e
  if [[ -n "${RELAY_PID}" ]]; then
    echo "${LOG_PREFIX} stopping relay (pid=${RELAY_PID})" >&2
    kill "${RELAY_PID}" 2>/dev/null || true
    wait "${RELAY_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

# ---------- Start relay ----------
reset || true
rm -rf "${RELAY_DATA_DIR}"

# Run go relay in background with required environment variables
(
  export ORLY_LOG_LEVEL="trace"
  export ORLY_LISTEN="0.0.0.0"
  export ORLY_PORT="${RELAY_PORT}"
  export ORLY_ADMINS=""
  export ORLY_ACL_MODE="none"
  export ORLY_DATA_DIR="${RELAY_DATA_DIR}"
  # Important: run from repo root
  cd "${REPO_ROOT}"
  # Prefix relay logs so they are distinguishable
  stdbuf -oL -eL go run . 2>&1 | sed -u "s/^/${LOG_PREFIX} /"
) &
RELAY_PID=$!
echo "${LOG_PREFIX} started (pid=${RELAY_PID}), waiting for readiness on ${RELAY_HOST}:${RELAY_PORT} …"

# ---------- Wait for readiness ----------
start_ts=$(date +%s)
while true; do
  if curl -fsS "http://${RELAY_HOST}:${RELAY_PORT}/" >/dev/null 2>&1; then
    break
  fi
  now=$(date +%s)
  if (( now - start_ts > WAIT_TIMEOUT )); then
    echo "ERROR: relay did not become ready within ${WAIT_TIMEOUT}s" >&2
    exit 1
  fi
  sleep 1
done
echo "${LOG_PREFIX} ready. Running Market seeding…"

# ---------- Run market seeding ----------
(
  cd "${MARKET_DIR}"
  # Stream bun output with clear prefix
  stdbuf -oL -eL bun dev:seed 2>&1 | sed -u 's/^/[market] /'
)

# After seeding completes, keep the relay up briefly for inspection
echo "${LOG_PREFIX} seeding finished. Relay is still running for inspection. Press Ctrl+C to stop."
# Wait indefinitely until interrupted, to allow observing relay logs/behavior
while true; do sleep 3600; done
