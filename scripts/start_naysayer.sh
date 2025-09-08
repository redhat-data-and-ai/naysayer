#!/bin/bash

# Script to start naysayer with a new GitLab token
# Usage: ./start_naysayer.sh [GITLAB_TOKEN] [PORT]

set -e

# Default values
DEFAULT_PORT="3001"
DEFAULT_BASE_URL="https://gitlab.cee.redhat.com"
DEFAULT_WEBHOOK_SECRET="gR32t62UfsbmbTJ"

# Parse arguments
GITLAB_TOKEN="${1:-}"
PORT="${2:-$DEFAULT_PORT}"

if [ -z "$GITLAB_TOKEN" ]; then
    echo "❌ Error: GitLab token is required"
    echo ""
    echo "Usage: $0 <GITLAB_TOKEN> [PORT]"
    echo ""
    echo "Examples:"
    echo "  $0 \"your-new-gitlab-token\"           # Start on port 3001"
    echo "  $0 \"your-new-gitlab-token\" 3002      # Start on port 3002"
    echo ""
    echo "💡 To get a GitLab token:"
    echo "   1. Go to https://gitlab.cee.redhat.com/-/profile/personal_access_tokens"
    echo "   2. Create token with 'api' scope"
    echo "   3. Ensure you have Developer/Maintainer role on the project"
    exit 1
fi

echo "🚀 Starting Naysayer"
echo "==================="
echo "🔑 Token: ${GITLAB_TOKEN:0:8}..."
echo "🌐 GitLab: $DEFAULT_BASE_URL"
echo "🔌 Port: $PORT"
echo "🔐 Webhook Secret: ${DEFAULT_WEBHOOK_SECRET:0:8}..."
echo ""

# Stop any existing naysayer processes
echo "🛑 Stopping any existing naysayer processes..."
pkill -f "go run.*main.go" || true
pkill -f "naysayer" || true
sleep 2

# Navigate to project root
cd "$(dirname "$0")/.."

# Export environment variables
export GITLAB_TOKEN="$GITLAB_TOKEN"
export GITLAB_BASE_URL="$DEFAULT_BASE_URL"
export PORT="$PORT"
export WEBHOOK_SECRET="$DEFAULT_WEBHOOK_SECRET"
export LOG_LEVEL="info"
export ENABLE_MR_COMMENTS="true"
export UPDATE_EXISTING_COMMENTS="true"

echo "🔧 Environment configured"
echo "📁 Working directory: $(pwd)"
echo ""

# Start naysayer in background
echo "🚀 Starting naysayer in background..."
nohup go run cmd/main.go > naysayer.log 2>&1 &
NAYSAYER_PID=$!

echo "✅ Naysayer started with PID: $NAYSAYER_PID"
echo "📋 Log file: $(pwd)/naysayer.log"

# Wait a moment and check if it's running
sleep 3

if ps -p $NAYSAYER_PID > /dev/null; then
    echo "✅ Naysayer is running successfully"
    
    # Test health endpoint
    echo "🔍 Testing health endpoint..."
    if curl -s "http://localhost:$PORT/health" > /dev/null; then
        echo "✅ Health endpoint responding"
        echo ""
        echo "🎉 Naysayer is ready!"
        echo ""
        echo "💡 Useful commands:"
        echo "   curl http://localhost:$PORT/health     # Check health"
        echo "   tail -f naysayer.log                   # View logs"
        echo "   kill $NAYSAYER_PID                     # Stop naysayer"
        echo "   ./scripts/test_mr.sh 1764              # Test with MR 1764"
    else
        echo "⚠️  Health endpoint not responding yet (may need more time)"
    fi
else
    echo "❌ Naysayer failed to start"
    echo "📋 Check the log file: $(pwd)/naysayer.log"
    exit 1
fi

echo ""
echo "🔗 Test URL: http://localhost:$PORT"
echo "📊 Health: http://localhost:$PORT/health"
echo "📝 Logs: tail -f $(pwd)/naysayer.log"
