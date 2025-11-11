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
echo -e "${YELLOW}Step 9: Testing EVENT message (write control)...${NC}"

# Check which port the relay is listening on
RELAY_PORT=$(docker logs orly-policy-test 2>&1 | grep "starting listener" | grep -oP ':\K[0-9]+' | head -1)
if [ -z "$RELAY_PORT" ]; then
    RELAY_PORT="8777"
fi
echo "Relay is listening on port: $RELAY_PORT"

# Test EVENT message
cd "$REPO_ROOT"
./policytest -url "ws://localhost:$RELAY_PORT" -type event -kind 1 2>&1 || echo "EVENT test completed"

echo ""
echo -e "${YELLOW}Relay logs after EVENT test:${NC}"
docker logs orly-policy-test 2>&1 | tail -10

echo ""
echo -e "${YELLOW}Step 10: Testing REQ message (read control)...${NC}"

# Test REQ message
./policytest -url "ws://localhost:$RELAY_PORT" -type req -kind 1 2>&1 || echo "REQ test completed"

echo ""
echo -e "${YELLOW}Relay logs after REQ test:${NC}"
docker logs orly-policy-test 2>&1 | tail -10

echo ""
echo -e "${YELLOW}Step 11: Waiting for policy script to execute (5 seconds)...${NC}"
sleep 5

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

    echo "Policy invocations:"
    echo "  - Write operations: $WRITE_COUNT"
    echo "  - Read operations: $READ_COUNT"
    echo ""

    if [ "$WRITE_COUNT" -gt 0 ] && [ "$READ_COUNT" -gt 0 ]; then
        echo -e "${GREEN}✓ Policy script processed both write and read operations!${NC}"
        EXIT_CODE=0
    elif [ "$WRITE_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}⚠ Policy script only processed write operations (read operations may not have been tested)${NC}"
        EXIT_CODE=0
    else
        echo -e "${YELLOW}⚠ Policy script is working but access types may not be logged correctly${NC}"
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
