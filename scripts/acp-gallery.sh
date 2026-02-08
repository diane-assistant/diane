#!/bin/bash
# ACP Agent Gallery Browser
# Browse and install agents from the Agent Client Protocol registry

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

REGISTRY_URL="https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json"
CACHE_FILE="$HOME/.diane/acp-registry.json"
CONFIG_FILE="$HOME/.diane/acp-agents.json"

# Ensure directories exist
mkdir -p "$(dirname "$CACHE_FILE")"

# ============================================================================
# Registry Functions
# ============================================================================

fetch_registry() {
    echo -e "${BLUE}Fetching agent registry...${NC}" >&2
    curl -s "$REGISTRY_URL" > "$CACHE_FILE"
    echo -e "${GREEN}Registry cached at $CACHE_FILE${NC}" >&2
}

ensure_registry() {
    if [ ! -f "$CACHE_FILE" ]; then
        fetch_registry
    fi
}

# ============================================================================
# Display Functions
# ============================================================================

list_agents() {
    ensure_registry
    
    echo -e "${BOLD}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║                    🤖 ACP Agent Gallery                          ║${NC}"
    echo -e "${BOLD}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    jq -r '.agents[] | "  \(.id)|\(.name)|\(.version)|\(.description[:50])"' "$CACHE_FILE" | while IFS='|' read -r id name version desc; do
        echo -e "  ${CYAN}$id${NC}"
        echo -e "    ${BOLD}$name${NC} v$version"
        echo -e "    ${desc}..."
        echo ""
    done
    
    echo -e "${YELLOW}To install an agent:${NC}"
    echo "  $0 install <agent-id>"
    echo ""
    echo -e "${YELLOW}To see install details:${NC}"
    echo "  $0 info <agent-id>"
}

show_agent_info() {
    local id="$1"
    ensure_registry
    
    local agent=$(jq -r ".agents[] | select(.id == \"$id\")" "$CACHE_FILE")
    
    if [ -z "$agent" ] || [ "$agent" = "null" ]; then
        echo -e "${RED}Agent '$id' not found${NC}"
        echo "Use '$0 list' to see available agents"
        exit 1
    fi
    
    echo -e "${BOLD}Agent: $(echo "$agent" | jq -r '.name')${NC}"
    echo ""
    echo "$agent" | jq -r '"  ID:          \(.id)"'
    echo "$agent" | jq -r '"  Version:     \(.version)"'
    echo "$agent" | jq -r '"  Description: \(.description)"'
    echo "$agent" | jq -r '"  Repository:  \(.repository // "N/A")"'
    echo "$agent" | jq -r '"  License:     \(.license // "N/A")"'
    echo "$agent" | jq -r '"  Authors:     \(.authors | join(", "))"'
    echo ""
    
    # Show install command
    local npx_package=$(echo "$agent" | jq -r '.distribution.npx.package // empty')
    local npx_args=$(echo "$agent" | jq -r '.distribution.npx.args // [] | join(" ")')
    
    if [ -n "$npx_package" ]; then
        echo -e "${GREEN}Install Command (npx):${NC}"
        echo "  npx $npx_package $npx_args"
        echo ""
    fi
    
    # Check for binary
    local has_binary=$(echo "$agent" | jq -r '.distribution.binary // empty')
    if [ -n "$has_binary" ]; then
        echo -e "${GREEN}Binary Downloads Available:${NC}"
        echo "$agent" | jq -r '.distribution.binary | keys[]' | while read platform; do
            echo "  - $platform"
        done
        echo ""
    fi
    
    echo -e "${YELLOW}To configure this agent in Diane:${NC}"
    echo "  $0 install $id"
}

install_agent() {
    local id="$1"
    local workdir="${2:-$(pwd)}"
    ensure_registry
    
    local agent=$(jq -r ".agents[] | select(.id == \"$id\")" "$CACHE_FILE")
    
    if [ -z "$agent" ] || [ "$agent" = "null" ]; then
        echo -e "${RED}Agent '$id' not found${NC}"
        exit 1
    fi
    
    local name=$(echo "$agent" | jq -r '.name')
    local description=$(echo "$agent" | jq -r '.description')
    local npx_package=$(echo "$agent" | jq -r '.distribution.npx.package // empty')
    local npx_args=$(echo "$agent" | jq -r '.distribution.npx.args // []')
    
    # Get workdir argument for this agent
    local workdir_arg="--cwd"
    case "$id" in
        gemini) workdir_arg="--include-directories" ;;
        github-copilot) workdir_arg="--workspace-folder" ;;
    esac
    
    echo -e "${BLUE}Installing $name...${NC}"
    echo -e "${CYAN}Workspace: $workdir${NC}"
    
    # Create unique name with workspace
    local unique_name="$id"
    if [ "$workdir" != "." ] && [ -n "$workdir" ]; then
        # Use basename of workdir for uniqueness
        local workspace_name=$(basename "$workdir")
        unique_name="${id}@${workspace_name}"
    fi
    
    # Create config entry with workdir
    local config_entry=$(jq -n \
        --arg id "$id" \
        --arg unique_name "$unique_name" \
        --arg name "$name" \
        --arg desc "$description" \
        --arg workdir "$workdir" \
        --arg workdir_arg "$workdir_arg" \
        --argjson npx_args "$npx_args" \
        '{
            name: $unique_name,
            type: "stdio",
            command: "npx",
            args: ([$id] + $npx_args + [$workdir_arg, $workdir]),
            workdir: $workdir,
            enabled: true,
            description: $desc
        }')
    
    # Add to config file
    if [ -f "$CONFIG_FILE" ]; then
        local existing=$(cat "$CONFIG_FILE")
        # Check if already exists
        if echo "$existing" | jq -e ".agents[] | select(.name == \"$unique_name\")" > /dev/null 2>&1; then
            echo -e "${YELLOW}Agent '$unique_name' already configured${NC}"
        else
            echo "$existing" | jq ".agents += [$config_entry]" > "$CONFIG_FILE"
            echo -e "${GREEN}✓ Agent '$unique_name' added to configuration${NC}"
        fi
    else
        echo "{\"agents\":[$config_entry]}" | jq '.' > "$CONFIG_FILE"
        echo -e "${GREEN}✓ Agent '$unique_name' added to configuration${NC}"
    fi
    
    echo ""
    echo -e "${YELLOW}Configuration saved to:${NC} $CONFIG_FILE"
    
    if [ -n "$npx_package" ]; then
        echo ""
        echo -e "${CYAN}To run this agent manually:${NC}"
        local args_str=$(echo "$npx_args" | jq -r 'join(" ")')
        echo "  npx $npx_package $args_str $workdir_arg $workdir"
    fi
}

