#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

compose() {
    docker compose -f "$REPO_ROOT/docker-compose.yml" -f "$REPO_ROOT/docker-compose.scale.yml" "$@"
}

NODE_COUNT="${NODE_COUNT:-3}"

node_services() {
    local services=()
    local i
    for i in $(seq 1 "$NODE_COUNT"); do
        services+=("node${i}")
    done
    printf '%s\n' "${services[@]}"
}

echo "============================================================"
echo "  Starting Blockchain-Secured Simulated LAN"
echo "============================================================"
echo

echo "Checking prerequisites..."

if ! docker ps --format '{{.Names}}' | grep -q '^peer0.org1.example.com$'; then
    echo "Fabric network is not running."
    echo
    echo "Start it first from fabric-samples/test-network:"
    echo "  ./network.sh up createChannel -ca -c mychannel"
    echo "  ./network.sh deployCC -ccn arptracker -ccp /path/to/Blockchain-ARP/chaincode -ccl go -c mychannel"
    exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q '^dev-peer0.*arptracker'; then
    echo "ARP chaincode container is not running."
    echo "Deploy arptracker before starting the simulated LAN."
    exit 1
fi

echo "Building Docker images..."
compose build

echo
echo "Starting containers..."
mapfile -t NODES < <(node_services)
compose up -d dashboard router "${NODES[@]}"

echo
echo "Waiting for services..."
sleep 10

compose ps

echo
echo "============================================================"
echo "  Simulated LAN is running"
echo "============================================================"
echo "Dashboard:  http://localhost:5000"
echo "Router:     blockchain-router (10.5.0.1)"
echo "Nodes:      ${NODE_COUNT} real LAN node containers"
for i in $(seq 1 "$NODE_COUNT"); do
    ip_octet=$((9 + i))
    printf '  Node %-2s   lan-node-%s (10.5.0.%s)\n' "$i" "$i" "$ip_octet"
done
echo
echo "Verify end-to-end:"
echo "  ./scripts/verify-e2e.sh"
