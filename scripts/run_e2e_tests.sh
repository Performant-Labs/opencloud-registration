#!/usr/bin/env bash
# run_e2e_tests.sh
set -e

echo "🚀 Running Pre-Flight Checks for OpenCloud Registration E2E Tests..."

# 1. Dependency Check
if ! command -v go &> /dev/null; then
    echo "❌ Error: 'go' command could not be found. Please install Go."
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "❌ Error: 'curl' command could not be found. Please install curl."
    exit 1
fi

echo "✅ Dependencies (go, curl) installed."

# 2. Container/Stack Check
echo "🔍 Checking if OpenCloud stack is running..."

# Wait up to 5 seconds to get a response from cloud.opencloud.test
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -k --connect-timeout 5 https://cloud.opencloud.test || echo "FAILED")

if [ "$HTTP_STATUS" = "FAILED" ] || [ "$HTTP_STATUS" = "000" ]; then
    echo "❌ Error: Cannot connect to https://cloud.opencloud.test."
    echo "Are you sure pl-opencloud-server is running?"
    echo "Please run: cd ~/Sites/pl-opencloud-server && ./occtl start"
    exit 1
fi
echo "✅ https://cloud.opencloud.test is online (HTTP $HTTP_STATUS)."

echo "🔍 Checking Registration App health..."
REG_HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -k --connect-timeout 5 https://register.opencloud.test/health || echo "FAILED")

if [ "$REG_HTTP_STATUS" != "200" ]; then
    echo "❌ Error: Registration healthcheck failed (HTTP $REG_HTTP_STATUS at https://register.opencloud.test/health)."
    echo "The registration app may not be running or the proxy is down."
    exit 1
fi
echo "✅ https://register.opencloud.test/health is responding (HTTP 200)."

# 3. Execution
echo "🎉 All pre-flight checks passed! Starting E2E tests..."
echo "--------------------------------------------------------"

# Assume script is run from project root, otherwise adjust path
cd "$(dirname "$0")/.."

go test ./e2e/ -v -timeout 60s
