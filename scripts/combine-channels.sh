#!/bin/bash

# Script to combine all M3U files from lalajo repository
# Usage: ./combine-channels.sh [output_file]

OUTPUT_FILE="${1:-channels/combined.m3u}"
TEMP_DIR="/tmp/lalajo-channels"
REPO_URL="https://raw.githubusercontent.com/Zerr0-C00L/lalajo/main"

# M3U files to download from the repository
M3U_FILES=(
    "iptv"
    "playlist25"
    "daddylive-channels"
    "denstv"
    "fstv"
    "rdstv"
    "trial"
    "tvpass"
    "udptv"
    "vision+"
    "vod"
    "yoyo"
    "DewaNonton"
    "china"
    "hbogoasia"
)

# Create temp directory
mkdir -p "$TEMP_DIR"

echo "Combining M3U files from lalajo repository..."
echo "#EXTM3U" > "$OUTPUT_FILE"
echo "# Combined playlist generated on $(date)" >> "$OUTPUT_FILE"
echo "# Source: https://github.com/Zerr0-C00L/lalajo" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Download and combine each file
for file in "${M3U_FILES[@]}"; do
    echo "Processing: $file"
    TEMP_FILE="$TEMP_DIR/$file.m3u"
    
    # Download the file
    if curl -sSL "$REPO_URL/$file" -o "$TEMP_FILE" 2>/dev/null; then
        # Check if file is valid
        if [ -s "$TEMP_FILE" ]; then
            # Add comment header for this source
            echo "# --- Channels from $file ---" >> "$OUTPUT_FILE"
            
            # Append content (skip #EXTM3U header if present)
            grep -v "^#EXTM3U" "$TEMP_FILE" >> "$OUTPUT_FILE"
            echo "" >> "$OUTPUT_FILE"
            
            echo "  ✓ Added channels from $file"
        else
            echo "  ⚠ Skipped $file (empty or not found)"
        fi
    else
        echo "  ✗ Failed to download $file"
    fi
done

# Clean up
rm -rf "$TEMP_DIR"

# Count channels
CHANNEL_COUNT=$(grep -c "^#EXTINF" "$OUTPUT_FILE")
echo ""
echo "✓ Combined $CHANNEL_COUNT channels into $OUTPUT_FILE"
echo "File size: $(du -h "$OUTPUT_FILE" | cut -f1)"
