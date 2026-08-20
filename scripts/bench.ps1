# scripts/bench.ps1 — Run Docker-isolated benchmark on Windows

$ErrorActionPreference = "Stop"

$resultsDir = ".\bench-results"
if (!(Test-Path $resultsDir)) {
    New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
}

Write-Host "=== Building benchmark container ===" -ForegroundColor Cyan
docker compose -f docker-compose.bench.yml build

Write-Host "=== Starting benchmark ===" -ForegroundColor Cyan
docker compose -f docker-compose.bench.yml up `
    --abort-on-container-exit `
    --exit-code-from bench-client

Write-Host "=== Results saved to $resultsDir ===" -ForegroundColor Green
Get-ChildItem $resultsDir

Write-Host "=== Cleaning up benchmark containers ===" -ForegroundColor Yellow
docker compose -f docker-compose.bench.yml down --volumes --remove-orphans
