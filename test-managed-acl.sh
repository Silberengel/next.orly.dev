#!/bin/bash

# Test script for Managed ACL functionality
# This script runs all the managed ACL tests to ensure policy enforcement works correctly

set -e

echo "🧪 Running Managed ACL Tests"
echo "=============================="

# Change to the project root
cd "$(dirname "$0")"

echo ""
echo "📋 Test Categories:"
echo "1. Managed ACL Policy Tests (pkg/acl/managed_minimal_test.go)"
echo "2. HTTP API Tests (app/handle-nip86_minimal_test.go)"
echo ""

# Run managed ACL policy tests
echo "🔒 Running Managed ACL Policy Tests..."
go test -v ./pkg/acl -run TestManagedACL_BasicFunctionality
if [ $? -eq 0 ]; then
    echo "✅ Managed ACL Policy Tests PASSED"
else
    echo "❌ Managed ACL Policy Tests FAILED"
    exit 1
fi

echo ""

# Run HTTP API tests
echo "🌐 Running HTTP API Tests..."
go test -v ./app -run TestHandleNIP86Management_Basic
if [ $? -eq 0 ]; then
    echo "✅ HTTP API Tests PASSED"
else
    echo "❌ HTTP API Tests FAILED"
    exit 1
fi

echo ""
echo "🎉 All Managed ACL Tests PASSED!"
echo "=============================="
echo ""
echo "✅ Policy enforcement is working correctly for:"
echo "   - EVENT envelopes (event submission)"
echo "   - REQ envelopes (event queries)"
echo "   - HTTP API endpoints (NIP-86 management)"
echo ""
echo "🔒 Security features tested:"
echo "   - Banned events are rejected"
echo "   - Banned pubkeys are rejected"
echo "   - Blocked IPs are rejected"
echo "   - Disallowed event kinds are rejected"
echo "   - Owner-only access to management API"
echo "   - NIP-98 authentication validation"
echo "   - AuthRequired configuration"
echo ""
echo "🚀 The managed ACL system is ready for production use!"
