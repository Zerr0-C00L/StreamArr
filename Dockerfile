# Build stage for Go backend
FROM golang:1.24-alpine AS backend-builder

# Build arguments for version info
ARG VERSION=main
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries with version info (get git commit and build date if not provided)
RUN GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    if [ "${COMMIT}" = "unknown" ] || [ -z "${COMMIT}" ]; then ACTUAL_COMMIT="${GIT_COMMIT}"; else ACTUAL_COMMIT="${COMMIT}"; fi && \
    if [ "${BUILD_DATE}" = "unknown" ] || [ -z "${BUILD_DATE}" ]; then ACTUAL_DATE="${BUILD_TIME}"; else ACTUAL_DATE="${BUILD_DATE}"; fi && \
    echo "Building with Version: ${VERSION}, Commit: ${ACTUAL_COMMIT}, Date: ${ACTUAL_DATE}" && \
    LDFLAGS="-X 'github.com/Zerr0-C00L/StreamArr/internal/api.Version=${VERSION}' -X 'github.com/Zerr0-C00L/StreamArr/internal/api.Commit=${ACTUAL_COMMIT}' -X 'github.com/Zerr0-C00L/StreamArr/internal/api.BuildDate=${ACTUAL_DATE}'" && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "$LDFLAGS" -o bin/server cmd/server/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "$LDFLAGS" -o bin/worker cmd/worker/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "$LDFLAGS" -o bin/migrate cmd/migrate/main.go

# Build stage for React frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/streamarr-pro-ui

# Copy package files
COPY streamarr-pro-ui/package*.json ./

# Install dependencies
RUN npm ci

# Copy source and build
COPY streamarr-pro-ui/ ./
RUN npm run build

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binaries from builder
COPY --from=backend-builder /app/bin/server /app/bin/server
COPY --from=backend-builder /app/bin/worker /app/bin/worker
COPY --from=backend-builder /app/bin/migrate /app/bin/migrate

# Copy migrations
COPY --from=backend-builder /app/migrations /app/migrations

# Copy frontend build
COPY --from=frontend-builder /app/streamarr-pro-ui/dist /app/streamarr-pro-ui/dist

# Copy channel files and configs
COPY channels/ /app/channels/

# Copy update and build scripts for in-app updates
COPY scripts/update.sh scripts/build.sh scripts/start.sh scripts/stop.sh docker-compose.yml entrypoint.sh load_proxies.sh ./

# Install git, docker-cli, docker-compose, and dos2unix for container updates
RUN apk add --no-cache git bash docker-cli docker-cli-compose dos2unix

# Convert line endings and make scripts executable
RUN dos2unix update.sh build.sh start.sh stop.sh entrypoint.sh load_proxies.sh && \
    chmod +x update.sh build.sh start.sh stop.sh entrypoint.sh load_proxies.sh

# Create directories
RUN mkdir -p /app/logs /app/cache /app/sessions

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Default command - use entrypoint to start both server and worker
CMD ["/app/entrypoint.sh"]
