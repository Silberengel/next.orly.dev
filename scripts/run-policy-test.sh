#!/bin/bash
set -euo pipefail

# Config
PORT=${PORT:-34567}
URL=${URL:-ws://127.0.0.1:${PORT}}
LOG=/tmp/orly-policy.out
PID=/tmp/orly-policy.pid
DATADIR=$(mktemp -d)

cleanup() {
  trap - EXIT
  if [[ -f "$PID" ]]; then
    kill -INT "$(cat "$PID")" 2>/dev/null || true
    rm -f "$PID"
  fi
  rm -rf "$DATADIR"
}
trap cleanup EXIT

echo "Building relay and test client..."
go build -o orly .
go build -o cmd/policytest/policytest ./cmd/policytest

echo "Starting relay on ${URL} with policy enabled (data dir: ${DATADIR})..."
ORLY_DATA_DIR="$DATADIR" \
ORLY_PORT=${PORT} \
ORLY_POLICY_ENABLED=true \
ORLY_ACL_MODE=none \
ORLY_LOG_LEVEL=trace \
./orly >"$LOG" 2>&1 & echo $! >"$PID"

sleep 1
if ! ps -p "$(cat "$PID")" >/dev/null 2>&1; then
  echo "Relay failed to start; logs:" >&2
  sed -n '1,200p' "$LOG" >&2
  exit 1
fi

echo "Running policy test against ${URL}..."
set +e
out=$(cmd/policytest/policytest -url "${URL}" 2>&1)
rc=$?
set -e
echo "$out"

# Expect rejection; return 0 if we saw REJECT, else forward exit code
if grep -q '^REJECT:' <<<"$out"; then
  exit 0
fi
exit $rc


