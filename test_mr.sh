#!/bin/bash

# Naysayer Real MR Testing Script
# This script helps you test the naysayer endpoint with real GitLab MRs

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Naysayer Real MR Testing Tool${NC}"
echo "====================================="

# Check if GitLab token is set
if [[ -z "${GITLAB_TOKEN}" ]]; then
    echo -e "${RED}❌ Error: GITLAB_TOKEN environment variable is required${NC}"
    echo "Please export your GitLab token:"
    echo "  export GITLAB_TOKEN='your_token_here'"
    exit 1
fi

# Default values
NAYSAYER_URL=${NAYSAYER_URL:-"http://localhost:3000"}
GITLAB_BASE_URL=${GITLAB_BASE_URL:-"https://gitlab.cee.redhat.com"}
WRITE_COMMENTS=${WRITE_COMMENTS:-"true"}

echo -e "${BLUE}Configuration:${NC}"
echo "  🔗 Naysayer URL: $NAYSAYER_URL"
echo "  🔗 GitLab URL: $GITLAB_BASE_URL"
echo "  🔑 GitLab Token: ${GITLAB_TOKEN:0:8}..."
echo "  💬 Write Comments: $WRITE_COMMENTS"
echo ""

# Function to test a specific MR
test_mr() {
    local project_id=$1
    local mr_iid=$2
    
    echo -e "${YELLOW}🎯 Testing Project $project_id, MR !$mr_iid${NC}"
    
    # Check if naysayer is running
    if ! curl -s "$NAYSAYER_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}❌ Error: Naysayer service is not running at $NAYSAYER_URL${NC}"
        echo "Please start the naysayer service first:"
        echo "  go run cmd/main.go"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Naysayer service is running${NC}"
    
    # Run the test
    go run test_real_mr.go "$project_id" "$mr_iid"
}

# Function to test with URL
test_mr_url() {
    local mr_url=$1
    
    echo -e "${YELLOW}🎯 Testing MR from URL: $mr_url${NC}"
    
    # Check if naysayer is running
    if ! curl -s "$NAYSAYER_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}❌ Error: Naysayer service is not running at $NAYSAYER_URL${NC}"
        echo "Please start the naysayer service first:"
        echo "  go run cmd/main.go"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Naysayer service is running${NC}"
    
    # Run the test
    echo -e "${BLUE}🔍 Running test with URL parsing...${NC}"
    go run test_real_mr.go "$mr_url"
}

# Function to start naysayer in background
start_naysayer() {
    echo -e "${YELLOW}🚀 Starting naysayer service...${NC}"
    
    # Check if already running
    if curl -s "$NAYSAYER_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Naysayer is already running${NC}"
        return 0
    fi
    
    # Start in background
    nohup go run cmd/main.go > naysayer.log 2>&1 &
    local pid=$!
    echo "Started naysayer with PID: $pid"
    
    # Wait for it to start
    echo "Waiting for naysayer to start..."
    for i in {1..30}; do
        if curl -s "$NAYSAYER_URL/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Naysayer started successfully${NC}"
            return 0
        fi
        sleep 1
    done
    
    echo -e "${RED}❌ Failed to start naysayer${NC}"
    exit 1
}

# Function to show usage
show_usage() {
    echo "Usage:"
    echo "  $0 test <project_id> <mr_iid>     # Test specific MR by ID"
    echo "  $0 test <gitlab_mr_url>           # Test MR by URL"
    echo "  $0 start                          # Start naysayer service"
    echo "  $0 help                           # Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 test 51 1764"
    echo "  $0 test https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/1764"
    echo ""
    echo "Environment variables:"
    echo "  GITLAB_TOKEN    - GitLab API token (required)"
    echo "  NAYSAYER_URL    - Naysayer service URL (default: http://localhost:3000)"
    echo "  GITLAB_BASE_URL - GitLab base URL (default: https://gitlab.cee.redhat.com)"
    echo "  WRITE_COMMENTS  - Write test results as MR comments (default: true)"
}

# Main script logic
case "${1:-}" in
    "test")
        if [[ $# -eq 3 ]]; then
            # Project ID and MR IID provided
            test_mr "$2" "$3"
        elif [[ $# -eq 2 ]]; then
            # URL provided
            test_mr_url "$2"
        else
            echo -e "${RED}❌ Error: Invalid arguments for test command${NC}"
            show_usage
            exit 1
        fi
        ;;
    "start")
        start_naysayer
        ;;
    "help"|"-h"|"--help")
        show_usage
        ;;
    *)
        echo -e "${RED}❌ Error: Unknown command${NC}"
        show_usage
        exit 1
        ;;
esac
