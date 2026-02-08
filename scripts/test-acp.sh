#!/bin/bash
# Test script for ACP (Agent Connection Protocol) communication with OpenCode
# This script demonstrates how to interact with an OpenCode agent via HTTP API

set -e

AGENT_URL="${AGENT_URL:-http://localhost:3100}"
SESSION_ID=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[ACP Test]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if agent is reachable
check_agent() {
    log "Checking if agent is reachable at $AGENT_URL..."
    
    if curl -s --fail "$AGENT_URL/agent" > /dev/null 2>&1; then
        success "Agent is reachable"
        return 0
    else
        error "Cannot reach agent at $AGENT_URL"
        return 1
    fi
}

# List available agents
list_agents() {
    log "Listing available agents..."
    
    local agents=$(curl -s "$AGENT_URL/agent" | jq -r '.[].name' 2>/dev/null)
    if [ -n "$agents" ]; then
        echo -e "${YELLOW}Available agents:${NC}"
        echo "$agents" | while read agent; do
            echo "  - $agent"
        done
    else
        error "Failed to list agents"
        return 1
    fi
}

# Create a new session
create_session() {
    log "Creating new session..."
    
    local response=$(curl -s -X POST "$AGENT_URL/session" \
        -H "Content-Type: application/json" \
        -d '{}')
    
    SESSION_ID=$(echo "$response" | jq -r '.id' 2>/dev/null)
    
    if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
        success "Created session: $SESSION_ID"
        local title=$(echo "$response" | jq -r '.title' 2>/dev/null)
        echo "  Title: $title"
    else
        error "Failed to create session"
        echo "$response"
        return 1
    fi
}

# Send a message and get response
send_message() {
    local message="$1"
    
    if [ -z "$SESSION_ID" ]; then
        error "No session ID. Create a session first."
        return 1
    fi
    
    log "Sending message: \"$message\""
    
    local payload=$(jq -n --arg text "$message" '{
        parts: [{type: "text", text: $text}]
    }')
    
    local response=$(curl -s -X POST "$AGENT_URL/session/$SESSION_ID/message" \
        -H "Content-Type: application/json" \
        -d "$payload")
    
    # Extract text response
    local text_response=$(echo "$response" | jq -r '.parts[] | select(.type == "text") | .text' 2>/dev/null)
    
    if [ -n "$text_response" ]; then
        success "Agent response:"
        echo -e "${YELLOW}$text_response${NC}"
        
        # Show token usage
        local input_tokens=$(echo "$response" | jq -r '.info.tokens.input // 0' 2>/dev/null)
        local output_tokens=$(echo "$response" | jq -r '.info.tokens.output // 0' 2>/dev/null)
        echo -e "\n${BLUE}Tokens: input=$input_tokens, output=$output_tokens${NC}"
    else
        error "Failed to get response or no text in response"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# Get session info
get_session_info() {
    if [ -z "$SESSION_ID" ]; then
        error "No session ID"
        return 1
    fi
    
    log "Getting session info for $SESSION_ID..."
    
    curl -s "$AGENT_URL/session" | jq ".[] | select(.id == \"$SESSION_ID\")" 2>/dev/null
}

# Main test flow
main() {
    echo "=========================================="
    echo "  ACP (Agent Connection Protocol) Test"
    echo "=========================================="
    echo ""
    
    # Step 1: Check agent
    if ! check_agent; then
        exit 1
    fi
    echo ""
    
    # Step 2: List agents
    list_agents
    echo ""
    
    # Step 3: Create session
    if ! create_session; then
        exit 1
    fi
    echo ""
    
    # Step 4: Send test messages
    echo "=========================================="
    echo "  Sending Test Messages"
    echo "=========================================="
    echo ""
    
    # Simple math test
    send_message "What is 15 * 7? Reply with just the number, nothing else."
    echo ""
    echo "---"
    echo ""
    
    # Another test
    send_message "What is the current date? Reply in YYYY-MM-DD format only."
    echo ""
    echo "---"
    echo ""
    
    # Test with instruction
    send_message "List 3 random colors, one per line. No other text."
    echo ""
    
    echo "=========================================="
    echo "  Test Complete"
    echo "=========================================="
    echo ""
    echo "Session ID: $SESSION_ID"
    echo "You can continue this session by setting SESSION_ID=$SESSION_ID"
}

# Run main or individual commands
case "${1:-}" in
    check)
        check_agent
        ;;
    agents)
        list_agents
        ;;
    session)
        create_session
        ;;
    send)
        SESSION_ID="$2"
        shift 2
        send_message "$*"
        ;;
    info)
        SESSION_ID="$2"
        get_session_info
        ;;
    *)
        main
        ;;
esac
