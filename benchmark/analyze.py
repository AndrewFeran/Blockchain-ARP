#!/usr/bin/env python3
"""Generate paper figures from benchmark CSV output."""

from __future__ import annotations

import csv
from collections import defaultdict
from pathlib import Path
from statistics import mean, median
from math import ceil

import matplotlib.pyplot as plt


ROOT = Path(__file__).resolve().parent
RESULTS = ROOT / "results"
FIGURES = ROOT / "figures"
RESULTS_README = RESULTS / "README.md"


def latest_file(patterns: list[str]) -> Path | None:
    files: list[Path] = []
    for pattern in patterns:
        files.extend(RESULTS.glob(pattern))
    if not files:
        return None
    return max(files, key=lambda path: path.name)


def read_latest(prefix: str) -> list[dict[str, str]]:
    path = latest_file([f"{prefix}_*.csv"])
    if path is None:
        return []
    with path.open(newline="") as handle:
        return list(csv.DictReader(handle))


def read_latest_latency() -> list[dict[str, str]]:
    scaled = latest_file(["scaled_latency_real_*.csv", "scaled_latency_*.csv"])
    if scaled is not None:
        with scaled.open(newline="") as handle:
            return list(csv.DictReader(handle))
    return read_latest("latency")


def latest_latency_path() -> Path | None:
    return latest_file(["scaled_latency_real_*.csv", "scaled_latency_*.csv", "latency_*.csv"])


def is_real_scaled_latency(path: Path | None) -> bool:
    return path is not None and path.name.startswith(("scaled_latency_real_", "scaled_latency_"))


def display_path(path: Path | None) -> str:
    if path is None:
        return "not present in this result set"
    return path.name


def grouped_mean(rows: list[dict[str, str]], key: str, value: str) -> tuple[list[int], list[float]]:
    groups: dict[int, list[float]] = defaultdict(list)
    for row in rows:
        groups[int(float(row[key]))].append(float(row[value]))
    xs = sorted(groups)
    ys = [mean(groups[x]) for x in xs]
    return xs, ys


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    sorted_values = sorted(values)
    index = ceil((p / 100) * len(sorted_values)) - 1
    index = max(0, min(index, len(sorted_values) - 1))
    return sorted_values[index]


def grouped_summary(rows: list[dict[str, str]], key: str, value: str) -> list[dict[str, float | int]]:
    groups: dict[int, list[float]] = defaultdict(list)
    for row in rows:
        groups[int(float(row[key]))].append(float(row[value]))
    return [
        {
            "group": group,
            "count": len(values),
            "mean": mean(values),
            "median": median(values),
            "p95": percentile(values, 95),
            "p99": percentile(values, 99),
        }
        for group, values in sorted(groups.items())
    ]


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


def save_latency_summary(rows: list[dict[str, str]], path: Path) -> None:
    if not rows:
        return

    summaries = grouped_summary(rows, "node_count", "detection_latency_ms")
    xs = [int(row["group"]) for row in summaries]
    means = [float(row["mean"]) for row in summaries]
    medians = [float(row["median"]) for row in summaries]
    p95s = [float(row["p95"]) for row in summaries]

    plt.figure()
    plt.plot(xs, means, marker="o", color="0.15", label="Mean")
    plt.plot(xs, medians, marker="s", color="0.45", linestyle="--", label="Median")
    plt.plot(xs, p95s, marker="^", color="0.25", linestyle=":", label="P95")
    plt.xlabel("Node count")
    plt.ylabel("Detection latency (ms)")
    plt.legend(frameon=False)
    plt.savefig(path)
    plt.close()


def save_latency_distribution(rows: list[dict[str, str]], path: Path) -> None:
    if not rows:
        return

    groups: dict[int, list[float]] = defaultdict(list)
    for row in rows:
        groups[int(float(row["node_count"]))].append(float(row["detection_latency_ms"]))

    xs = sorted(groups)
    plt.figure(figsize=(5.6, 3.2))
    for x in xs:
        values = sorted(groups[x])
        offsets = [((i % 7) - 3) * 0.025 for i in range(len(values))]
        plt.scatter([x + offset for offset in offsets], values, s=13, color="0.25", alpha=0.62)
        plt.plot([x - 0.18, x + 0.18], [median(values), median(values)], color="0.05", linewidth=2.0)
        plt.plot([x - 0.14, x + 0.14], [percentile(values, 95), percentile(values, 95)], color="0.45", linewidth=1.5, linestyle=":")

    plt.xticks(xs, [str(x) for x in xs])
    plt.xlabel("Node count")
    plt.ylabel("Detection latency (ms)")
    plt.title("Latency samples with median and P95 markers")
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


