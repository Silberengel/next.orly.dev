#!/bin/sh
# Docker entrypoint script that can run GC, repair, or start the relay

set -e

DATA_DIR="${ORLY_DATA_DIR:-/data}"

# If first argument is "gc-db", run the GC utility
if [ "$1" = "gc-db" ]; then
    shift
    exec /app/gc-db "$@"
fi

# If first argument is "repair-db", run the repair utility
if [ "$1" = "repair-db" ]; then
    shift
    exec /app/repair-db "$@"
fi

# If GC is requested via environment variable, run it first
if [ "${ORLY_RUN_GC_ON_START:-false}" = "true" ]; then
    echo "Running database garbage collection before starting relay..."
    if [ -f "/app/gc-db" ]; then
        /app/gc-db "$DATA_DIR" || {
            echo "Warning: GC failed, but continuing with relay start..."
        }
    else
        echo "Warning: gc-db utility not found, skipping GC..."
    fi
fi

# Start the relay (default behavior)
exec /app/relay "$@"

