# Blockchain-ARP Benchmark Suite

This directory contains the reproducible measurements used to evaluate the
Blockchain-ARP prototype.

## What Is Measured

The suite writes timestamped CSV files to `benchmark/results/`, updates
`benchmark/results/README.md`, and writes figures to `benchmark/figures/`.

| Metric | Output CSV | Figure | Meaning |
| --- | --- | --- | --- |
| Detection latency | `latency_*.csv` | `latency_vs_nodes.png` | Time from benchmark submission start until the expected node containers emit `node_cache_updated` for the same trial ID. |
| Real scaled latency | `scaled_latency_real_*.csv` | `latency_vs_nodes.png`, `latency_summary.png` | Same detection-latency metric, but the script changes the actual number of running LAN node containers before each topology. |
| Throughput under attack | `throughput_*.csv` | `throughput_curve.png` | Detection latency while events are submitted at increasing rates. |
| Cold-start sync | `coldstart_*.csv` | `coldstart_vs_ledger.png` | Ledger query time plus a conservative per-entry ARP installation estimate. Live nodes also emit `COLD_START_COMPLETE entries=N elapsed_ms=M`. |
| Baseline comparison | `baseline_*.csv` | `protection_rate.png` | Baseline poison-time assumption compared with protected-path node confirmation. |
| End-to-end verification | `e2e_*.txt` | n/a | Live spoof attempt proof: dashboard spoof count increases and ledger mapping remains legitimate. |

## Reproducible Run

From WSL, with Fabric and the simulated LAN already running:

```bash
cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP
./benchmark/run-reproducible.sh
```

By default this runs the `smoke` profile: one trial per metric, figure
generation, log collection, and live end-to-end spoof verification. It is meant
to prove that the full benchmark workflow is reproducible without taking a long
time.

For the full publication-sized run:

```bash
./benchmark/run-reproducible.sh full
```

or equivalently:

```bash
BENCHMARK_PROFILE=full ./benchmark/run-reproducible.sh
```

The script:

1. Checks required tools (`docker`, `go`, `python3` or `python`).
2. Verifies Fabric crypto material exists.
3. Verifies the required containers are running.
4. Runs benchmark package tests.
5. Runs either the smoke profile or full `go run . all` profile.
6. Regenerates the figures and `benchmark/results/README.md`.
7. Collects merged benchmark logs into `benchmark/results/raw_<timestamp>.log`.
8. Runs the live spoofing end-to-end verifier.
9. Writes a manifest to `benchmark/results/run_<timestamp>.txt`.

`MPLBACKEND=Agg` is set by default so figure generation works in headless WSL
or CI sessions.

Each benchmark command is wrapped with GNU `timeout`. Override the default:

```bash
BENCHMARK_COMMAND_TIMEOUT=90m ./benchmark/run-reproducible.sh full
```

The default Fabric path is:

```bash
../fabric-samples/test-network
```

Override it when needed:

```bash
FABRIC_TEST_NETWORK=/path/to/fabric-samples/test-network ./benchmark/run-reproducible.sh
```

## Direct Commands

Run all metrics:

```bash
cd benchmark
CRYPTO_PATH=../../fabric-samples/test-network/organizations/peerOrganizations/org1.example.com \
PEER_ENDPOINT=localhost:7051 \
GATEWAY_PEER=peer0.org1.example.com \
go run . all
```

Run individual metrics:

```bash
go run . latency --nodes 5,10,15,20 --trials 30
go run . throughput --max-rate 100
go run . coldstart --ledger-sizes 10,50,100,250 --trials 10
go run . baseline --trials 10
```

Run the real scaled LAN-node latency experiment:

```bash
cd /mnt/c/Users/Perky/OneDrive/Desktop/barp/Blockchain-ARP
TRIALS=30 WAIT_SECONDS=30 BENCH_EVENT_TIMEOUT=30s ./benchmark/run-scaled-latency.sh
```

This starts/stops real `lan-node-*` containers so the 5, 10, 15, and 20-node
measurements correspond to actual running container counts. The extra node
services are defined in `docker-compose.scale.yml`.

Regenerate figures and the latest results summary from the latest CSVs:

```bash
python3 analyze.py
```

The generated results summary is:

```text
benchmark/results/README.md
```

Run the live spoofing verification:

```bash
../scripts/verify-e2e.sh
```

## CSV Columns

`latency_*.csv`

- `node_count`: requested node-count configuration.
- `trial`: trial number within that configuration.
- `trial_id`: unique ID carried through Fabric events and logs.
- `ip`, `mac`: mapping submitted for the trial.
- `submit_latency_ms`: Fabric transaction submit duration.
- `detection_latency_ms`: elapsed time until node cache-update evidence appears.
- `nodes_observed`: number of node containers that emitted cache-update evidence.

`throughput_*.csv`

- Same latency columns, grouped by `rate_per_sec`.

`coldstart_*.csv`

- `ledger_size`: requested prepopulation target.
- `query_elapsed_ms`: `GetAllARPEntries` evaluation time.
- `simulated_install_elapsed_ms`: query time plus a conservative per-entry install estimate.
- `entries`: active entries returned by the ledger.

`baseline_*.csv`

- `baseline_poison_ms`: configured baseline poisoning assumption.
- `protected_submit_ms`: Fabric submit duration in protected mode.
- `protected_detection_ms`: node-observed protected-path latency.
- `protected_rejected`: `1` when protected-path node evidence was observed.

## Latest Results

The latest generated benchmark summary is:

```text
benchmark/results/README.md
```

That file is regenerated by `python3 analyze.py` and records the exact source
CSVs/logs used for the current summary. Keep the numeric result tables there so
the repository has one benchmark-results source of truth.

## Reproducibility Notes

- The base simulated LAN starts three nodes. For real scaling experiments, use
  `benchmark/run-scaled-latency.sh`, which brings the topology to 5, 10, 15,
  and 20 running node containers before measuring each point.
- Detection latency depends on Docker log timestamps emitted by the running
  router and node containers. Do not prune logs mid-run.
- `BENCH_EVENT_TIMEOUT` controls how long each trial waits for node evidence.
  Default: `20s`.
- `BENCH_BASELINE_POISON_MS` controls the baseline poison-time assumption.
  Default: `50`.
- `BENCH_ARP_INSTALL_MS` controls the conservative per-entry cold-start install
  estimate. Default: `2`.
