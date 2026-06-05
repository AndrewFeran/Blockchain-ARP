#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FABRIC_TEST_NETWORK="${FABRIC_TEST_NETWORK:-"$REPO_ROOT/../fabric-samples/test-network"}"
TARGET_IP="${TARGET_IP:-10.5.0.10}"
LEGIT_MAC="${LEGIT_MAC:-02:42:0a:05:00:0a}"
SPOOF_MAC="${SPOOF_MAC:-02:42:0a:05:00:0c}"
ROUTER_IP="${ROUTER_IP:-10.5.0.1}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:5000}"
RESULTS_DIR="${RESULTS_DIR:-"$REPO_ROOT/benchmark/results"}"
DISABLE_BACKGROUND_TRAFFIC="${DISABLE_BACKGROUND_TRAFFIC:-true}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
RESULT_FILE="$RESULTS_DIR/e2e_${RUN_ID}.txt"

mkdir -p "$RESULTS_DIR"

log() {
    printf '%s\n' "$*" | tee -a "$RESULT_FILE"
}

fail() {
    log "FAIL: $*"
    exit 1
}

stats_field() {
    local field="$1"
    curl -fsS "$DASHBOARD_URL/api/stats" |
        python3 -c "import json,sys; print(json.load(sys.stdin).get('$field', 0))"
}

current_mac() {
    local ip="$1"
    (
        cd "$FABRIC_TEST_NETWORK"
        export PATH="$PWD/../bin:$PATH"
        export FABRIC_CFG_PATH="$PWD/../config/"
        export CORE_PEER_TLS_ENABLED=true
        export CORE_PEER_LOCALMSPID=Org1MSP
        export CORE_PEER_TLS_ROOTCERT_FILE="$PWD/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt"
        export CORE_PEER_MSPCONFIGPATH="$PWD/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
        export CORE_PEER_ADDRESS=localhost:7051
        peer chaincode query -C mychannel -n arptracker -c "{\"Args\":[\"GetCurrentARPEntry\",\"$ip\"]}" 2>/dev/null
    ) | python3 -c 'import json,sys; print(json.load(sys.stdin).get("macAddress",""))'
}

wait_for_http() {
    local url="$1"
    local attempts="${2:-30}"
    for _ in $(seq 1 "$attempts"); do
        if curl -fsS "$url" >/dev/null; then
            return 0
        fi
        sleep 2
    done
    return 1
}

wait_for_current_mac() {
    local ip="$1"
    local expected="$2"
    local attempts="${3:-45}"
    local observed=""
    for _ in $(seq 1 "$attempts"); do
        observed="$(current_mac "$ip" || true)"
        if [ "$observed" = "$expected" ]; then
            return 0
        fi
        sleep 2
    done
    log "Observed current MAC for $ip: ${observed:-<none>}"
    return 1
}

wait_for_spoof_increment() {
    local before="$1"
    local attempts="${2:-45}"
    local now=""
    for _ in $(seq 1 "$attempts"); do
        now="$(stats_field spoofing || echo 0)"
        if [ "$now" -gt "$before" ]; then
            log "Spoofing counter increased: $before -> $now"
            return 0
        fi
        sleep 2
    done
    log "Spoofing counter did not increase from $before"
    return 1
}

log "============================================================"
log "Blockchain-ARP end-to-end verification"
log "Run ID: $RUN_ID"
log "============================================================"

cd "$REPO_ROOT"

log "Preparing quiet base topology..."
export DISABLE_BACKGROUND_TRAFFIC
docker compose -f docker-compose.yml -f docker-compose.scale.yml stop node4 node5 node6 node7 node8 node9 node10 >/dev/null 2>&1 || true
docker compose -f docker-compose.yml -f docker-compose.scale.yml up -d --build --force-recreate dashboard router node1 node2 node3 | tee -a "$RESULT_FILE"
log "Waiting for recreated services to settle..."
sleep 25

log "Checking container status..."
docker compose ps | tee -a "$RESULT_FILE"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' |
    grep -E 'peer0|orderer|arptracker|blockchain-|lan-node' | tee -a "$RESULT_FILE" || true

for name in peer0.org1.example.com peer0.org2.example.com orderer.example.com blockchain-router blockchain-dashboard lan-node-1 lan-node-2 lan-node-3; do
    docker ps --format '{{.Names}}' | grep -qx "$name" || fail "$name is not running"
done

wait_for_http "$DASHBOARD_URL/api/stats" 30 || fail "dashboard API is not reachable at $DASHBOARD_URL"

log "Establishing legitimate mapping $TARGET_IP -> $LEGIT_MAC..."
docker compose exec -T node1 sh -lc "arping -c 2 -I eth0 $ROUTER_IP >/dev/null 2>&1 || true"
wait_for_current_mac "$TARGET_IP" "$LEGIT_MAC" 60 || fail "ledger did not learn legitimate MAC $LEGIT_MAC for $TARGET_IP"
log "Ledger learned legitimate mapping: $TARGET_IP -> $LEGIT_MAC"

before_spoof="$(stats_field spoofing)"
before_total="$(stats_field total)"
log "Dashboard before spoof: total=$before_total spoofing=$before_spoof"

log "Launching spoof from node3: $TARGET_IP -> $SPOOF_MAC"
docker compose exec -T node3 /bin/bash /app/spoof-attack.sh "$TARGET_IP" "$ROUTER_IP" | tee -a "$RESULT_FILE"

wait_for_spoof_increment "$before_spoof" 60 || fail "dashboard did not record a spoofing event"

after_total="$(stats_field total)"
after_spoof="$(stats_field spoofing)"
log "Dashboard after spoof: total=$after_total spoofing=$after_spoof"

protected_mac="$(current_mac "$TARGET_IP")"
if [ "$protected_mac" != "$LEGIT_MAC" ]; then
    fail "ledger current MAC changed to $protected_mac; expected protected MAC $LEGIT_MAC"
fi
log "Ledger protection verified: $TARGET_IP still maps to $protected_mac"

log "Recent dashboard events:"
curl -fsS "$DASHBOARD_URL/api/events?limit=5" | tee -a "$RESULT_FILE"
log

log "Recent router/node proof logs:"
docker compose logs --tail=160 router node1 node2 node3 |
    grep -E 'SPOOFING DETECTED|BENCHMARK_EVENT|Recorded to blockchain|POST /api/event' |
    tail -n 80 | tee -a "$RESULT_FILE" || true

log "PASS: end-to-end spoof detection and ledger protection verified"
log "Result file: $RESULT_FILE"
