#!/bin/bash
# Upgrade Diane on all nodes in the development environment
# Usage: ./scripts/upgrade-all-nodes.sh [--force]
#
# Canonical binary location on all nodes: ~/.diane/bin/diane
# On macOS: ~/.diane/bin/diane is a symlink -> Diane.app/Contents/MacOS/diane-server
# On Linux: ~/.diane/bin/diane is the real binary

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Node definitions: name|host|user
NODES=(
    "Master (mcj-emergent)|100.71.82.7|root"
    "Slave 1 (Mac.banglab)|100.123.170.53|mcj"
    "Slave 2 (Laptop)|100.75.227.125|mcj"
)

# Parse flags
FORCE_FLAG=""
if [[ "$1" == "--force" ]]; then
    FORCE_FLAG="--force"
    echo -e "${YELLOW}Force upgrade enabled${NC}"
fi

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Diane Multi-Node Upgrade Script      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo "Canonical binary location: ~/.diane/bin/diane"
echo ""

# Track results (parallel arrays — works on bash 3+)
node_names=()
node_results=()

# Function to upgrade a remote node
upgrade_node() {
    local node_name="$1"
    local host="$2"
    local user="$3"

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Upgrading: ${node_name}${NC}"
    echo -e "${BLUE}Host: ${user}@${host}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Check if host is reachable
    if ! ssh -o ConnectTimeout=5 -o BatchMode=yes "${user}@${host}" exit 2>/dev/null; then
        echo -e "${YELLOW}⚠ ${node_name}: Host unreachable (skipping)${NC}"
        return 2
    fi

    # Canonical binary location — always ~/.diane/bin/diane
    local DIANE_BIN="~/.diane/bin/diane"

    # Verify the binary exists and is executable
    if ! ssh "${user}@${host}" "test -x ${DIANE_BIN}" 2>/dev/null; then
        echo -e "${YELLOW}⚠ ${node_name}: No executable binary at ${DIANE_BIN} (skipping)${NC}"
        echo -e "   Install diane first: https://github.com/diane-assistant/diane/releases"
        return 2
    fi

    # Warn if stale binaries exist outside the canonical location
    ssh "${user}@${host}" "
        for extra in /usr/bin/diane /usr/local/bin/diane /usr/local/bin/diane-ctl; do
            if [ -e \"\$extra\" ]; then
                echo \"WARNING: stale binary found at \$extra — consider removing it\"
            fi
        done
    " 2>/dev/null || true

    echo "Using diane binary: ${DIANE_BIN}"

    # Run upgrade
    if ssh "${user}@${host}" "${DIANE_BIN} upgrade ${FORCE_FLAG}" 2>&1; then
        echo -e "${GREEN}✓ ${node_name}: Upgrade successful${NC}"
        return 0
    else
        echo -e "${RED}✗ ${node_name}: Upgrade failed${NC}"
        return 1
    fi
}

total=0
success=0
failed=0
skipped=0

for node_def in "${NODES[@]}"; do
    IFS='|' read -r name host user <<< "$node_def"
    total=$((total + 1))
    node_names+=("$name")

    set +e
    upgrade_node "$name" "$host" "$user"
    ret=$?
    set -e

    echo ""
    if [[ $ret -eq 0 ]]; then
        node_results+=("success")
        success=$((success + 1))
    elif [[ $ret -eq 2 ]]; then
        node_results+=("skipped")
        skipped=$((skipped + 1))
    else
        node_results+=("failed")
        failed=$((failed + 1))
    fi
done

# Summary
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Upgrade Summary                       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "Total nodes:    ${total}"
echo -e "${GREEN}Successful:     ${success}${NC}"
echo -e "${RED}Failed:         ${failed}${NC}"
echo -e "${YELLOW}Skipped:        ${skipped}${NC}"
echo ""

echo "Details:"
for i in "${!node_names[@]}"; do
    case "${node_results[$i]}" in
        success)
            echo -e "  ${node_names[$i]}: ${GREEN}✓ Success${NC}"
            ;;
        failed)
            echo -e "  ${node_names[$i]}: ${RED}✗ Failed${NC}"
            ;;
        skipped)
            echo -e "  ${node_names[$i]}: ${YELLOW}⚠ Skipped (unreachable or not installed)${NC}"
            ;;
    esac
done
echo ""

# Exit code
if [[ $failed -gt 0 ]]; then
    echo -e "${RED}Some upgrades failed. Please check the output above.${NC}"
    exit 1
elif [[ $skipped -gt 0 && $success -eq 0 ]]; then
    echo -e "${YELLOW}All nodes were skipped. No upgrades performed.${NC}"
    exit 2
else
    echo -e "${GREEN}All reachable nodes upgraded successfully!${NC}"
    exit 0
fi
