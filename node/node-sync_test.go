package main

import (
	"errors"
	"testing"
	"time"
)

func withNodeTestHooks(t *testing.T, localIP string) (*[]string, *[]string) {
	t.Helper()
	originalLocalIPLookup := localIPLookup
	originalARPReplace := arpReplace
	originalARPDelete := arpDelete
	originalPreventionMode := preventionMode
	originalPreventionTimeout := preventionTimeout
	originalPendingEntries := pendingEntries

	replaced := []string{}
	deleted := []string{}

	localIPLookup = func() (string, error) { return localIP, nil }
	arpReplace = func(ip, mac string) error {
		replaced = append(replaced, ip+"="+mac)
		return nil
	}
	arpDelete = func(ip string) error {
		deleted = append(deleted, ip)
		return nil
	}
	preventionMode = false
	preventionTimeout = 500 * time.Millisecond
	pendingEntries = make(map[string]DetectionEvent)

	t.Cleanup(func() {
		localIPLookup = originalLocalIPLookup
		arpReplace = originalARPReplace
		arpDelete = originalARPDelete
		preventionMode = originalPreventionMode
		preventionTimeout = originalPreventionTimeout
		pendingEntries = originalPendingEntries
	})

	return &replaced, &deleted
}

func TestInstallColdStartEntriesOnlyInstallsActiveNonLocalLanMappings(t *testing.T) {
	replaced, _ := withNodeTestHooks(t, "10.5.0.10")

	installed := installColdStartEntries([]ARPEntry{
		{IPAddress: "10.5.0.10", MACAddress: "aa:bb:cc:dd:ee:10"},
		{IPAddress: "10.5.0.11", MACAddress: "aa:bb:cc:dd:ee:11"},
		{IPAddress: "10.5.0.12", MACAddress: "aa:bb:cc:dd:ee:12", IsExpired: true},
		{IPAddress: "192.168.1.20", MACAddress: "aa:bb:cc:dd:ee:20"},
		{IPAddress: "", MACAddress: "aa:bb:cc:dd:ee:30"},
	})

	if installed != 1 {
		t.Fatalf("expected exactly one installed entry, got %d", installed)
	}
	if len(*replaced) != 1 || (*replaced)[0] != "10.5.0.11=aa:bb:cc:dd:ee:11" {
		t.Fatalf("unexpected ARP replace calls: %#v", *replaced)
	}
}

func TestParseColdStartEntriesTreatsEmptyPayloadAsEmptyLedger(t *testing.T) {
	entries, err := parseColdStartEntries(nil)
	if err != nil {
		t.Fatalf("parseColdStartEntries returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries, got %d", len(entries))
	}
}

func TestHandleExpiredEventDeletesCacheEntryAndDoesNotReplace(t *testing.T) {
	replaced, deleted := withNodeTestHooks(t, "10.5.0.10")

	handleARPEvent(DetectionEvent{
		EventType:  "expired",
		IPAddress:  "10.5.0.11",
		MACAddress: "aa:bb:cc:dd:ee:11",
		Timestamp:  time.Now(),
		TrialID:    "trial-expired",
	})

	if len(*deleted) != 1 || (*deleted)[0] != "10.5.0.11" {
		t.Fatalf("unexpected ARP delete calls: %#v", *deleted)
	}
	if len(*replaced) != 0 {
		t.Fatalf("expired event should not replace ARP entries: %#v", *replaced)
	}
}

func TestConfirmARPEntryHonorsPreventionModeDelayAndClearsPendingEntry(t *testing.T) {
	replaced, _ := withNodeTestHooks(t, "10.5.0.10")
	preventionMode = true
	preventionTimeout = 2 * time.Millisecond

	start := time.Now()
	err := confirmARPEntry(DetectionEvent{
		EventType:  "new",
		IPAddress:  "10.5.0.11",
		MACAddress: "aa:bb:cc:dd:ee:11",
		Timestamp:  time.Now(),
	})
	if err != nil {
		t.Fatalf("confirmARPEntry failed: %v", err)
	}
	if time.Since(start) < preventionTimeout {
		t.Fatalf("prevention mode did not wait for configured timeout")
	}
	if _, exists := pendingEntries["10.5.0.11"]; exists {
		t.Fatalf("pending entry was not cleared after prevention timeout")
	}
	if len(*replaced) != 1 {
		t.Fatalf("expected ARP replacement after prevention delay, got %#v", *replaced)
	}
}

func TestAddARPEntryReturnsReplaceError(t *testing.T) {
	withNodeTestHooks(t, "10.5.0.10")
	expected := errors.New("replace failed")
	arpReplace = func(ip, mac string) error { return expected }

	err := addARPEntry("10.5.0.11", "aa:bb:cc:dd:ee:11")
	if !errors.Is(err, expected) {
		t.Fatalf("expected replace error, got %v", err)
	}
}

func TestBackgroundTrafficCanBeDisabledByEnvironment(t *testing.T) {
	t.Setenv("DISABLE_BACKGROUND_TRAFFIC", "true")
	if backgroundTrafficEnabled() {
		t.Fatalf("expected background traffic to be disabled")
	}

	t.Setenv("DISABLE_BACKGROUND_TRAFFIC", "false")
	if !backgroundTrafficEnabled() {
		t.Fatalf("expected background traffic to be enabled")
	}
}
