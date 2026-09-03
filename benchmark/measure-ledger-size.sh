#!/usr/bin/env bash
#
# Measure the on-chain size of the ARP binding registry directly from a running
# Fabric peer. Reports two distinct, independently verifiable figures:
#
#   1. World state (the registry)      - current IP->MAC bindings; what a paper
#                                         means by "how big is the registry".
#      a. logical KV bytes             - exact compact JSON as stored by the
#                                         chaincode (json.Marshal), recomputed
#                                         from the ledger contents.
#      b. stateLeveldb on-disk KB      - peer's state DB directory (also holds
#                                         HISTORY_ keys + LevelDB overhead).
#
#   2. Block store (chains) on-disk KB - the full transaction history. Grows with
#                                         the number of transactions ever
#                                         submitted, NOT with host count. Do not
#                                         quote this as the "registry size".
#
# Everything here is read-only and re-runnable against the live ledger.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FABRIC_TEST_NETWORK="${FABRIC_TEST_NETWORK:-"$ROOT/../fabric-samples/test-network"}"
CHANNEL_NAME="${CHANNEL_NAME:-mychannel}"
CHAINCODE_NAME="${CHAINCODE_NAME:-arptracker}"
PEER_CONTAINER="${PEER_CONTAINER:-peer0.org1.example.com}"
RESULTS_DIR="${RESULTS_DIR:-"$ROOT/benchmark/results"}"
LEDGER_PATH="${LEDGER_PATH:-/var/hyperledger/production/ledgersData}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$RESULTS_DIR/ledgersize_${RUN_ID}.txt"

mkdir -p "$RESULTS_DIR"
log() { printf '%s\n' "$*" | tee -a "$OUT"; }

[ -d "$FABRIC_TEST_NETWORK" ] || { echo "missing FABRIC_TEST_NETWORK=$FABRIC_TEST_NETWORK" >&2; exit 3; }

cd "$FABRIC_TEST_NETWORK"
export PATH="$PWD/../bin:$PATH"
export FABRIC_CFG_PATH="$PWD/../config/"
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE="$PWD/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt"
export CORE_PEER_MSPCONFIGPATH="$PWD/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
export CORE_PEER_ADDRESS="${CORE_PEER_ADDRESS:-localhost:7051}"

log "============================================================"
log "Blockchain-ARP ledger size measurement"
log "Run ID: $RUN_ID   Channel: $CHANNEL_NAME   Chaincode: $CHAINCODE_NAME"
log "============================================================"

ENTRIES_JSON="$(mktemp)"
trap 'rm -f "$ENTRIES_JSON"' EXIT
peer chaincode query -C "$CHANNEL_NAME" -n "$CHAINCODE_NAME" \
    -c '{"Args":["GetAllARPEntriesIncludingExpired"]}' > "$ENTRIES_JSON"

PYTHON_BIN="$(command -v python3 || command -v python)"
"$PYTHON_BIN" - "$ENTRIES_JSON" <<'PY' | tee -a "$OUT"
import json, sys, statistics
entries = json.loads(open(sys.argv[1], 'rb').read().rstrip(b'\n'))
n = len(entries)
# Field order matches the chaincode ARPEntry struct; json.dumps with compact
# separators reproduces Go's json.Marshal output byte-for-byte for ASCII values.
ORDER = ["ipAddress","macAddress","interface","hostname","timestamp",
         "entryType","state","recordedBy","isExpired","expiredAt"]
def compact(e): return json.dumps({k: e.get(k) for k in ORDER}, separators=(",",":"))
vb = [len(compact(e).encode()) for e in entries]
kb = [len(("ARP_"+e.get("ipAddress","")).encode()) for e in entries]
tv, tk = sum(vb), sum(kb)
print(f"current bindings (ARP_ keys):        {n}")
if n:
    print(f"per-binding value bytes (stored):    min={min(vb)} mean={statistics.mean(vb):.1f} max={max(vb)}")
    print(f"per-binding KV bytes (value+key):    mean={(tv+tk)/n:.1f}")
    print(f"world state logical (values):        {tv} B = {tv/1024:.1f} KB")
    print(f"world state logical (values+keys):   {tv+tk} B = {(tv+tk)/1024:.1f} KB")
PY

log ""
log "on-disk (container $PEER_CONTAINER):"
docker exec "$PEER_CONTAINER" sh -c "
  echo \"  world state (stateLeveldb): \$(du -sk $LEDGER_PATH/stateLeveldb | cut -f1) KB\"
  echo \"  block store (chains):       \$(du -sk $LEDGER_PATH/chains | cut -f1) KB\"
  echo \"  total ledgersData:          \$(du -sk $LEDGER_PATH | cut -f1) KB\"
" | tee -a "$OUT"

log ""
log "NOTE: 'block store (chains)' is the full transaction history and grows with"
log "the number of transactions ever submitted, not with the number of hosts."
log "The registry size is the world state figure."
log "Result: $OUT"
