#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="/work/reports"
BIN="/build/relay"
CPU_DIR="/profiles/cpu"
MEM_DIR="/profiles/mem"
ALLOC_DIR="/profiles/alloc"
REPORT="$OUT_DIR/profile-analysis.txt"

mkdir -p "$OUT_DIR"

# Helper: wait for any file matching a glob in a directory to exist and be non-empty
wait_for_glob() {
  local dir="$1"; local pattern="$2"; local timeout=${3:-180}; local waited=0
  echo "[analyze] Waiting for profiles in ${dir}/${pattern} (timeout ${timeout}s)..." >&2
  while [[ $waited -lt $timeout ]]; do
    # shellcheck disable=SC2086
    local f
    f=$(ls -1 ${dir}/${pattern} 2>/dev/null | head -n1 || true)
    if [[ -n "${f}" && -s "${f}" ]]; then
      echo "[analyze] Found: ${f}" >&2
      echo -n "${f}"
      return 0
    fi
    sleep 3; waited=$((waited+3))
  done
  echo "" # return empty string
  return 1
}

CPU_FILE=$(wait_for_glob "$CPU_DIR" "cpu*.pprof" 180 || true)
MEM_FILE=$(wait_for_glob "$MEM_DIR" "*.pprof" 180 || true)
ALLOC_FILE=$(wait_for_glob "$ALLOC_DIR" "*.pprof" 180 || true)

if [[ -z "$CPU_FILE" ]]; then echo "[analyze] WARNING: CPU profile not found at $CPU_DIR" >&2; fi
if [[ -z "$MEM_FILE" ]]; then echo "[analyze] WARNING: Mem profile not found at $MEM_DIR" >&2; fi
if [[ -z "$ALLOC_FILE" ]]; then echo "[analyze] WARNING: Alloc profile not found at $ALLOC_DIR" >&2; fi

{
  echo "==== next.orly.dev Profiling Analysis ===="
  date
  echo

  if [[ -n "$CPU_FILE" && -s "$CPU_FILE" ]]; then
    echo "-- CPU Hotspots (top by flat CPU) --"
    go tool pprof -top -nodecount=15 "$BIN" "$CPU_FILE" 2>/dev/null | sed '1,2d'
    echo
  else
    echo "CPU profile: not available"
    echo
  fi

  if [[ -n "$MEM_FILE" && -s "$MEM_FILE" ]]; then
    echo "-- Memory (In-Use Space) Hotspots --"
    go tool pprof -top -sample_index=inuse_space -nodecount=15 "$BIN" "$MEM_FILE" 2>/dev/null | sed '1,2d'
    echo
  else
    echo "Memory (in-use) profile: not available"
    echo
  fi

  if [[ -n "$ALLOC_FILE" && -s "$ALLOC_FILE" ]]; then
    echo "-- Allocations (Total Alloc Space) Hotspots --"
    go tool pprof -top -sample_index=alloc_space -nodecount=15 "$BIN" "$ALLOC_FILE" 2>/dev/null | sed '1,2d'
    echo
    echo "-- Allocation Frequency (Alloc Objects) --"
    go tool pprof -top -sample_index=alloc_objects -nodecount=15 "$BIN" "$ALLOC_FILE" 2>/dev/null | sed '1,2d'
    echo
  else
    echo "Allocation profile: not available"
    echo
  fi

  echo "Notes:"
  echo "- CPU section identifies functions using the most CPU time."
  echo "- Memory section identifies which functions retain the most memory (in-use)."
  echo "- Allocations sections identify functions responsible for the most allocation volume and count, which correlates with GC pressure."
  echo "- Profiles are created by github.com/pkg/profile and may only be flushed when the relay process receives a shutdown; CPU profile often requires process exit."
} > "$REPORT"

echo "[analyze] Wrote report to $REPORT" >&2
