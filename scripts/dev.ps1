# Stonez School Management local development launcher
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$env:APP_PORT = "8080"
$env:JWT_SECRET = "stonez-local-development-secret-change-before-production-2026"
$env:DB_DRIVER = "sqlite"
$env:DB_PATH = Join-Path $root "users.db"
$env:APP_ENV = "development"

Write-Host "Starting Stonez School Management..." -ForegroundColor Cyan
Write-Host "API: http://localhost:8080" -ForegroundColor Green
Write-Host "Frontend: http://localhost:3000" -ForegroundColor Green

$backend = Start-Process powershell -ArgumentList "-NoExit","-Command","Set-Location '$root'; go run ./cmd/server" -PassThru
Start-Sleep -Seconds 2

if ($backend.HasExited) {
  throw "Go API stopped during startup. Check the backend PowerShell window for the error."
}

Set-Location (Join-Path $root "frontend")
if (-not (Test-Path "node_modules")) {
  npm install
}

npm run dev
