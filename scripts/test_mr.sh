#!/bin/bash

# Simple script to test naysayer on a specific MR
# Usage: ./test_mr.sh [MR_IID] [ACTION] [GITLAB_TOKEN]

set -e  # Exit on any error

# Default values
DEFAULT_MR_IID="1764"
DEFAULT_ACTION="update"
DEFAULT_NAYSAYER_URL="http://localhost:3001"

# Parse arguments
MR_IID="${1:-$DEFAULT_MR_IID}"
ACTION="${2:-$DEFAULT_ACTION}"
GITLAB_TOKEN="${3:-}"
NAYSAYER_URL="${4:-$DEFAULT_NAYSAYER_URL}"

echo "🧪 Simple Naysayer MR Test"
echo "=========================="
echo "📄 MR IID: $MR_IID"
echo "⚡ Action: $ACTION"
echo "🔗 Naysayer: $NAYSAYER_URL"

if [ -n "$GITLAB_TOKEN" ]; then
    echo "🔑 Using provided GitLab token: ${GITLAB_TOKEN:0:8}..."
    export GITLAB_TOKEN="$GITLAB_TOKEN"
else
    echo "🔑 Using token from environment/config"
fi

echo ""

# Check if naysayer is running
echo "🔍 Checking if naysayer is running..."
if curl -s "$NAYSAYER_URL/health" > /dev/null 2>&1; then
    echo "✅ Naysayer is running"
else
    echo "❌ Naysayer is not running at $NAYSAYER_URL"
    echo ""
    echo "💡 To start naysayer:"
    echo "   cd /Users/isequeir/go/src/github.com/naysayer"
    if [ -n "$GITLAB_TOKEN" ]; then
        echo "   GITLAB_TOKEN=\"$GITLAB_TOKEN\" go run cmd/main.go &"
    else
        echo "   go run cmd/main.go &"
    fi
    exit 1
fi

echo ""
echo "🚀 Sending test webhook to naysayer..."

# Run the webhook test
cd "$(dirname "$0")"
if [ -n "$GITLAB_TOKEN" ]; then
    # If token provided, temporarily export it for the test
    ORIGINAL_TOKEN="${GITLAB_TOKEN:-}"
    export GITLAB_TOKEN="$GITLAB_TOKEN"
    go run test_webhook.go "$MR_IID" "$ACTION" "$NAYSAYER_URL"
    if [ -n "$ORIGINAL_TOKEN" ]; then
        export GITLAB_TOKEN="$ORIGINAL_TOKEN"
    else
        unset GITLAB_TOKEN
    fi
else
    go run test_webhook.go "$MR_IID" "$ACTION" "$NAYSAYER_URL"
fi

echo ""
echo "✅ Test completed!"
echo "🔗 Check the MR: https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/$MR_IID"
echo ""
echo "💡 Usage examples:"
echo "   ./test_mr.sh                           # Test MR 1764 with update action"
echo "   ./test_mr.sh 1234                      # Test MR 1234 with update action"
echo "   ./test_mr.sh 1234 open                 # Test MR 1234 with open action"
echo "   ./test_mr.sh 1234 update \"new-token\"   # Test with specific token"
