#!/bin/bash
# Test script for ACP (Agent Communication Protocol) multi-agent setup
# This script demonstrates how to configure and test multiple ACP agents

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log() { echo -e "${BLUE}[ACP Multi-Agent]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
info() { echo -e "${CYAN}[INFO]${NC} $1"; }

# Default paths
DIANE_CTL="${DIANE_CTL:-diane-ctl}"
CONFIG_FILE="$HOME/.diane/acp-agents.json"

# ============================================================================
# Utility Functions
# ============================================================================

check_diane_running() {
    if ! $DIANE_CTL health >/dev/null 2>&1; then
        error "Diane is not running. Please start it first."
        echo "  Run: diane start"
        return 1
    fi
    success "Diane is running"
    return 0
}

# ============================================================================
# Agent Management Functions
# ============================================================================

add_agent() {
    local name="$1"
    local url="$2"
    local description="${3:-}"
    
    log "Adding agent: $name ($url)"
    
    if [ -n "$description" ]; then
        $DIANE_CTL agent add "$name" "$url" "$description"
    else
        $DIANE_CTL agent add "$name" "$url"
    fi
}

remove_agent() {
    local name="$1"
    log "Removing agent: $name"
    $DIANE_CTL agent remove "$name" || true
}

test_agent() {
    local name="$1"
    log "Testing agent: $name"
    $DIANE_CTL agent test "$name"
}

list_agents() {
    log "Listing all configured agents..."
    $DIANE_CTL agents
}

# ============================================================================
# Example Configurations
# ============================================================================

setup_example_agents() {
    echo "=========================================="
    echo "  Setting Up Example ACP Agents"
    echo "=========================================="
    echo ""
    
    # Example 1: BeeAI Platform (local)
    log "Adding BeeAI Platform agent (if running locally)..."
    add_agent "beeai" "http://localhost:8000" "BeeAI Platform - Local" || true
    
    # Example 2: Custom ACP Server
    log "Adding custom ACP server..."
    add_agent "custom-acp" "http://localhost:9000" "Custom ACP Server" || true
    
    echo ""
    success "Example agents configured!"
}

# ============================================================================
# Test Suite
# ============================================================================

run_tests() {
    echo "=========================================="
    echo "  ACP Multi-Agent Test Suite"
    echo "=========================================="
    echo ""
    echo "Config file: $CONFIG_FILE"
    echo ""
    
    # Check Diane is running
    if ! check_diane_running; then
        exit 1
    fi
    echo ""
    
    # List current agents
    echo "--- Current Agents ---"
    list_agents
    echo ""
    
    # Test each agent
    echo "--- Testing Agents ---"
    local agents=$($DIANE_CTL agents 2>/dev/null | grep -E "^\s+\S+" | awk '{print $1}' || true)
    
    if [ -z "$agents" ]; then
        info "No agents configured. Add some with:"
        echo "  $DIANE_CTL agent add <name> <url>"
        echo ""
        echo "Example ACP servers you can connect to:"
        echo "  - BeeAI Platform: pip install beeai-framework && beeai serve"
        echo "  - LangServe: langchain serve --port 8000"
        echo ""
    else
        for agent in $agents; do
            test_agent "$agent" || true
            echo ""
        done
    fi
    
    echo "=========================================="
    echo "  Test Complete"
    echo "=========================================="
}

# ============================================================================
# Direct Agent Configuration (without Diane running)
# ============================================================================

direct_add_agent() {
    local name="$1"
    local url="$2"
    local description="${3:-}"
    
    log "Adding agent directly to config: $name"
    
    # Ensure config directory exists
    mkdir -p "$(dirname "$CONFIG_FILE")"
    
    # Read existing config or create new
    if [ -f "$CONFIG_FILE" ]; then
        local config=$(cat "$CONFIG_FILE")
    else
        local config='{"agents":[]}'
    fi
    
    # Check if agent already exists
    if echo "$config" | jq -e ".agents[] | select(.name == \"$name\")" >/dev/null 2>&1; then
        error "Agent '$name' already exists"
        return 1
    fi
    
    # Add new agent
    local new_agent=$(jq -n \
        --arg name "$name" \
        --arg url "$url" \
        --arg desc "$description" \
        '{name: $name, url: $url, type: "acp", enabled: true, description: $desc}')
    
    echo "$config" | jq ".agents += [$new_agent]" > "$CONFIG_FILE"
    
    success "Agent '$name' added to $CONFIG_FILE"
}

direct_list_agents() {
    if [ ! -f "$CONFIG_FILE" ]; then
        info "No agents configured. Config file: $CONFIG_FILE"
        return
    fi
    
    log "Agents from $CONFIG_FILE:"
    jq -r '.agents[] | "  \(.name): \(.url) [\(if .enabled then "enabled" else "disabled" end)]"' "$CONFIG_FILE"
}

direct_remove_agent() {
    local name="$1"
    
    if [ ! -f "$CONFIG_FILE" ]; then
        error "Config file not found: $CONFIG_FILE"
        return 1
    fi
    
    log "Removing agent: $name"
    
    local config=$(cat "$CONFIG_FILE")
    echo "$config" | jq "del(.agents[] | select(.name == \"$name\"))" > "$CONFIG_FILE"
    
    success "Agent '$name' removed"
}

# ============================================================================
# CLI
# ============================================================================

show_help() {
    echo "ACP Multi-Agent Configuration Script"
    echo ""
    echo "Usage: $0 [command] [args...]"
    echo ""
    echo "Commands (via Diane API - requires Diane running):"
    echo "  test                    Run test suite on all agents"
    echo "  list                    List all configured agents"
    echo "  add <name> <url> [desc] Add a new agent"
    echo "  remove <name>           Remove an agent"
    echo "  test-agent <name>       Test a specific agent"
    echo "  setup                   Add example agents"
    echo ""
    echo "Commands (direct config file access):"
    echo "  direct-add <name> <url> [desc]  Add agent directly to config"
    echo "  direct-list                     List agents from config file"
    echo "  direct-remove <name>            Remove agent from config"
    echo ""
    echo "Environment:"
    echo "  DIANE_CTL               Path to diane-ctl (default: diane-ctl)"
    echo ""
    echo "Examples:"
    echo "  $0 add beeai http://localhost:8000 'BeeAI Platform'"
    echo "  $0 test-agent beeai"
    echo "  $0 direct-add myagent http://example.com:8000"
}

case "${1:-test}" in
    test)
        run_tests
        ;;
    list|agents)
        list_agents
        ;;
    add)
        if [ $# -lt 3 ]; then
            error "Usage: $0 add <name> <url> [description]"
            exit 1
        fi
        add_agent "$2" "$3" "${4:-}"
        ;;
    remove|rm|delete)
        if [ $# -lt 2 ]; then
            error "Usage: $0 remove <name>"
            exit 1
        fi
        remove_agent "$2"
        ;;
    test-agent)
        if [ $# -lt 2 ]; then
            error "Usage: $0 test-agent <name>"
            exit 1
        fi
        test_agent "$2"
        ;;
    setup)
        setup_example_agents
        ;;
    direct-add)
        if [ $# -lt 3 ]; then
            error "Usage: $0 direct-add <name> <url> [description]"
            exit 1
        fi
        direct_add_agent "$2" "$3" "${4:-}"
        ;;
    direct-list)
        direct_list_agents
        ;;
    direct-remove)
        if [ $# -lt 2 ]; then
            error "Usage: $0 direct-remove <name>"
            exit 1
        fi
        direct_remove_agent "$2"
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