def write_results_readme(latency: list[dict[str, str]]) -> None:
    RESULTS.mkdir(parents=True, exist_ok=True)
    latency_path = latest_latency_path()
    e2e_path = latest_file(["e2e_*.txt"])
    throughput_path = latest_file(["throughput_*.csv"])
    coldstart_path = latest_file(["coldstart_*.csv"])
    baseline_path = latest_file(["baseline_*.csv"])

    lines = [
        "# Benchmark Results",
        "",
        "This file is generated by `benchmark/analyze.py` from the latest files in this directory.",
        "It only reports metrics whose source CSV/log files are present in `benchmark/results/`.",
        "",
        "## Latest Files",
        "",
        "| Result | File |",
        "| --- | --- |",
        f"| Latency source | `{display_path(latency_path)}` |",
        f"| Throughput source | `{display_path(throughput_path)}` |",
        f"| Cold-start source | `{display_path(coldstart_path)}` |",
        f"| Baseline source | `{display_path(baseline_path)}` |",
        f"| End-to-end proof | `{display_path(e2e_path)}` |",
        "",
    ]

    missing = [
        name
        for name, path in [
            ("throughput", throughput_path),
            ("cold-start", coldstart_path),
            ("baseline", baseline_path),
        ]
        if path is None
    ]
    if missing:
        lines.extend(
            [
                "## Missing Result Types",
                "",
                "The current kept result set does not include source CSVs for: "
                + ", ".join(missing)
                + ".",
                "Run `./benchmark/run-reproducible.sh full` or the individual benchmark commands in `benchmark/README.md` to generate them.",
                "",
            ]
        )

    if latency:
        scaled_latency = is_real_scaled_latency(latency_path)
        heading = "Real Scaled Latency Summary" if scaled_latency else "Configured Latency Summary"
        group_label = "Running LAN nodes" if scaled_latency else "Configured node count"
        lines.extend(
            [
                f"## {heading}",
                "",
                f"| {group_label} | Trials | Mean | Median | P95 | P99 | Nodes observed |",
                "| --- | ---: | ---: | ---: | ---: | ---: | --- |",
            ]
        )
        summaries = grouped_summary(latency, "node_count", "detection_latency_ms")
        for summary in summaries:
            node_count = int(summary["group"])
            observed = sorted(
                {
                    int(row["nodes_observed"])
                    for row in latency
                    if int(float(row["node_count"])) == node_count and row.get("nodes_observed")
                }
            )
            observed_text = ", ".join(str(value) for value in observed) if observed else "n/a"
            lines.append(
                "| {group} | {count} | {mean:.2f} ms | {median:.2f} ms | {p95:.2f} ms | {p99:.2f} ms | {observed} |".format(
                    group=node_count,
                    count=int(summary["count"]),
                    mean=float(summary["mean"]),
                    median=float(summary["median"]),
                    p95=float(summary["p95"]),
                    p99=float(summary["p99"]),
                    observed=observed_text,
                )
            )
        lines.extend(
            [
                "",
                "Notes:",
                "",
                "- The latency figure plots mean, median, and P95 so bimodal runs are visible.",
                "- Non-monotonic node-count results can occur when Fabric submission, Docker log timing, and node event fan-out batch differently between runs.",
                "- `nodes_observed` confirms how many running node containers emitted cache-update evidence for each topology.",
            ]
        )
        if not scaled_latency:
            lines.append("- This is the standard full-run latency benchmark. Use `benchmark/run-scaled-latency.sh` for measurements where the actual running LAN-node count changes.")
        lines.append("")

    if e2e_path is not None:
        text = e2e_path.read_text(errors="replace")
        verdict = "PASS" if "PASS:" in text else "CHECK LOG"
        lines.extend(
            [
                "## End-to-End Verification",
                "",
                f"Latest verdict: **{verdict}**",
                "",
                f"Source log: `{e2e_path.name}`",
                "",
            ]
        )

    figures = [
        ("Latency mean curve", FIGURES / "latency_vs_nodes.png", latency_path),
        ("Latency summary curve", FIGURES / "latency_summary.png", latency_path),
        ("Latency sample distribution", FIGURES / "latency_distribution.png", latency_path),
        ("Throughput curve", FIGURES / "throughput_curve.png", throughput_path),
        ("Cold-start curve", FIGURES / "coldstart_vs_ledger.png", coldstart_path),
        ("Protection-rate chart", FIGURES / "protection_rate.png", baseline_path),
    ]
    present_figures = [(label, path, source) for label, path, source in figures if path.exists()]
    if present_figures:
        lines.extend(
            [
                "## Figures",
                "",
                "| Figure | File | Source status |",
                "| --- | --- | --- |",
            ]
        )
        for label, path, source in present_figures:
            status = "current source present" if source is not None else "source CSV not present in this result set"
            lines.append(f"| {label} | `../figures/{path.name}` | {status} |")
        lines.append("")

    RESULTS_README.write_text("\n".join(lines), encoding="utf-8")


def main() -> None:
    FIGURES.mkdir(parents=True, exist_ok=True)
    style()

    latency = read_latest_latency()
    xs, ys = grouped_mean(latency, "node_count", "detection_latency_ms")
    save_line(xs, ys, "Node count", "Latency (ms)", FIGURES / "latency_vs_nodes.png")
    save_latency_summary(latency, FIGURES / "latency_summary.png")
    save_latency_distribution(latency, FIGURES / "latency_distribution.png")

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

    write_results_readme(latency)
    print(f"Figures written to {FIGURES}")
    print(f"Results summary written to {RESULTS_README}")


if __name__ == "__main__":
    main()
