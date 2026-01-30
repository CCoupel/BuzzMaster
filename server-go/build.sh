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

# Step 2: Read version from config.json
echo ""
echo "[2/3] Reading version..."
VERSION=$(grep '"version"' config.json | sed 's/.*"version": *"\([^"]*\)".*/\1/')
echo "Version: $VERSION"

# Step 3: Build Go executable with embedded version
echo ""
echo "[3/3] Building Go executable..."
go build -ldflags="-X main.Version=$VERSION" -o server.exe ./cmd/server

# Show result
SIZE=$(ls -lh server.exe | awk '{print $5}')
echo ""
echo "=== Build Complete ==="
echo "Executable: server.exe ($SIZE)"
echo "Mode: Portable (embedded web files from web/dist)"
echo ""
echo "To run: ./server.exe"
