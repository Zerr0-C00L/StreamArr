#!/bin/bash
echo "Testing Balkan VOD Categories API..."
echo "Endpoint: POST http://77.42.16.119:8080/api/v1/balkan-vod/preview-categories"
echo ""
curl -X POST http://77.42.16.119:8080/api/v1/balkan-vod/preview-categories \
  -H "Content-Type: application/json" \
  --max-time 10 \
  2>&1 | head -50
