#!/bin/bash
# Docker build wrapper that passes version info

# Get version info from git
export VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "main")
export COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
export BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "Building StreamArr Pro Docker image..."
echo "  Version: ${VERSION}"
echo "  Commit:  ${COMMIT}"
echo "  Date:    ${BUILD_DATE}"
echo ""

# Build with docker-compose
docker-compose build "$@"
