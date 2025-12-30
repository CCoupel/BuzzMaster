.PHONY: build run clean test build-pi build-windows

# Default target
all: build

# Build for current platform
build:
	go build -o buzzcontrol ./cmd/server

# Run locally
run:
	go run ./cmd/server

# Build for Raspberry Pi (ARM64)
build-pi:
	GOOS=linux GOARCH=arm64 go build -o buzzcontrol-linux-arm64 ./cmd/server

# Build for Raspberry Pi Zero (ARM32)
build-pi-zero:
	GOOS=linux GOARCH=arm GOARM=6 go build -o buzzcontrol-linux-arm ./cmd/server

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o buzzcontrol.exe ./cmd/server

# Build for Linux x64
build-linux:
	GOOS=linux GOARCH=amd64 go build -o buzzcontrol-linux-amd64 ./cmd/server

# Build all platforms
build-all: build-windows build-linux build-pi build-pi-zero

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f buzzcontrol buzzcontrol.exe buzzcontrol-linux-*

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	go vet ./...

# Copy web assets from ESP32 data folder
copy-web:
	mkdir -p data
	cp -r ../data/* ./data/ 2>/dev/null || true

# Deploy to Raspberry Pi (requires SSH key)
deploy-pi: build-pi
	scp buzzcontrol-linux-arm64 pi@raspberrypi.local:~/buzzcontrol/
	scp config.json pi@raspberrypi.local:~/buzzcontrol/
