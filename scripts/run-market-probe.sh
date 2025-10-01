#!/usr/bin/env bash
set -euo pipefail

# run-market-probe.sh
# Starts the ORLY relay with relaxed ACL, then executes the Market repo's
# scripts/startup.ts to publish seed events and finally runs a small NDK-based
# fetcher to verify the events can be read back from the relay. The goal is to
# print detailed logs to diagnose end-to-end publish/subscribe behavior.
#
# Usage:
#   scripts/run-market-probe.sh /path/to/market <hex_private_key>
#   MARKET_DIR=/path/to/market APP_PRIVATE_KEY=hex scripts/run-market-probe.sh
#
# Requirements:
#   - go, bun, curl
#   - Market repo available locally with scripts/startup.ts (see path above)
#
# Behavior:
#   - Clears relay data dir (/tmp/plebeian) each run
#   - Starts relay on 127.0.0.1:10547 with ORLY_ACL_MODE=none (no auth needed)
#   - Exports APP_RELAY_URL to ws://127.0.0.1:10547 for the Market startup.ts
#   - Runs Market's startup.ts to publish events (kinds 31990, 10002, 10000, 30000)
#   - Runs a temporary TypeScript fetcher using NDK to subscribe & log results
#

# ---------- Config ----------
RELAY_HOST="127.0.0.1"
RELAY_PORT="10547"
RELAY_DATA_DIR="/tmp/plebeian"
WAIT_TIMEOUT="120"  # seconds - increased for slow startup
RELAY_LOG_PREFIX="[relay]"
MARKET_LOG_PREFIX="[market-seed]"
FETCH_LOG_PREFIX="[fetch]"

# ---------- Resolve repo root ----------
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# ---------- Resolve Market directory and private key ----------
MARKET_DIR=${1:-${MARKET_DIR:-}}
APP_PRIVATE_KEY_INPUT=${2:-${APP_PRIVATE_KEY:-${NOSTR_SK:-}}}
if [[ -z "${MARKET_DIR}" ]]; then
  echo "ERROR: Market repository directory not provided. Set MARKET_DIR env or pass as first arg." >&2
  echo "Example: MARKET_DIR=$HOME/src/github.com/PlebianApp/market scripts/run-market-probe.sh" >&2
  exit 1
fi
if [[ ! -d "${MARKET_DIR}" ]]; then
  echo "ERROR: MARKET_DIR does not exist: ${MARKET_DIR}" >&2
  exit 1
fi
if [[ -z "${APP_PRIVATE_KEY_INPUT}" ]]; then
  echo "ERROR: Private key not provided. Pass as 2nd arg or set APP_PRIVATE_KEY or NOSTR_SK env var." >&2
  exit 1
fi

# ---------- Prerequisites ----------
command -v go   >/dev/null 2>&1 || { echo "ERROR: 'go' not found in PATH" >&2; exit 1; }
command -v bun  >/dev/null 2>&1 || { echo "ERROR: 'bun' not found in PATH. Install Bun: https://bun.sh" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "ERROR: 'curl' not found in PATH" >&2; exit 1; }

# ---------- Cleanup handler ----------
RELAY_PID=""
TMP_FETCH_DIR=""
TMP_FETCH_TS=""
cleanup() {
  set +e
  if [[ -n "${RELAY_PID}" ]]; then
    echo "${RELAY_LOG_PREFIX} stopping relay (pid=${RELAY_PID})" >&2
    kill "${RELAY_PID}" 2>/dev/null || true
    wait "${RELAY_PID}" 2>/dev/null || true
  fi
  if [[ -n "${TMP_FETCH_DIR}" && -d "${TMP_FETCH_DIR}" ]]; then
    rm -rf "${TMP_FETCH_DIR}" || true
  fi
}
trap cleanup EXIT INT TERM

