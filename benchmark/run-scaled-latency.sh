#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BENCHMARK_DIR="$ROOT/benchmark"
FABRIC_TEST_NETWORK="${FABRIC_TEST_NETWORK:-"$ROOT/../fabric-samples/test-network"}"
TRIALS="${TRIALS:-30}"
NODE_COUNTS="${NODE_COUNTS:-5,10,15,20}"
WAIT_SECONDS="${WAIT_SECONDS:-30}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${RESULTS_DIR:-"$BENCHMARK_DIR/results"}"
COMBINED="$RESULTS_DIR/scaled_latency_real_${RUN_ID}.csv"
MAX_NODE_COUNT="${MAX_NODE_COUNT:-20}"

mkdir -p "$RESULTS_DIR"

export CRYPTO_PATH="${CRYPTO_PATH:-"$FABRIC_TEST_NETWORK/organizations/peerOrganizations/org1.example.com"}"
export PEER_ENDPOINT="${PEER_ENDPOINT:-localhost:7051}"
export GATEWAY_PEER="${GATEWAY_PEER:-peer0.org1.example.com}"
export BENCH_EVENT_TIMEOUT="${BENCH_EVENT_TIMEOUT:-30s}"
export RESULTS_DIR

compose() {
    docker compose -f "$ROOT/docker-compose.yml" -f "$ROOT/docker-compose.scale.yml" "$@"
}

node_services_for_count() {
    local count="$1"
    local services=()
    local i
    for i in $(seq 1 "$count"); do
        services+=("node${i}")
    done
    printf '%s\n' "${services[@]}"
}

extra_node_services_after_count() {
    local count="$1"
    local services=()
    local i
    if [ "$count" -ge "$MAX_NODE_COUNT" ]; then
        return 0
    fi
    for i in $(seq $((count + 1)) "$MAX_NODE_COUNT"); do
        services+=("node${i}")
    done
    printf '%s\n' "${services[@]}"
}

running_node_count() {
    docker ps --format '{{.Names}}' | grep -E '^lan-node-[0-9]+$' | wc -l
}

append_latest_latency_csv() {
    local csv
    csv="$(ls -t "$RESULTS_DIR"/latency_*.csv | head -1)"
    if [ ! -f "$COMBINED" ]; then
        cp "$csv" "$COMBINED"
    else
        tail -n +2 "$csv" >> "$COMBINED"
    fi
}

echo "============================================================"
echo "Scaled latency benchmark with real LAN node counts"
echo "Run ID: $RUN_ID"
echo "Node counts: $NODE_COUNTS"
echo "Trials per count: $TRIALS"
echo "Combined CSV: $COMBINED"
echo "============================================================"

IFS=',' read -ra counts <<< "$NODE_COUNTS"
for count in "${counts[@]}"; do
    count="$(echo "$count" | xargs)"
    echo
    echo "Configuring real topology with $count LAN node containers..."

    mapfile -t keep < <(node_services_for_count "$count")
    mapfile -t stop < <(extra_node_services_after_count "$count")

    compose up -d dashboard router "${keep[@]}"
    if [ "${#stop[@]}" -gt 0 ]; then
        compose stop "${stop[@]}" >/dev/null
    fi

    echo "Waiting ${WAIT_SECONDS}s for node subscriptions..."
    sleep "$WAIT_SECONDS"

    actual="$(running_node_count)"
    if [ "$actual" != "$count" ]; then
        echo "Expected $count running LAN nodes, found $actual" >&2
        docker ps --format 'table {{.Names}}\t{{.Status}}' | grep -E 'lan-node|blockchain-router|blockchain-dashboard' || true
        exit 1
    fi

    docker ps --format 'table {{.Names}}\t{{.Status}}' | grep -E 'lan-node|blockchain-router|blockchain-dashboard' || true

    echo "Running latency benchmark for $count real nodes..."
    (
        cd "$BENCHMARK_DIR"
        go run . latency --nodes "$count" --trials "$TRIALS"
    )
    append_latest_latency_csv
done

echo
echo "PASS: scaled latency benchmark complete"
echo "Combined CSV: $COMBINED"
