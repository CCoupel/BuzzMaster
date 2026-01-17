# Build script for BuzzControl portable executable
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

Write-Host "=== BuzzControl Build Script ===" -ForegroundColor Cyan

# Step 1: Build frontend
Write-Host "`n[1/3] Building frontend..." -ForegroundColor Yellow
Set-Location web
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Frontend build failed!" -ForegroundColor Red
    exit 1
}
Set-Location ..

# Step 2: Copy dist to cmd/server for embedding
Write-Host "`n[2/3] Copying dist for embedding..." -ForegroundColor Yellow
if (Test-Path "cmd/server/dist") {
    Remove-Item -Recurse -Force "cmd/server/dist"
}
Copy-Item -Recurse "web/dist" "cmd/server/dist"

# Step 3: Build Go executable
Write-Host "`n[3/3] Building Go executable..." -ForegroundColor Yellow
go build -o server.exe ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Go build failed!" -ForegroundColor Red
    exit 1
}

# Show result
$size = (Get-Item "server.exe").Length / 1MB
Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Write-Host "Executable: server.exe ($([math]::Round($size, 2)) MB)" -ForegroundColor Green
Write-Host "Mode: Portable (embedded web files)" -ForegroundColor Green
Write-Host "`nTo run: .\server.exe" -ForegroundColor Cyan
