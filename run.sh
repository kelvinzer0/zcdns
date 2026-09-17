#!/bin/bash
set -eu

# Graceful cleanup on SIGINT or SIGTERM
trap 'kill $(jobs -p) 2>/dev/null || true' SIGINT SIGTERM EXIT

echo "=================================================="
echo "    Starting ZeroCentDNS (ZCDNS) Environment"
echo "    Backend API & DNS Engine + Frontend Web UI"
echo "=================================================="

export PORT="${PORT:-8080}"
export DNS_PORT="${DNS_PORT:-5354}"
export BASE_DOMAIN="${BASE_DOMAIN:-zcdns.id}"
export DB_PATH="${DB_PATH:-./zcdns.sqlite}"

# 1. Start Go Backend
echo "[1/2] Launching ZCDNS backend engine..."
(
  cd backend
  if [ -f "./zcdns-server" ]; then
    ./zcdns-server
  else
    go run .
  fi
) &

sleep 1

# 2. Start Frontend Dev Server
echo "[2/2] Launching Vite frontend..."
bun run dev --host 0.0.0.0 --port 5173 &

wait
