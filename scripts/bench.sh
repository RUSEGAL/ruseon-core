#!/usr/bin/env bash
# scripts/bench.sh — Run Docker-isolated benchmark
set -euo pipefail

PROFILE="${1:-baseline}"
export BENCH_PROFILE="$PROFILE"

COMPOSE_FILE="docker-compose.bench.yml"
RESULTS_DIR="./bench-results"

mkdir -p "$RESULTS_DIR"

echo "=== Running Benchmark Profile: $PROFILE ==="

echo "=== Building benchmark container ==="
docker compose -f "$COMPOSE_FILE" build

echo "=== Starting benchmark ==="
docker compose -f "$COMPOSE_FILE" up \
  --abort-on-container-exit \
  --exit-code-from bench-client

echo "=== Results saved to $RESULTS_DIR ==="
ls -lh "$RESULTS_DIR"

echo "=== Cleaning up benchmark containers ==="
docker compose -f "$COMPOSE_FILE" down --volumes --remove-orphans
