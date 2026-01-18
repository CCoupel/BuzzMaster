# build-release.ps1
# Script de build pour la release BuzzControl (Windows + Linux ARM64)

param(
    [switch]$SkipFrontend,
    [switch]$WindowsOnly,
    [switch]$LinuxOnly
)

$ErrorActionPreference = "Stop"

# Lire la version depuis config.json
$config = Get-Content config.json | ConvertFrom-Json
$VERSION = $config.version

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " BuzzControl Release Build v$VERSION" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Build du frontend (si pas skip)
if (-not $SkipFrontend) {
    Write-Host "[1/4] Building frontend..." -ForegroundColor Yellow
    npm run build --prefix web
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Frontend build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "Frontend build complete." -ForegroundColor Green
} else {
    Write-Host "[1/4] Skipping frontend build." -ForegroundColor Gray
}

# 2. Copier dist pour l'embedding
Write-Host "[2/4] Preparing embedded files..." -ForegroundColor Yellow
Remove-Item -Recurse -Force cmd/server/dist -ErrorAction SilentlyContinue
Copy-Item -Recurse web/dist cmd/server/dist
Write-Host "Embedded files ready." -ForegroundColor Green

# 3. Creer le dossier releases
New-Item -ItemType Directory -Force -Path releases | Out-Null

# 4. Build des executables
Write-Host "[3/4] Building executables..." -ForegroundColor Yellow

if (-not $LinuxOnly) {
    Write-Host "  - Windows AMD64..." -ForegroundColor Gray
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o "releases/buzzcontrol-v$VERSION-windows-amd64.exe" ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Windows build failed!" -ForegroundColor Red
        exit 1
    }
}

if (-not $WindowsOnly) {
    Write-Host "  - Linux ARM64 (Raspberry Pi)..." -ForegroundColor Gray
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o "releases/buzzcontrol-v$VERSION-linux-arm64" ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Linux ARM64 build failed!" -ForegroundColor Red
        exit 1
    }
}

# Reset env
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "Executables built." -ForegroundColor Green

# 5. Afficher les resultats
Write-Host "[4/4] Build complete!" -ForegroundColor Yellow
Write-Host ""
Write-Host "Generated files:" -ForegroundColor Cyan
Get-ChildItem releases/ | ForEach-Object {
    $size = [math]::Round($_.Length / 1MB, 2)
    Write-Host "  - $($_.Name) ($size MB)" -ForegroundColor White
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Release v$VERSION ready!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Test the executables"
Write-Host "  2. git add . && git commit -m 'release: vX.Y.0'"
Write-Host "  3. git push"
Write-Host "  4. git tag -a v$VERSION -m 'Release v$VERSION' && git push origin v$VERSION"
Write-Host "  5. Upload to Bitbucket Downloads"
