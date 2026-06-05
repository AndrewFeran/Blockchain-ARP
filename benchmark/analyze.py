#!/usr/bin/env python3
"""Generate paper figures from benchmark CSV output."""

from __future__ import annotations

import csv
from collections import defaultdict
from pathlib import Path
from statistics import mean

import matplotlib.pyplot as plt


ROOT = Path(__file__).resolve().parent
RESULTS = ROOT / "results"
FIGURES = ROOT / "figures"


def read_latest(prefix: str) -> list[dict[str, str]]:
    files = sorted(RESULTS.glob(f"{prefix}_*.csv"))
    if not files:
        return []
    with files[-1].open(newline="") as handle:
        return list(csv.DictReader(handle))


def read_latest_latency() -> list[dict[str, str]]:
    scaled = sorted(RESULTS.glob("scaled_latency_real_*.csv"))
    if scaled:
        with scaled[-1].open(newline="") as handle:
            return list(csv.DictReader(handle))
    return read_latest("latency")


def grouped_mean(rows: list[dict[str, str]], key: str, value: str) -> tuple[list[int], list[float]]:
    groups: dict[int, list[float]] = defaultdict(list)
    for row in rows:
        groups[int(float(row[key]))].append(float(row[value]))
    xs = sorted(groups)
    ys = [mean(groups[x]) for x in xs]
    return xs, ys


def style() -> None:
    plt.rcParams.update(
        {
            "figure.figsize": (5.0, 3.0),
            "font.size": 9,
            "axes.grid": True,
            "grid.linestyle": ":",
            "grid.color": "0.75",
            "lines.linewidth": 1.5,
            "savefig.dpi": 300,
            "savefig.bbox": "tight",
        }
    )


def save_line(xs: list[int], ys: list[float], xlabel: str, ylabel: str, path: Path) -> None:
    if not xs:
        return
    plt.figure()
    plt.plot(xs, ys, marker="o", color="0.15")
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.savefig(path)
    plt.close()


def save_bar(xs: list[int | str], ys: list[float], xlabel: str, ylabel: str, path: Path) -> None:
    if not xs:
        return
    plt.figure()
    plt.bar([str(x) for x in xs], ys, color="0.35", edgecolor="0.05", hatch="//")
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.savefig(path)
    plt.close()


def main() -> None:
    FIGURES.mkdir(parents=True, exist_ok=True)
    style()

    latency = read_latest_latency()
    xs, ys = grouped_mean(latency, "node_count", "detection_latency_ms")
    save_line(xs, ys, "Node count", "Latency (ms)", FIGURES / "latency_vs_nodes.png")

    throughput = read_latest("throughput")
    xs, ys = grouped_mean(throughput, "rate_per_sec", "detection_latency_ms")
    save_line(xs, ys, "Events per second", "Latency (ms)", FIGURES / "throughput_curve.png")

    coldstart = read_latest("coldstart")
    xs, ys = grouped_mean(coldstart, "ledger_size", "simulated_install_elapsed_ms")
    save_bar(xs, ys, "Ledger entries", "Cold-start sync time (ms)", FIGURES / "coldstart_vs_ledger.png")

    baseline = read_latest("baseline")
    if baseline:
        baseline_rate = 0.0
        protected_rate = 100.0 * mean(float(row["protected_rejected"]) for row in baseline)
        save_bar(["Baseline", "Protected"], [baseline_rate, protected_rate], "Mode", "Protected nodes (%)", FIGURES / "protection_rate.png")

    print(f"Figures written to {FIGURES}")


if __name__ == "__main__":
    main()
