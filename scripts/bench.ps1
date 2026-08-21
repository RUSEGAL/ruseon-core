# scripts/bench.ps1 — Run Docker-isolated benchmark on Windows
param(
    [ValidateSet("baseline", "chaos")]
    [string]$Profile = "baseline"
)

$ErrorActionPreference = "Stop"

$resultsDir = ".\bench-results"
if (!(Test-Path $resultsDir)) {
    New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null
}

$env:BENCH_PROFILE = $Profile
Write-Host "=== Running Benchmark Profile: $Profile ===" -ForegroundColor Magenta

Write-Host "=== Building benchmark container ===" -ForegroundColor Cyan
docker compose -f docker-compose.bench.yml build

Write-Host "=== Starting benchmark ===" -ForegroundColor Cyan
docker compose -f docker-compose.bench.yml up `
    --abort-on-container-exit `
    --exit-code-from bench-client

Write-Host "=== Results for Profile '$Profile' saved to $resultsDir ===" -ForegroundColor Green
Get-ChildItem -Path "$resultsDir\bench-$Profile.*"

Write-Host "=== Cleaning up benchmark containers ===" -ForegroundColor Yellow
docker compose -f docker-compose.bench.yml down --volumes --remove-orphans
