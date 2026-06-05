package main

import (
	"strings"
	"testing"
	"time"
)

func TestPreventionModeTimeoutModel(t *testing.T) {
	timeout := 500 * time.Millisecond
	fabricLatency := 1200 * time.Millisecond
	if preventionResolutionSucceeds(timeout, fabricLatency) {
		t.Fatalf("prevention mode should fail when ledger confirmation exceeds ARP timeout")
	}
	if !detectionResolutionSucceeds() {
		t.Fatalf("detection mode should preserve normal ARP resolution")
	}
}

func preventionResolutionSucceeds(timeout, fabricLatency time.Duration) bool {
	return fabricLatency <= timeout
}

func detectionResolutionSucceeds() bool {
	return true
}

func TestParseIntList(t *testing.T) {
	values, err := parseIntList("3, 5,10")
	if err != nil {
		t.Fatalf("parseIntList failed: %v", err)
	}
	expected := []int{3, 5, 10}
	for i, value := range expected {
		if values[i] != value {
			t.Fatalf("expected %v, got %v", expected, values)
		}
	}
}

func TestExpandFlagArgsSupportsEqualsAndSeparatedValues(t *testing.T) {
	expanded := expandFlagArgs([]string{"--nodes", "3,5", "--trials=2"})
	joined := strings.Join(expanded, " ")
	if joined != "--nodes=3,5 --trials=2" {
		t.Fatalf("unexpected expanded flags: %q", joined)
	}
}

func TestDeterministicMACIsStableAndFormatted(t *testing.T) {
	first := deterministicMAC("latency", 3, 1)
	second := deterministicMAC("latency", 3, 1)
	if first != second {
		t.Fatalf("expected stable MAC, got %s and %s", first, second)
	}
	if !strings.HasPrefix(first, "be:bc:") || len(first) != len("be:bc:00:00:00:00") {
		t.Fatalf("unexpected MAC format: %s", first)
	}
}

func TestParseBenchmarkTimestamp(t *testing.T) {
	ts, ok := parseBenchmarkTimestamp("BENCHMARK_EVENT stage=node_cache_updated trial=abc ip=10.5.0.10 mac=aa ts=1780284834402")
	if !ok || ts != 1780284834402 {
		t.Fatalf("unexpected timestamp parse result: ts=%d ok=%t", ts, ok)
	}
}

func TestActiveNodeLimitMatchesSimulatedLAN(t *testing.T) {
	if activeNodeLimit(10, 7) != 7 {
		t.Fatalf("expected requested node count to cap at available nodes")
	}
	if activeNodeLimit(2, 7) != 2 {
		t.Fatalf("expected requested node count below cap to be preserved")
	}
	if activeNodeLimit(2, 0) != 0 {
		t.Fatalf("expected zero when no LAN nodes are running")
	}
}

func TestNodeOrdinal(t *testing.T) {
	if nodeOrdinal("lan-node-10") != 10 {
		t.Fatalf("expected node ordinal 10")
	}
}
