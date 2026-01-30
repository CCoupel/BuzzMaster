# Build script for BuzzControl portable executable
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

Write-Host "=== BuzzControl Build Script ===" -ForegroundColor Cyan

# Step 1: Build frontend
Write-Host "`n[1/2] Building frontend..." -ForegroundColor Yellow
Set-Location web
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Frontend build failed!" -ForegroundColor Red
    exit 1
}
Set-Location ..

# Step 2: Read version from config.json
Write-Host "`n[2/3] Reading version..." -ForegroundColor Yellow
$configJson = Get-Content "config.json" | ConvertFrom-Json
$version = $configJson.version
Write-Host "Version: $version" -ForegroundColor Cyan

# Step 3: Build Go executable with embedded version
Write-Host "`n[3/3] Building Go executable..." -ForegroundColor Yellow
$ldflags = "-X main.Version=$version"
go build -ldflags "$ldflags" -o server.exe ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Go build failed!" -ForegroundColor Red
    exit 1
}

# Show result
$size = (Get-Item "server.exe").Length / 1MB
Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Write-Host "Executable: server.exe ($([math]::Round($size, 2)) MB)" -ForegroundColor Green
Write-Host "Mode: Portable (embedded web files from web/dist)" -ForegroundColor Green
Write-Host "`nTo run: .\server.exe" -ForegroundColor Cyan
