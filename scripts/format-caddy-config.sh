#!/usr/bin/env bash
#
# format-caddy-config.sh - Validate and format Caddyfile using Docker container
#
# Usage: ./format-caddy-config.sh <path-to-Caddyfile>
#
# Validates and reformats Caddyfile configuration using the Caddy fmt command
# in a Docker container. Uses the same Caddy image version as production (from straw/plans/.env).
# The formatted file will overwrite the original.
#
# Exit codes:
#   0 - Configuration is valid and formatted successfully
#   1 - Configuration is invalid or formatting failed

set -euo pipefail

# Configuration - defaults to GHCR image with digest
CADDY_IMAGE="${CADDY_IMAGE:-ghcr.io/the-agent-c-ai/caddy}"
CADDY_DIGEST="${CADDY_DIGEST:-sha256:47ff1754ab3210cefbb4287d840bd091e8eac9981dbe2a92b0ab2a3222721d22}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Usage
usage() {
    echo "Usage: $0 <path-to-Caddyfile>"
    echo ""
    echo "Validates and formats Caddyfile configuration using Docker container."
    echo "The formatted file will overwrite the original."
    echo ""
    echo "Environment variables:"
    echo "  CADDY_IMAGE   - Caddy Docker image (default: ghcr.io/the-agent-c-ai/caddy)"
    echo "  CADDY_DIGEST  - Caddy image digest (default: sha256:47ff1754ab3210cefbb4287d840bd091e8eac9981dbe2a92b0ab2a3222721d22)"
    exit 1
}

# Check arguments
if [ $# -ne 1 ]; then
    echo -e "${RED}Error: Caddyfile path required${NC}" >&2
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
CONFIG_DIR="$(dirname "$CONFIG_FILE_ABS")"
CONFIG_FILENAME="$(basename "$CONFIG_FILE_ABS")"

# Build full image name (using digest for production stability)
FULL_IMAGE="${CADDY_IMAGE}@${CADDY_DIGEST}"

echo -e "${YELLOW}Validating and formatting Caddyfile: $CONFIG_FILE${NC}"
echo -e "${YELLOW}Using image: $FULL_IMAGE${NC}"
echo ""

# Run Caddy fmt in container
# Mount the config directory and run caddy fmt command with --overwrite
if docker run --rm \
    -v "$CONFIG_DIR:/etc/caddy:rw" \
    "$FULL_IMAGE" \
    caddy fmt --overwrite "/etc/caddy/$CONFIG_FILENAME"; then
    echo ""
    echo -e "${GREEN}✓ Configuration is valid and has been formatted${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}✗ Configuration validation or formatting failed${NC}"
    exit 1
fi