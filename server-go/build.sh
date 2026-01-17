#!/bin/bash
# Build script for BuzzControl portable executable
# Usage: ./build.sh

set -e

echo "=== BuzzControl Build Script ==="

# Step 1: Build frontend
echo ""
echo "[1/3] Building frontend..."
cd web
npm run build
cd ..

# Step 2: Copy dist to cmd/server for embedding
echo ""
echo "[2/3] Copying dist for embedding..."
rm -rf cmd/server/dist
cp -r web/dist cmd/server/dist

# Step 3: Build Go executable
echo ""
echo "[3/3] Building Go executable..."
go build -o server.exe ./cmd/server

# Show result
SIZE=$(ls -lh server.exe | awk '{print $5}')
echo ""
echo "=== Build Complete ==="
echo "Executable: server.exe ($SIZE)"
echo "Mode: Portable (embedded web files)"
echo ""
echo "To run: ./server.exe"
