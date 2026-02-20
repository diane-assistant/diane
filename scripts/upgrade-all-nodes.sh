#!/bin/bash
# Upgrade Diane on all nodes in the development environment
# Usage: ./scripts/upgrade-all-nodes.sh [--force]

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Node definitions
MASTER_HOST="100.71.82.7"
MASTER_USER="root"
SLAVE1_HOST="100.123.170.53"
SLAVE1_USER="mcj"
SLAVE2_HOST="100.75.227.125"
SLAVE2_USER="mcj"

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

# Function to upgrade a node
upgrade_node() {
    local node_name="$1"
    local host="$2"
    local user="$3"
    local is_local="$4"
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Upgrading: ${node_name}${NC}"
    echo -e "${BLUE}Host: ${user}@${host}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if [[ "$is_local" == "true" ]]; then
        # Local upgrade
        if diane upgrade $FORCE_FLAG; then
            echo -e "${GREEN}✓ ${node_name}: Upgrade successful${NC}"
            return 0
        else
            echo -e "${RED}✗ ${node_name}: Upgrade failed${NC}"
            return 1
        fi
    else
        # Remote upgrade via SSH
        # First check if host is reachable
        if ! ssh -o ConnectTimeout=5 -o BatchMode=yes "${user}@${host}" exit 2>/dev/null; then
            echo -e "${YELLOW}⚠ ${node_name}: Host unreachable (skipping)${NC}"
            return 2
        fi
        
        # Find diane binary (try multiple common locations)
        # On macOS, prefer diane-binary over symlinked diane to avoid conflicts with Diane.app
        DIANE_BIN=$(ssh "${user}@${host}" "if [ -x ~/.diane/bin/diane-binary ]; then echo ~/.diane/bin/diane-binary; elif which diane >/dev/null 2>&1; then which diane; else echo ~/.diane/bin/diane; fi" 2>/dev/null)
        
        # Verify the binary exists and is executable
        if ! ssh "${user}@${host}" "test -x $DIANE_BIN" 2>/dev/null; then
            echo -e "${YELLOW}⚠ ${node_name}: Diane binary not found or not executable at $DIANE_BIN (skipping)${NC}"
            return 2
        fi
        
        echo "Using diane binary: $DIANE_BIN"
        
        # Try to upgrade
        if ssh "${user}@${host}" "$DIANE_BIN upgrade $FORCE_FLAG" 2>&1; then
            echo -e "${GREEN}✓ ${node_name}: Upgrade successful${NC}"
            return 0
        else
            echo -e "${RED}✗ ${node_name}: Upgrade failed${NC}"
            return 1
        fi
    fi
}

# Track results
declare -A results
total=0
success=0
failed=0
skipped=0

# Upgrade master node (mcj-emergent)
total=$((total + 1))
if upgrade_node "Master (mcj-emergent)" "$MASTER_HOST" "$MASTER_USER" "false"; then
    results["master"]="success"
    success=$((success + 1))
else
    ret=$?
    if [[ $ret -eq 2 ]]; then
        results["master"]="skipped"
        skipped=$((skipped + 1))
    else
        results["master"]="failed"
        failed=$((failed + 1))
    fi
fi
echo ""

# Upgrade slave node 1 (Mac.banglab)
total=$((total + 1))
upgrade_node "Slave 1 (Mac.banglab)" "$SLAVE1_HOST" "$SLAVE1_USER" "false"
ret=$?
if [[ $ret -eq 0 ]]; then
    results["slave1"]="success"
    success=$((success + 1))
elif [[ $ret -eq 2 ]]; then
    results["slave1"]="skipped"
    skipped=$((skipped + 1))
else
    results["slave1"]="failed"
    failed=$((failed + 1))
fi
echo ""

# Upgrade slave node 2 (laptop)
total=$((total + 1))
upgrade_node "Slave 2 (Laptop)" "$SLAVE2_HOST" "$SLAVE2_USER" "false"
ret=$?
if [[ $ret -eq 0 ]]; then
    results["slave2"]="success"
    success=$((success + 1))
elif [[ $ret -eq 2 ]]; then
    results["slave2"]="skipped"
    skipped=$((skipped + 1))
else
    results["slave2"]="failed"
    failed=$((failed + 1))
fi
echo ""

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

# Detailed results
echo "Details:"
for node in master slave1 slave2; do
    case ${results[$node]} in
        success)
            echo -e "  ${node}: ${GREEN}✓ Success${NC}"
            ;;
        failed)
            echo -e "  ${node}: ${RED}✗ Failed${NC}"
            ;;
        skipped)
            echo -e "  ${node}: ${YELLOW}⚠ Skipped (unreachable)${NC}"
            ;;
    esac
done
echo ""

# Exit with appropriate code
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
