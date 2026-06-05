#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BENCHMARK_DIR="$ROOT/benchmark"
RESULTS_DIR="${RESULTS_DIR:-"$BENCHMARK_DIR/results"}"
FIGURES_DIR="$BENCHMARK_DIR/figures"
FABRIC_TEST_NETWORK="${FABRIC_TEST_NETWORK:-"$ROOT/../fabric-samples/test-network"}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
MANIFEST="$RESULTS_DIR/run_${RUN_ID}.txt"
PROFILE="${BENCHMARK_PROFILE:-${1:-smoke}}"
COMMAND_TIMEOUT="${BENCHMARK_COMMAND_TIMEOUT:-45m}"

mkdir -p "$RESULTS_DIR" "$FIGURES_DIR"

log() {
    printf '%s\n' "$*" | tee -a "$MANIFEST"
}

fail() {
    log "FAIL: $*"
    exit 1
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_container() {
    docker ps --format '{{.Names}}' | grep -qx "$1" || fail "container is not running: $1"
}

collect_benchmark_logs() {
    local out="$RESULTS_DIR/raw_$(date -u +%Y%m%dT%H%M%SZ).log"
    local tmp
    local containers=(
        blockchain-router
        peer0.org1.example.com
        peer0.org2.example.com
    )

    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' RETURN

    while IFS= read -r node_container; do
        containers+=("$node_container")
    done < <(docker ps -a --format '{{.Names}}' | grep -E '^lan-node-[0-9]+$' | sort -V)

    for container in "${containers[@]}"; do
        if docker ps -a --format '{{.Names}}' | grep -Fxq "$container"; then
            docker logs "$container" 2>&1 |
                grep 'BENCHMARK_EVENT' |
                sed "s/^/[$container] /" >> "$tmp" || true
        fi
    done

    sort -t= -k6,6n "$tmp" > "$out"
    echo "wrote $out"
}

export CRYPTO_PATH="${CRYPTO_PATH:-"$FABRIC_TEST_NETWORK/organizations/peerOrganizations/org1.example.com"}"
export PEER_ENDPOINT="${PEER_ENDPOINT:-localhost:7051}"
export GATEWAY_PEER="${GATEWAY_PEER:-peer0.org1.example.com}"
export MSP_ID="${MSP_ID:-Org1MSP}"
export CHANNEL_NAME="${CHANNEL_NAME:-mychannel}"
export CHAINCODE_NAME="${CHAINCODE_NAME:-arptracker}"
export BENCH_EVENT_TIMEOUT="${BENCH_EVENT_TIMEOUT:-20s}"
export RESULTS_DIR
export MPLBACKEND="${MPLBACKEND:-Agg}"

log "============================================================"
log "Blockchain-ARP reproducible benchmark run"
log "Run ID: $RUN_ID"
log "============================================================"
log "Repository: $ROOT"
log "Fabric test network: $FABRIC_TEST_NETWORK"
log "Results directory: $RESULTS_DIR"
log "Figures directory: $FIGURES_DIR"
log "Peer endpoint: $PEER_ENDPOINT"
log "Gateway peer: $GATEWAY_PEER"
log "Channel: $CHANNEL_NAME"
log "Chaincode: $CHAINCODE_NAME"
log "Benchmark event timeout: $BENCH_EVENT_TIMEOUT"
log "Matplotlib backend: $MPLBACKEND"
log "Benchmark profile: $PROFILE"
log "Command timeout: $COMMAND_TIMEOUT"
log

require_cmd docker
require_cmd go
require_cmd timeout
if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN=python3
elif command -v python >/dev/null 2>&1; then
    PYTHON_BIN=python
else
    fail "missing required command: python3 or python"
fi
log "Python: $($PYTHON_BIN --version 2>&1)"
log "Go: $(go version)"
log "Docker: $(docker --version)"
log

[ -f "$CRYPTO_PATH/users/User1@org1.example.com/msp/signcerts/cert.pem" ] ||
    fail "missing Org1 user certificate under CRYPTO_PATH=$CRYPTO_PATH"
[ -d "$CRYPTO_PATH/users/User1@org1.example.com/msp/keystore" ] ||
    fail "missing Org1 user private key directory under CRYPTO_PATH=$CRYPTO_PATH"
[ -f "$CRYPTO_PATH/peers/peer0.org1.example.com/tls/ca.crt" ] ||
    fail "missing Org1 peer TLS CA under CRYPTO_PATH=$CRYPTO_PATH"

for container in \
    peer0.org1.example.com \
    peer0.org2.example.com \
    orderer.example.com \
    blockchain-router \
    blockchain-dashboard \
    lan-node-1 \
    lan-node-2 \
    lan-node-3; do
    require_container "$container"
done

log "Container snapshot:"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' |
    grep -E 'peer0|orderer|arptracker|blockchain-|lan-node' | tee -a "$MANIFEST" || true
log

log "Running benchmark package tests..."
(
    cd "$BENCHMARK_DIR"
    go test -count=1 ./...
) | tee -a "$MANIFEST"
log

case "$PROFILE" in
    smoke)
        log "Running smoke benchmark suite..."
        (
            cd "$BENCHMARK_DIR"
            timeout "$COMMAND_TIMEOUT" go run . latency --nodes 3 --trials 1
            timeout "$COMMAND_TIMEOUT" go run . throughput --max-rate 1 --step 1 --events 1
            timeout "$COMMAND_TIMEOUT" go run . coldstart --ledger-sizes 10 --trials 1
            timeout "$COMMAND_TIMEOUT" go run . baseline --trials 1
            "$PYTHON_BIN" analyze.py
        ) 2>&1 | tee -a "$MANIFEST"
        ;;
    full)
        log "Running full benchmark suite..."
        (
            cd "$BENCHMARK_DIR"
            timeout "$COMMAND_TIMEOUT" go run . all
        ) 2>&1 | tee -a "$MANIFEST"
        ;;
    *)
        fail "unknown benchmark profile: $PROFILE (expected smoke or full)"
        ;;
esac
log

log "Collecting benchmark logs..."
collect_benchmark_logs | tee -a "$MANIFEST"
log

log "Running end-to-end spoof verification..."
(
    cd "$ROOT"
    FABRIC_TEST_NETWORK="$FABRIC_TEST_NETWORK" ./scripts/verify-e2e.sh
) | tee -a "$MANIFEST"
log

log "Latest result artifacts:"
find "$RESULTS_DIR" -maxdepth 1 -type f -printf '%TY-%Tm-%Td %TH:%TM:%TS %p\n' |
    sort |
    tail -n 12 | tee -a "$MANIFEST"
log

log "Figure artifacts:"
find "$FIGURES_DIR" -maxdepth 1 -type f -printf '%p\n' | sort | tee -a "$MANIFEST"
log

log "PASS: reproducible benchmark run completed"
log "Manifest: $MANIFEST"
