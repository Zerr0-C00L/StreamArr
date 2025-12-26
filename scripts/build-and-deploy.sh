#!/bin/bash
# Build and deploy StreamArr changes

echo "=== Building StreamArr ==="

# Build the binary
cd /Users/zeroq/StreamArr\ Pro
go build -o bin/server ./cmd/server

if [ $? -eq 0 ]; then
    echo "✓ Build successful"
    
    echo ""
    echo "=== Deploying to Docker ==="
    
    # Rebuild Docker image
    docker build -t streamarrpro-streamarr:latest .
    
    if [ $? -eq 0 ]; then
        echo "✓ Docker image built"
        
        echo ""
        echo "=== Restarting container ==="
        
        # Stop and remove old container (if running locally)
        # For remote: you'll need to do this manually or use SSH
        
        echo ""
        echo "Build complete! For remote deployment:"
        echo "  1. Upload new image or binary"
        echo "  2. Restart the container: docker restart streamarr"
        echo "  3. Test: curl http://77.42.16.119/get.php?username=streamarr&password=streamarr"
    else
        echo "✗ Docker build failed"
        exit 1
    fi
else
    echo "✗ Build failed"
    exit 1
fi
