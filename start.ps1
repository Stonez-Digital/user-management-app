# Stonez School Management - one-command local launcher
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$launcher = Join-Path $root "scripts\dev.ps1"

if (-not (Test-Path $launcher)) {
  throw "Startup launcher not found: $launcher"
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "Go is not installed or not available on PATH."
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
  throw "Node.js/npm is not installed or not available on PATH."
}

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  STONEZ DIGITAL SCHOOL MANAGEMENT" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  API      : http://localhost:8080" -ForegroundColor Green
Write-Host "  Frontend : http://localhost:3000" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

& $launcher