# ============================================================================
# Featured Agents
# ============================================================================

show_featured() {
    ensure_registry
    
    echo -e "${BOLD}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║                    ⭐ Featured Agents                            ║${NC}"
    echo -e "${BOLD}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Featured agents (curated list)
    local featured="claude-code-acp gemini github-copilot opencode codex-acp"
    
    for id in $featured; do
        local agent=$(jq -r ".agents[] | select(.id == \"$id\")" "$CACHE_FILE" 2>/dev/null)
        if [ -n "$agent" ] && [ "$agent" != "null" ]; then
            local name=$(echo "$agent" | jq -r '.name')
            local version=$(echo "$agent" | jq -r '.version')
            local desc=$(echo "$agent" | jq -r '.description[:60]')
            
            echo -e "  ⭐ ${CYAN}$id${NC}"
            echo -e "     ${BOLD}$name${NC} v$version"
            echo -e "     $desc..."
            echo ""
        fi
    done
    
    echo -e "${YELLOW}Quick Install:${NC}"
    echo "  $0 install claude-code-acp    # Claude Code by Anthropic"
    echo "  $0 install gemini             # Gemini CLI by Google"
    echo "  $0 install github-copilot     # GitHub Copilot"
    echo "  $0 install opencode           # OpenCode (open-source)"
}

# ============================================================================
# CLI
# ============================================================================

show_help() {
    echo "ACP Agent Gallery - Browse and install AI coding agents"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  list                        List all available agents"
    echo "  featured                    Show featured agents"
    echo "  info <id>                   Show detailed info for an agent"
    echo "  install <id> [workspace]    Configure an agent for a workspace"
    echo "  refresh                     Refresh the registry cache"
    echo ""
    echo "Examples:"
    echo "  $0 list"
    echo "  $0 info gemini"
    echo "  $0 install claude-code-acp                    # Install for current directory"
    echo "  $0 install gemini /path/to/project            # Install for specific project"
    echo ""
    echo "Workspace Support:"
    echo "  Each agent can be configured for a specific project/workspace."
    echo "  This allows running multiple instances of the same agent for different projects."
    echo "  The agent name will include the workspace: gemini@myproject"
    echo ""
    echo "Available Agents:"
    echo "  - claude-code-acp   Claude Code by Anthropic"
    echo "  - gemini            Gemini CLI by Google"
    echo "  - github-copilot    GitHub Copilot"
    echo "  - opencode          OpenCode (open-source)"
    echo "  - codex-acp         Codex CLI by OpenAI"
    echo "  - auggie            Auggie CLI by Augment Code"
    echo "  - qwen-code         Qwen Code by Alibaba"
    echo "  - kimi              Kimi CLI by Moonshot AI"
    echo "  - mistral-vibe      Mistral Vibe by Mistral AI"
}

case "${1:-help}" in
    list|ls)
        list_agents
        ;;
    featured|top)
        show_featured
        ;;
    info|show)
        if [ -z "$2" ]; then
            echo "Usage: $0 info <agent-id>"
            exit 1
        fi
        show_agent_info "$2"
        ;;
    install|add)
        if [ -z "$2" ]; then
            echo "Usage: $0 install <agent-id> [workspace-path]"
            echo ""
            echo "If workspace-path is omitted, current directory is used."
            exit 1
        fi
        install_agent "$2" "${3:-$(pwd)}"
        ;;
    refresh)
        fetch_registry
        echo "Registry refreshed"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        # Try as agent ID
        if [ -f "$CACHE_FILE" ]; then
            if jq -e ".agents[] | select(.id == \"$1\")" "$CACHE_FILE" > /dev/null 2>&1; then
                show_agent_info "$1"
                exit 0
            fi
        fi
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
