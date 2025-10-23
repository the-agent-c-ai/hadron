#!/usr/bin/env bash
#
# validate-vector-config.sh - Validate Vector configuration using Docker container
#
# Usage: ./validate-vector-config.sh <path-to-config.yaml>
#
# Validates Vector YAML configuration by running it through the Vector validate command
# in a Docker container. Uses the same Vector image version as production (from straw/plans/.env).
#
# Note: Dummy environment variables are provided for validation purposes only.
#
# Exit codes:
#   0 - Configuration is valid
#   1 - Configuration is invalid or validation failed

set -euo pipefail

# Configuration - defaults to GHCR image with digest
VECTOR_IMAGE="${VECTOR_IMAGE:-ghcr.io/the-agent-c-ai/vector}"
VECTOR_DIGEST="${VECTOR_DIGEST:-sha256:fa91645f7ca1fbb3e10a103acfcb6832228578d2d3746b79de9582a1a95335e9}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Usage
usage() {
    echo "Usage: $0 <path-to-config.yaml>"
    echo ""
    echo "Validates Vector configuration using Docker container."
    echo ""
    echo "Environment variables:"
    echo "  VECTOR_IMAGE   - Vector Docker image (default: ghcr.io/the-agent-c-ai/vector)"
    echo "  VECTOR_DIGEST  - Vector image digest (default: sha256:fa91645f7ca1fbb3e10a103acfcb6832228578d2d3746b79de9582a1a95335e9)"
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
FULL_IMAGE="${VECTOR_IMAGE}@${VECTOR_DIGEST}"

echo -e "${YELLOW}Validating Vector config: $CONFIG_FILE${NC}"
echo -e "${YELLOW}Using image: $FULL_IMAGE${NC}"
echo ""

# Run Vector validate in container
# Provide common dummy environment variables for validation (syntax check only)
if docker run --rm \
    -v "$CONFIG_FILE_ABS:/etc/vector/vector.yaml:ro" \
    -e "LOKI_ENDPOINT=https://validation.example.com" \
    -e "LOKI_USERNAME=validation-user" \
    -e "LOKI_PASSWORD=validation-password" \
    -e "ENVIRONMENT=validation" \
    -e "VECTOR_LOG=info" \
    -e "VECTOR_VERSION=0.0.0" \
    "$FULL_IMAGE" \
    validate /etc/vector/vector.yaml; then
    echo ""
    echo -e "${GREEN}✓ Configuration is valid${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}✗ Configuration validation failed${NC}"
    exit 1
fi