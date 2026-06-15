# Blockchain-ARP

Blockchain-ARP is a Hyperledger Fabric prototype for detecting ARP spoofing on a simulated LAN. The router container is the trust anchor: it observes real ARP traffic on the Docker bridge network, records authoritative IP-to-MAC mappings on Fabric, and forwards events to the dashboard. LAN node containers subscribe to Fabric events and keep their ARP caches synchronized with the ledger.

## What This Repo Contains

- **Demo runtime**
  - `docker-compose.yml` starts the dashboard, router, and base three LAN nodes.
  - `docker-compose.scale.yml` adds `node4` through `node20` for real scaling experiments.
  - `scripts/start-simulated-lan.sh` and `scripts/stop-simulated-lan.sh` manage the simulated LAN.

- **Core implementation**
  - `chaincode/` contains the Fabric smart contract.
  - `router/` captures ARP traffic and writes events to Fabric.
  - `node/` performs cold-start sync, subscribes to Fabric events, and manages ARP cache entries.
  - `dashboard/` serves the web panel and event API.

- **Data collection and findings**
  - `benchmark/benchmark.go` runs latency, throughput, cold-start, and baseline measurements.
  - `benchmark/run-reproducible.sh` runs a reproducible smoke or full benchmark workflow.
  - `benchmark/run-scaled-latency.sh` changes the actual running LAN-node topology and measures 5, 10, 15, and 20 real containers.
  - `scripts/verify-e2e.sh` proves live spoof detection and ledger protection.

Generated benchmark CSVs/logs/figures live under `benchmark/results/` and
`benchmark/figures/`. Bulky generated artifacts are ignored by git, while
`benchmark/results/README.md` is kept as the current results index.

## Requirements

- WSL2 or Linux
- Docker with Compose v2
- Go
- Python 3
- Hyperledger Fabric test-network checked out at `../fabric-samples/test-network`

The commands below assume this repo and `fabric-samples` are siblings:

```text
barp/
  Blockchain-ARP/
  fabric-samples/
```

## Start Fabric

From WSL:

```bash
cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples/test-network
./network.sh up createChannel -ca -c mychannel
./network.sh deployCC -ccn arptracker \
  -ccp /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP/chaincode \
  -ccl go \
  -c mychannel
```

## Start The Simulated LAN

Base three-node demo:

```bash
cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP
./scripts/start-simulated-lan.sh
```

Twenty-node topology:

```bash
NODE_COUNT=20 ./scripts/start-simulated-lan.sh
```

Dashboard:

```text
http://localhost:5000
```

## Verify Spoof Detection

Run the end-to-end verifier:

```bash
./scripts/verify-e2e.sh
```

It recreates a quiet three-node demo topology, establishes a legitimate mapping, launches a spoof from `lan-node-3`, checks that the dashboard records a spoofing event, and verifies that the ledger keeps the legitimate MAC. The verifier sets `DISABLE_BACKGROUND_TRAFFIC=true` so the proof is not delayed by ordinary generated ARP traffic; normal demo runs keep background traffic enabled.

Manual spoof from `lan-node-3`:

```bash
docker compose exec node3 /bin/bash /app/spoof-attack.sh 10.5.0.10 10.5.0.1
```

## Run Benchmarks

Fast reproducibility check:

```bash
./benchmark/run-reproducible.sh
```

Full publication-sized workflow:

```bash
./benchmark/run-reproducible.sh full
```

Real scaled LAN-node latency experiment:

```bash
TRIALS=30 WAIT_SECONDS=30 BENCH_EVENT_TIMEOUT=30s ./benchmark/run-scaled-latency.sh
```

Benchmark methodology, commands, CSV schemas, and reproducibility notes:

```text
benchmark/README.md
```

Latest generated benchmark results:

```text
benchmark/results/README.md
```

## Stop

Stop the simulated LAN:

```bash
./scripts/stop-simulated-lan.sh
```

Stop Fabric:

```bash
cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/fabric-samples/test-network
./network.sh down
```