# ---------- Start relay ----------
reset || true
rm -rf "${RELAY_DATA_DIR}"
(
  export ORLY_LOG_LEVEL="trace"
  export ORLY_LISTEN="0.0.0.0"
  export ORLY_PORT="${RELAY_PORT}"
  export ORLY_ADMINS=""          # ensure no admin ACL
  export ORLY_ACL_MODE="none"     # fully open for test
  export ORLY_DATA_DIR="${RELAY_DATA_DIR}"
  cd "${REPO_ROOT}"
  stdbuf -oL -eL go run . 2>&1 | sed -u "s/^/${RELAY_LOG_PREFIX} /"
) &
RELAY_PID=$!
echo "${RELAY_LOG_PREFIX} started (pid=${RELAY_PID}), waiting for readiness on ${RELAY_HOST}:${RELAY_PORT} …"

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
 echo "${RELAY_LOG_PREFIX} ready. Starting Market publisher…"
 
 # ---------- Publish via Market's startup.ts ----------
 (
   export APP_RELAY_URL="ws://${RELAY_HOST}:${RELAY_PORT}"
   export APP_PRIVATE_KEY="${APP_PRIVATE_KEY_INPUT}"
   cd "${MARKET_DIR}"
   # Use bun to run the exact startup.ts the app uses. Expect its dependencies in Market repo.
   echo "${MARKET_LOG_PREFIX} running scripts/startup.ts against ${APP_RELAY_URL} …"
   stdbuf -oL -eL bun run scripts/startup.ts 2>&1 | sed -u "s/^/${MARKET_LOG_PREFIX} /"
 )
 
 # ---------- Prepare a temporary NDK fetcher workspace ----------
 TMP_FETCH_DIR=$(mktemp -d /tmp/ndk-fetch-XXXXXX)
 TMP_FETCH_TS="${TMP_FETCH_DIR}/probe.ts"
 
 # Write probe script
 cat >"${TMP_FETCH_TS}" <<'TS'
 import { config } from 'dotenv'
 config()
 
 const RELAY_URL = process.env.APP_RELAY_URL
 const APP_PRIVATE_KEY = process.env.APP_PRIVATE_KEY
 
 if (!RELAY_URL || !APP_PRIVATE_KEY) {
   console.error('[fetch] Missing APP_RELAY_URL or APP_PRIVATE_KEY in env')
   process.exit(2)
 }
 
 // Use NDK like startup.ts does
 import NDK, { NDKEvent, NDKPrivateKeySigner, NDKFilter } from '@nostr-dev-kit/ndk'
 
 const relay = RELAY_URL as string
 const privateKey = APP_PRIVATE_KEY as string
 
 async function main() {
   console.log(`[fetch] initializing NDK -> ${relay}`)
   const ndk = new NDK({ explicitRelayUrls: [relay] })
   ndk.pool?.on('relay:connect', (r) => console.log('[fetch] relay connected:', r.url))
   ndk.pool?.on('relay:disconnect', (r) => console.log('[fetch] relay disconnected:', r.url))
   ndk.pool?.on('relay:notice', (r, msg) => console.log('[fetch] relay notice:', r.url, msg))
 
   await ndk.connect(8000)
   console.log('[fetch] connected')
 
   // Setup signer and derive pubkey
   const signer = new NDKPrivateKeySigner(privateKey)
   ndk.signer = signer
   await signer.blockUntilReady()
   const pubkey = (await signer.user())?.pubkey
   console.log('[fetch] signer pubkey:', pubkey)
 
   // Subscribe to the kinds published by startup.ts authored by pubkey
   const filters: NDKFilter[] = [
     { kinds: [31990, 10002, 10000, 30000], authors: pubkey ? [pubkey] : undefined, since: Math.floor(Date.now()/1000) - 3600 },
   ]
   console.log('[fetch] subscribing with filters:', JSON.stringify(filters))
 
   const sub = ndk.subscribe(filters, { closeOnEose: true })
   let count = 0
   const received: string[] = []
 
   sub.on('event', (e: NDKEvent) => {
     count++
     received.push(`${e.kind}:${e.tagValue('d') || ''}:${e.id}`)
     console.log('[fetch] EVENT kind=', e.kind, 'id=', e.id, 'tags=', e.tags)
   })
   sub.on('eose', () => {
     console.log('[fetch] EOSE received; total events:', count)
   })
   sub.on('error', (err: any) => {
     console.error('[fetch] subscription error:', err)
   })
 
   // Also try to fetch by kinds one by one to be verbose
   const kinds = [31990, 10002, 10000, 30000]
   for (const k of kinds) {
     try {
       const e = await ndk.fetchEvent({ kinds: [k], authors: pubkey ? [pubkey] : undefined }, { cacheUsage: 'ONLY_RELAY' })
       if (e) {
         console.log(`[fetch] fetchEvent kind=${k} -> id=${e.id}`)
       } else {
         console.log(`[fetch] fetchEvent kind=${k} -> not found`)
       }
     } catch (err) {
       console.error(`[fetch] fetchEvent kind=${k} error`, err)
     }
   }
 
   // Wait a bit to allow sub to drain
   await new Promise((res) => setTimeout(res, 2000))
   console.log('[fetch] received summary:', received)
   // Note: NDK v2.14.x does not expose pool.close(); rely on closeOnEose and process exit
 }
 
 main().catch((e) => {
   console.error('[fetch] fatal error:', e)
   process.exit(3)
 })
TS
 
 # Write minimal package.json to pin dependencies and satisfy NDK peer deps
 cat >"${TMP_FETCH_DIR}/package.json" <<'JSON'
 {
   "name": "ndk-fetch-probe",
   "version": "0.0.1",
   "private": true,
   "type": "module",
   "dependencies": {
     "@nostr-dev-kit/ndk": "^2.14.36",
     "nostr-tools": "^2.7.0",
     "dotenv": "^16.4.5"
   }
 }
JSON
 
 # ---------- Install probe dependencies explicitly (avoid Bun auto-install pitfalls) ----------
 (
   cd "${TMP_FETCH_DIR}"
   echo "${FETCH_LOG_PREFIX} installing probe deps (@nostr-dev-kit/ndk, nostr-tools, dotenv) …"
   stdbuf -oL -eL bun install 2>&1 | sed -u "s/^/${FETCH_LOG_PREFIX} [install] /"
 )
 
 # ---------- Run the fetcher ----------
 (
   export APP_RELAY_URL="ws://${RELAY_HOST}:${RELAY_PORT}"
   export APP_PRIVATE_KEY="${APP_PRIVATE_KEY_INPUT}"
   echo "${FETCH_LOG_PREFIX} running fetch probe against ${APP_RELAY_URL} …"
   (
     cd "${TMP_FETCH_DIR}"
     stdbuf -oL -eL bun "${TMP_FETCH_TS}" 2>&1 | sed -u "s/^/${FETCH_LOG_PREFIX} /"
   )
 )
 
 echo "[probe] Completed. Review logs above for publish/subscribe flow."