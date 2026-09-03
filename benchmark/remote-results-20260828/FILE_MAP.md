# File map for the 2026-08-27/28 full-suite run

Run ID: `20260827T202430Z`. Node counts 3, 5, 7, 9 ran in that order, each with
`--trials 500` (throughput used the default `--max-rate 100`).

| Node count | latency | throughput | coldstart | baseline |
| --- | --- | --- | --- | --- |
| 3 | latency_20260827T202500Z.csv | throughput_20260827T224459Z.csv | coldstart_20260827T224830Z.csv | baseline_20260827T230329Z.csv |
| 5 | latency_20260828T012419Z.csv | throughput_20260828T034420Z.csv | coldstart_20260828T034755Z.csv | baseline_20260828T040125Z.csv |
| 7 | latency_20260828T062230Z.csv | throughput_20260828T084307Z.csv | coldstart_20260828T084653Z.csv | baseline_20260828T090059Z.csv |
| 9 | latency_20260828T112252Z.csv | throughput_20260828T134435Z.csv | coldstart_20260828T134850Z.csv | baseline_20260828T140204Z.csv |

`full_<metric>_20260827T202430Z.csv` = all 4 node counts concatenated (in the order above)
for that metric.

`scaled_latency_real_20260828T202430Z.csv` = a copy of `full_latency_...csv`, renamed so
`analyze.py`/the results README pick it up as the "real scaled latency" source instead of
the stale June scaled-latency file.

## Known bug affecting `latency` and `baseline` at trials > 255

`benchmark.go` builds the ARP IP address from the raw trial number as the last IPv4
octet (`10.250.<nodes>.<trial>` for latency, `10.252.0.<trial>` for baseline) with no
wraparound. Once `trial` passes 255 the octet is out of range (e.g. `10.250.3.256`),
the resulting observation is never detected by the LAN nodes, and every trial from
256 onward times out. In every one of the 8 files above (4 node counts x latency/baseline)
this produces **exactly 245/500 (49%) timeouts, always starting at trial 256** — confirmed
by inspecting all 8 files. `coldstart` and `throughput` are not affected (their IP math
already wraps/stays under 256).

Practical effect: only the first 255 trials per node count are valid latency/baseline
samples. The mean/P95/P99 in the generated `results/README.md` are skewed by the
timed-out tail and should not be used as-is.
