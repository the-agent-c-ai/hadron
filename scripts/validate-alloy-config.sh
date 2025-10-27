#!/usr/bin/env bash
#
# validate-alloy-config.sh - Validate Grafana Alloy configuration using Docker container
#
# Usage: ./validate-alloy-config.sh <path-to-config.alloy>
#
# Validates Alloy configuration by running it through the Alloy fmt command
# in a Docker container. Uses the same Alloy image version as production (from straw/plans/.env).
#
# Note: Dummy environment variables are provided for validation purposes only.
#
# Exit codes:
#   0 - Configuration is valid
#   1 - Configuration is invalid or validation failed

set -euo pipefail

# Configuration - defaults to GHCR image with digest
ALLOY_IMAGE="${ALLOY_IMAGE:-ghcr.io/the-agent-c-ai/alloy}"
ALLOY_DIGEST="${ALLOY_DIGEST:-sha256:b21dce08f83209909b975aa99de5434733f0d89dcaf257d540d2bcc85431470a}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Usage
usage() {
    echo "Usage: $0 <path-to-config.alloy>"
    echo ""
    echo "Validates Grafana Alloy configuration using Docker container."
    echo ""
    echo "Environment variables:"
    echo "  ALLOY_IMAGE   - Alloy Docker image (default: ghcr.io/the-agent-c-ai/alloy)"
    echo "  ALLOY_DIGEST  - Alloy image digest (default: sha256:b21dce08f83209909b975aa99de5434733f0d89dcaf257d540d2bcc85431470a)"
    exit 1
}

# Check arguments
if [ $# -ne 1 ]; then
    echo -e "${RED}Error: Config file path required${NC}" >&2
    usage
fi

CONFIG_FILE="$1"

# Validate config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_FILE${NC}" >&2
    exit 1
fi

# Get absolute path
CONFIG_FILE_ABS="$(cd "$(dirname "$CONFIG_FILE")" && pwd)/$(basename "$CONFIG_FILE")"

# Build full image name (using digest for production stability)
FULL_IMAGE="${ALLOY_IMAGE}@${ALLOY_DIGEST}"

echo -e "${YELLOW}Validating Alloy config: $CONFIG_FILE${NC}"
echo -e "${YELLOW}Using image: $FULL_IMAGE${NC}"
echo ""

# Run Alloy fmt in container
# Provide common dummy environment variables for validation (syntax check only)
if docker run --rm \
    -v "$CONFIG_FILE_ABS:/etc/alloy/config.alloy:ro" \
    -e "PROMETHEUS_ENDPOINT=https://prometheus-validation.grafana.net/api/prom/push" \
    -e "PROMETHEUS_USERNAME=validation-user" \
    -e "PROMETHEUS_PASSWORD=validation-password" \
    -e "ENVIRONMENT=validation" \
    "$FULL_IMAGE" \
    fmt --write=false /etc/alloy/config.alloy; then
    echo ""
    echo -e "${GREEN}✓ Configuration is valid${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}✗ Configuration validation failed${NC}"
    exit 1
fi
