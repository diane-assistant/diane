#!/bin/bash
# Test script for ACP (Agent Communication Protocol) - The Real Standard
# Based on the ACP spec: https://agentcommunicationprotocol.dev
# OpenAPI spec: https://github.com/i-am-bee/acp/blob/main/docs/spec/openapi.yaml

set -e

ACP_URL="${ACP_URL:-http://localhost:8000}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log() { echo -e "${BLUE}[ACP]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
info() { echo -e "${CYAN}[INFO]${NC} $1"; }

# ============================================================================
# ACP API Functions (per spec)
# ============================================================================

# GET /ping - Health check
acp_ping() {
    log "Pinging ACP server at $ACP_URL..."
    
    local response=$(curl -s -w "\n%{http_code}" "$ACP_URL/ping" 2>/dev/null)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "200" ]; then
        success "Server is reachable"
        return 0
    else
        error "Server not reachable (HTTP $http_code)"
        return 1
    fi
}

# GET /agents - List available agents
acp_list_agents() {
    local limit="${1:-10}"
    local offset="${2:-0}"
    
    log "Listing agents (limit=$limit, offset=$offset)..."
    
    local response=$(curl -s "$ACP_URL/agents?limit=$limit&offset=$offset" 2>/dev/null)
    
    if echo "$response" | jq -e '.agents' > /dev/null 2>&1; then
        local count=$(echo "$response" | jq '.agents | length')
        success "Found $count agents"
        
        echo -e "\n${YELLOW}Available Agents:${NC}"
        echo "$response" | jq -r '.agents[] | "  \(.name): \(.description)"'
        return 0
    else
        error "Failed to list agents"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# GET /agents/{name} - Get agent manifest
acp_get_agent() {
    local name="$1"
    
    if [ -z "$name" ]; then
        error "Agent name required"
        return 1
    fi
    
    log "Getting manifest for agent: $name"
    
    local response=$(curl -s "$ACP_URL/agents/$name" 2>/dev/null)
    
    if echo "$response" | jq -e '.name' > /dev/null 2>&1; then
        success "Got agent manifest"
        echo "$response" | jq .
        return 0
    else
        error "Failed to get agent"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# POST /runs - Create a new run (sync mode)
acp_run_sync() {
    local agent_name="$1"
    local prompt="$2"
    
    if [ -z "$agent_name" ] || [ -z "$prompt" ]; then
        error "Usage: acp_run_sync <agent_name> <prompt>"
        return 1
    fi
    
    log "Creating sync run for agent: $agent_name"
    info "Prompt: $prompt"
    
    local payload=$(jq -n \
        --arg agent "$agent_name" \
        --arg text "$prompt" \
        '{
            agent_name: $agent,
            mode: "sync",
            input: [{
                role: "user",
                parts: [{
                    content_type: "text/plain",
                    content: $text
                }]
            }]
        }')
    
    local response=$(curl -s -X POST "$ACP_URL/runs" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null)
    
    if echo "$response" | jq -e '.run_id' > /dev/null 2>&1; then
        local run_id=$(echo "$response" | jq -r '.run_id')
        local status=$(echo "$response" | jq -r '.status')
        
        success "Run created: $run_id (status: $status)"
        
        # Extract text output
        local output=$(echo "$response" | jq -r '.output[]?.parts[]? | select(.content_type == "text/plain" or .content_type == null) | .content' 2>/dev/null | head -1)
        
        if [ -n "$output" ]; then
            echo -e "\n${YELLOW}Agent Response:${NC}"
            echo "$output"
        fi
        
        echo -e "\n${CYAN}Full Response:${NC}"
        echo "$response" | jq .
        return 0
    else
        error "Failed to create run"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# POST /runs - Create a new run (async mode)
acp_run_async() {
    local agent_name="$1"
    local prompt="$2"
    
    if [ -z "$agent_name" ] || [ -z "$prompt" ]; then
        error "Usage: acp_run_async <agent_name> <prompt>"
        return 1
    fi
    
    log "Creating async run for agent: $agent_name"
    
    local payload=$(jq -n \
        --arg agent "$agent_name" \
        --arg text "$prompt" \
        '{
            agent_name: $agent,
            mode: "async",
            input: [{
                role: "user",
                parts: [{
                    content_type: "text/plain",
                    content: $text
                }]
            }]
        }')
    
    local response=$(curl -s -X POST "$ACP_URL/runs" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null)
    
    if echo "$response" | jq -e '.run_id' > /dev/null 2>&1; then
        local run_id=$(echo "$response" | jq -r '.run_id')
        success "Run created: $run_id"
        echo "$run_id"
        return 0
    else
        error "Failed to create run"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# GET /runs/{run_id} - Get run status
acp_get_run() {
    local run_id="$1"
    
    if [ -z "$run_id" ]; then
        error "Run ID required"
        return 1
    fi
    
    log "Getting run status: $run_id"
    
    local response=$(curl -s "$ACP_URL/runs/$run_id" 2>/dev/null)
    
    if echo "$response" | jq -e '.run_id' > /dev/null 2>&1; then
        local status=$(echo "$response" | jq -r '.status')
        success "Run status: $status"
        echo "$response" | jq .
        return 0
    else
        error "Failed to get run"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# POST /runs/{run_id}/cancel - Cancel a run
acp_cancel_run() {
    local run_id="$1"
    
    if [ -z "$run_id" ]; then
        error "Run ID required"
        return 1
    fi
    
    log "Cancelling run: $run_id"
    
    local response=$(curl -s -X POST "$ACP_URL/runs/$run_id/cancel" 2>/dev/null)
    
    if echo "$response" | jq -e '.run_id' > /dev/null 2>&1; then
        success "Run cancelled"
        return 0
    else
        error "Failed to cancel run"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# GET /runs/{run_id}/events - Get run events
acp_get_run_events() {
    local run_id="$1"
    
    if [ -z "$run_id" ]; then
        error "Run ID required"
        return 1
    fi
    
    log "Getting run events: $run_id"
    
    local response=$(curl -s "$ACP_URL/runs/$run_id/events" 2>/dev/null)
    echo "$response" | jq .
}

# ============================================================================
# Test Suite
# ============================================================================

run_tests() {
    echo "=========================================="
    echo "  ACP (Agent Communication Protocol)"
    echo "  Standard Test Suite"
    echo "=========================================="
    echo ""
    echo "Server: $ACP_URL"
    echo "Spec: https://agentcommunicationprotocol.dev"
    echo ""
    
    # Test 1: Ping
    echo "--- Test 1: Ping ---"
    if ! acp_ping; then
        error "Server not available. Make sure an ACP server is running."
        echo ""
        echo "To run a test ACP server, you can use BeeAI platform:"
        echo "  pip install beeai-framework"
        echo "  beeai serve"
        echo ""
        echo "Or set ACP_URL to point to your ACP server:"
        echo "  ACP_URL=http://your-server:8000 $0"
        exit 1
    fi
    echo ""
    
    # Test 2: List agents
    echo "--- Test 2: List Agents ---"
    acp_list_agents
    echo ""
    
    # If agents are available, run more tests
    local first_agent=$(curl -s "$ACP_URL/agents?limit=1" 2>/dev/null | jq -r '.agents[0].name // empty')
    
    if [ -n "$first_agent" ]; then
        # Test 3: Get agent manifest
        echo "--- Test 3: Get Agent Manifest ---"
        acp_get_agent "$first_agent"
        echo ""
        
        # Test 4: Run sync
        echo "--- Test 4: Sync Run ---"
        acp_run_sync "$first_agent" "What is 2 + 2? Reply with just the number."
        echo ""
    else
        info "No agents available for run tests"
    fi
    
    echo "=========================================="
    echo "  Tests Complete"
    echo "=========================================="
}

# ============================================================================
# CLI
# ============================================================================

show_help() {
    echo "ACP Test Client - Agent Communication Protocol"
    echo ""
    echo "Usage: $0 [command] [args...]"
    echo ""
    echo "Commands:"
    echo "  test                    Run full test suite"
    echo "  ping                    Check if server is reachable"
    echo "  agents                  List available agents"
    echo "  agent <name>            Get agent manifest"
    echo "  run <agent> <prompt>    Run agent synchronously"
    echo "  async <agent> <prompt>  Run agent asynchronously"
    echo "  status <run_id>         Get run status"
    echo "  cancel <run_id>         Cancel a run"
    echo "  events <run_id>         Get run events"
    echo ""
    echo "Environment:"
    echo "  ACP_URL                 Server URL (default: http://localhost:8000)"
    echo ""
    echo "Examples:"
    echo "  $0 test"
    echo "  $0 agents"
    echo "  $0 run my-agent 'Hello, world!'"
    echo "  ACP_URL=http://other-server:8000 $0 ping"
}

case "${1:-test}" in
    test)
        run_tests
        ;;
    ping)
        acp_ping
        ;;
    agents|list)
        acp_list_agents "${2:-10}" "${3:-0}"
        ;;
    agent|get)
        acp_get_agent "$2"
        ;;
    run|sync)
        acp_run_sync "$2" "$3"
        ;;
    async)
        acp_run_async "$2" "$3"
        ;;
    status|get-run)
        acp_get_run "$2"
        ;;
    cancel)
        acp_cancel_run "$2"
        ;;
    events)
        acp_get_run_events "$2"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        error "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
