#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "============================================================"
echo "  Stopping Simulated LAN"
echo "============================================================"
echo

docker compose -f "$REPO_ROOT/docker-compose.yml" -f "$REPO_ROOT/docker-compose.scale.yml" down

echo
echo "All simulated LAN containers stopped and removed."
