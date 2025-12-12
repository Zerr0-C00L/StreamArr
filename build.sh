#!/bin/bash

# StreamArr Build Script
# Automatically embeds version info from git tags

# Get version from latest git tag, or default to "main"
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "main")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Package path for ldflags
PKG="github.com/Zerr0-C00L/StreamArr/internal/api"

# Build flags
LDFLAGS="-X '${PKG}.Version=${VERSION}' -X '${PKG}.Commit=${COMMIT}' -X '${PKG}.BuildDate=${BUILD_DATE}'"

echo "Building StreamArr..."
echo "  Version: ${VERSION}"
echo "  Commit:  ${COMMIT}"
echo "  Date:    ${BUILD_DATE}"
echo ""

# Determine target OS
TARGET_OS=${1:-$(go env GOOS)}
TARGET_ARCH=${2:-$(go env GOARCH)}

# Always output to bin/server for simplicity
OUTPUT="bin/server"

echo "Building for ${TARGET_OS}/${TARGET_ARCH}..."
GOOS=$TARGET_OS GOARCH=$TARGET_ARCH go build -ldflags "$LDFLAGS" -o "$OUTPUT" cmd/server/main.go

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful: $OUTPUT"
    ls -lh "$OUTPUT"
else
    echo ""
    echo "❌ Build failed"
    exit 1
fi
