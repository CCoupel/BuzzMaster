#!/bin/bash
# Build script for BuzzControl portable executable
# Usage: ./build.sh

set -e

echo "=== BuzzControl Build Script ==="

# Step 1: Build frontend
echo ""
echo "[1/2] Building frontend..."
cd web
npm run build
cd ..

# Step 2: Build Go executable (web/dist is embedded directly via web/embed.go)
echo ""
echo "[2/2] Building Go executable..."
go build -o server.exe ./cmd/server

# Show result
SIZE=$(ls -lh server.exe | awk '{print $5}')
echo ""
echo "=== Build Complete ==="
echo "Executable: server.exe ($SIZE)"
echo "Mode: Portable (embedded web files from web/dist)"
echo ""
echo "To run: ./server.exe"
