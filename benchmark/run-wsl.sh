#!/usr/bin/env bash
# run-wsl.sh — Build and run the benchmark directly inside WSL.
# The Fabric test-network must already be up and the chaincode deployed.
#
# Usage:
#   cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/benchmark
#   bash run-wsl.sh
#
# Optional: override any env var before calling, e.g.
#   BENCH_WRITE_TRIALS=100 bash run-wsl.sh

set -euo pipefail

# ── Fabric crypto materials ────────────────────────────────────────────────────
# Points to the test-network organizations directory on the Windows filesystem.
FABRIC_SAMPLES_ORG_DIR="/mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples/test-network/organizations"

export CRYPTO_PATH="${FABRIC_SAMPLES_ORG_DIR}/peerOrganizations/org1.example.com"

# ── Fabric network connection ──────────────────────────────────────────────────
# Fabric peers run in Docker (via Docker Desktop + WSL2 integration).
# From inside WSL2, 'localhost' reaches Docker-published ports.
export PEER_ENDPOINT="${PEER_ENDPOINT:-localhost:7051}"
export GATEWAY_PEER="${GATEWAY_PEER:-peer0.org1.example.com}"
export MSP_ID="${MSP_ID:-Org1MSP}"
export CHANNEL_NAME="${CHANNEL_NAME:-mychannel}"
export CHAINCODE_NAME="${CHAINCODE_NAME:-arptracker}"

# ── Results directory ──────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/benchmark-results"
mkdir -p "$RESULTS_DIR"
export RESULTS_DIR

# ── Benchmark tuning (all overridable) ────────────────────────────────────────
export BENCH_WRITE_TRIALS="${BENCH_WRITE_TRIALS:-50}"
export BENCH_READ_TRIALS="${BENCH_READ_TRIALS:-100}"
export BENCH_READ_ALL_TRIALS="${BENCH_READ_ALL_TRIALS:-20}"
export BENCH_THROUGHPUT_COUNT="${BENCH_THROUGHPUT_COUNT:-100}"
export BENCH_CONCURRENCY="${BENCH_CONCURRENCY:-10}"
export BENCH_WARMUP="${BENCH_WARMUP:-5}"

# ── Pre-flight checks ──────────────────────────────────────────────────────────
echo "============================================================"
echo "  PRE-FLIGHT CHECKS"
echo "============================================================"

# Go installed?
if ! command -v go &>/dev/null; then
    echo "ERROR: 'go' not found. Install Go 1.21+ in WSL:"
    echo "  sudo apt-get update && sudo apt-get install -y golang-go"
    echo "  (or download from https://go.dev/dl/ for a newer version)"
    exit 1
fi
echo "✅  Go:          $(go version)"

# Crypto materials present?
if [[ ! -f "${CRYPTO_PATH}/users/User1@org1.example.com/msp/signcerts/cert.pem" ]]; then
    echo "ERROR: Crypto materials not found at:"
    echo "  ${CRYPTO_PATH}"
    echo ""
    echo "Is the Fabric test-network running?  Try:"
    echo "  cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples/test-network"
    echo "  ./network.sh up createChannel -c mychannel -ca"
    exit 1
fi
echo "✅  Crypto:      ${CRYPTO_PATH}"

# Peer reachable?
if ! nc -z -w2 localhost 7051 2>/dev/null; then
    echo "WARNING: Cannot reach peer at localhost:7051."
    echo "  Fabric may not be running, or Docker Desktop WSL2 integration"
    echo "  may need to be enabled."
    echo "  Continuing anyway — the benchmark will fail with a clear error."
fi

echo "✅  Peer:        ${PEER_ENDPOINT}"
echo "✅  Channel:     ${CHANNEL_NAME}  Chaincode: ${CHAINCODE_NAME}"
echo "✅  Results dir: ${RESULTS_DIR}"
echo ""

# ── Build ──────────────────────────────────────────────────────────────────────
echo "🔨  Building benchmark..."
cd "$SCRIPT_DIR"
go build -o benchmark-bin . 2>&1
echo "✅  Build complete."
echo ""

# ── Run ───────────────────────────────────────────────────────────────────────
echo "🚀  Running benchmark..."
echo ""
./benchmark-bin

echo ""
echo "Results written to: ${RESULTS_DIR}/"
ls -lh "${RESULTS_DIR}/" 2>/dev/null || true
