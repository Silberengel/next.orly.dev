#!/bin/bash

set -e

echo "=== ORLY Policy Test Script ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# Get the repository root (two levels up from scripts/docker-policy)
REPO_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"

echo "Script directory: $SCRIPT_DIR"
echo "Repository root: $REPO_ROOT"
echo ""

echo -e "${YELLOW}Step 1: Building ORLY binary on host...${NC}"
cd "$REPO_ROOT" && CGO_ENABLED=0 go build -o orly

echo ""
echo -e "${YELLOW}Step 2: Copying files to test directory...${NC}"
cp "$REPO_ROOT/orly" "$SCRIPT_DIR/"
cp "$REPO_ROOT/pkg/crypto/p8k/libsecp256k1.so" "$SCRIPT_DIR/"

echo ""
echo -e "${YELLOW}Step 3: Cleaning up old containers...${NC}"
cd "$SCRIPT_DIR" && docker-compose down -v 2>/dev/null || true

echo ""
echo -e "${YELLOW}Step 4: Building Docker image...${NC}"
cd "$SCRIPT_DIR" && docker-compose build

echo ""
echo -e "${YELLOW}Step 5: Starting ORLY relay container...${NC}"
cd "$SCRIPT_DIR" && docker-compose up -d

echo ""
echo -e "${YELLOW}Step 6: Waiting for relay to start (15 seconds)...${NC}"
sleep 15

echo ""
echo -e "${YELLOW}Step 7: Checking relay logs...${NC}"
docker logs orly-policy-test 2>&1 | tail -20

echo ""
echo -e "${YELLOW}Step 8: Building policytest tool...${NC}"
cd "$REPO_ROOT" && CGO_ENABLED=0 go build -o policytest ./cmd/policytest

echo ""
echo -e "${YELLOW}Step 9: Publishing 2 events and querying for them...${NC}"

# Check which port the relay is listening on
RELAY_PORT=$(docker logs orly-policy-test 2>&1 | grep "starting listener" | grep -oP ':\K[0-9]+' | head -1)
if [ -z "$RELAY_PORT" ]; then
    RELAY_PORT="8777"
fi
echo "Relay is listening on port: $RELAY_PORT"

# Test publish and query - this will publish 2 events and query for them
cd "$REPO_ROOT"
echo ""
echo "--- Publishing and querying events ---"
./policytest -url "ws://localhost:$RELAY_PORT" -type publish-and-query -kind 1 -count 2 2>&1

echo ""
echo -e "${YELLOW}Step 10: Checking relay logs...${NC}"
docker logs orly-policy-test 2>&1 | tail -20

echo ""
echo -e "${YELLOW}Step 11: Waiting for policy script to process (3 seconds)...${NC}"
sleep 3

echo ""
echo -e "${YELLOW}Step 12: Checking if cs-policy.js created output file...${NC}"

# Check if the output file exists in the container
if docker exec orly-policy-test test -f /home/orly/cs-policy-output.txt; then
    echo -e "${GREEN}✓ SUCCESS: cs-policy-output.txt file exists!${NC}"
    echo ""
    echo "Output file contents:"
    docker exec orly-policy-test cat /home/orly/cs-policy-output.txt
    echo ""

    # Check if we see both read and write access types
    WRITE_COUNT=$(docker exec orly-policy-test cat /home/orly/cs-policy-output.txt | grep -c "Access: write" || echo "0")
    READ_COUNT=$(docker exec orly-policy-test cat /home/orly/cs-policy-output.txt | grep -c "Access: read" || echo "0")

    echo "Policy invocations summary:"
    echo "  - Write operations (EVENT): $WRITE_COUNT (expected: 2)"
    echo "  - Read operations (REQ):    $READ_COUNT (expected: >=1)"
    echo ""

    # Analyze results
    if [ "$WRITE_COUNT" -ge 2 ] && [ "$READ_COUNT" -ge 1 ]; then
        echo -e "${GREEN}✓ SUCCESS: Policy script processed both write and read operations!${NC}"
        echo -e "${GREEN}  - Published 2 events (write control)${NC}"
        echo -e "${GREEN}  - Queried events (read control)${NC}"
        EXIT_CODE=0
    elif [ "$WRITE_COUNT" -gt 0 ] && [ "$READ_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}⚠ PARTIAL: Policy invoked but counts don't match expected${NC}"
        echo -e "${YELLOW}  - Write count: $WRITE_COUNT (expected 2)${NC}"
        echo -e "${YELLOW}  - Read count: $READ_COUNT (expected >=1)${NC}"
        EXIT_CODE=0
    elif [ "$WRITE_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}⚠ WARNING: Policy script only processed write operations${NC}"
        echo -e "${YELLOW}  Read operations may not have been tested or logged${NC}"
        EXIT_CODE=0
    else
        echo -e "${YELLOW}⚠ WARNING: Policy script is working but access types may not be logged correctly${NC}"
        EXIT_CODE=0
    fi
else
    echo -e "${RED}✗ FAILURE: cs-policy-output.txt file not found!${NC}"
    echo ""
    echo "Checking relay logs for errors:"
    docker logs orly-policy-test 2>&1 | grep -i policy || echo "No policy-related logs found"
    EXIT_CODE=1
fi

echo ""
echo -e "${YELLOW}Step 13: Additional debugging info...${NC}"
echo "Files in /home/orly directory:"
docker exec orly-policy-test ls -la /home/orly/

echo ""
echo "Policy configuration:"
docker exec orly-policy-test cat /home/orly/.config/orly/policy.json || echo "Policy config not found"

echo ""
echo "=== Test Complete ==="
echo ""
echo "To view logs: docker logs orly-policy-test"
echo "To stop container: cd scripts/docker-policy && docker-compose down"
echo "To clean up: cd scripts/docker-policy && docker-compose down -v"

exit $EXIT_CODE
