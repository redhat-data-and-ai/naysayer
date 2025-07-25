#!/bin/bash

# Simple test script for any MR from dataverse/dataverse-config/dataproduct-config
# Usage: ./test_mr.sh [MR_NUMBER]

MR_NUMBER=${1:-1502}
PROJECT_ID=106670

echo "🧪 Testing self-service bot for MR ${MR_NUMBER}"
echo "================================================="

# Check if server is running
if ! curl -s http://localhost:3000/health > /dev/null; then
    echo "❌ Server not running. Start with './naysayer' first."
    exit 1
fi

echo "✅ Server is running"

# Create simple webhook payload
cat > /tmp/mr_test.json << EOF
{
  "object_kind": "merge_request",
  "object_attributes": {
    "iid": ${MR_NUMBER},
    "title": "Test MR ${MR_NUMBER}",
    "state": "opened",
    "action": "open",
    "source_branch": "test-branch",
    "target_branch": "main"
  },
  "project": {
    "id": ${PROJECT_ID},
    "name": "dataproduct-config",
    "web_url": "https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config"
  }
}
EOF

echo ""
echo "📤 Sending webhook for MR ${MR_NUMBER}..."

# Send webhook and show response
response=$(curl -s -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d @/tmp/mr_test.json)

echo "📥 Bot Response:"
echo "$response"

# Clean up
rm -f /tmp/mr_test.json

echo ""
echo "🏁 Test completed!"
echo ""
echo "Usage: ./test_mr.sh [MR_NUMBER]"