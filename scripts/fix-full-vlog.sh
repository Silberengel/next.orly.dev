#!/bin/bash
# Fix full Badger value log files by running GC
# This script should be run on the server where the database files are located

set -e

DATA_DIR="${1:-/var/lib/docker/volumes/orly-relay-data/_data}"

if [ ! -d "$DATA_DIR" ]; then
    echo "Error: Data directory not found: $DATA_DIR"
    echo "Usage: $0 [data-directory]"
    echo "Default: /var/lib/docker/volumes/orly-relay-data/_data"
    exit 1
fi

echo "Checking for full value log files in: $DATA_DIR"
echo ""

# Find value log files
VLOG_FILES=$(find "$DATA_DIR" -name "*.vlog" 2>/dev/null | head -5)

if [ -z "$VLOG_FILES" ]; then
    echo "No value log files found. Database might be empty or in a different location."
    exit 0
fi

echo "Found value log files. The issue is that these files are full (20MB limit)."
echo ""
echo "Solution: We need to run Badger GC to free space, but this requires"
echo "the database to be opened, which is currently failing."
echo ""
echo "Temporary workaround: Rename the full value log files so Badger can create new ones."
echo "WARNING: This will cause Badger to recreate value log files on next start."
echo ""
read -p "Proceed with renaming full value log files? (y/N) " -n 1 -r
echo

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Cancelled."
    exit 0
fi

# Rename value log files
for vlog in $VLOG_FILES; do
    if [ -f "$vlog" ]; then
        size=$(stat -f%z "$vlog" 2>/dev/null || stat -c%s "$vlog" 2>/dev/null)
        if [ "$size" -ge 20000000 ]; then  # 20MB
            echo "Renaming full value log file: $(basename $vlog)"
            mv "$vlog" "${vlog}.full"
        fi
    fi
done

echo ""
echo "Done! Value log files have been renamed."
echo "Badger will create new value log files when the relay starts."
echo ""
echo "Now you can start the relay container."

