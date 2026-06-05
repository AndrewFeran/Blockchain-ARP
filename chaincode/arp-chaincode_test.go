package main

import (
	"encoding/json"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func newTestContext() (*contractapi.TransactionContext, *shimtest.MockStub) {
	stub := shimtest.NewMockStub("arp", nil)
	ctx := new(contractapi.TransactionContext)
	ctx.SetStub(stub)
	return ctx, stub
}

func beginTx(t *testing.T, stub *shimtest.MockStub, txID, trialID string) {
	t.Helper()
	stub.MockTransactionStart(txID)
	if err := stub.SetTransient(map[string][]byte{"trialID": []byte(trialID)}); err != nil {
		t.Fatalf("SetTransient failed: %v", err)
	}
}

func endTx(stub *shimtest.MockStub, txID string) {
	stub.MockTransactionEnd(txID)
}

func readCurrentEntry(t *testing.T, contract *SmartContract, ctx *contractapi.TransactionContext, ip string) ARPEntry {
	t.Helper()
	entry, err := contract.GetCurrentARPEntry(ctx, ip)
	if err != nil {
		t.Fatalf("GetCurrentARPEntry failed: %v", err)
	}
	return *entry
}

func readEvent(t *testing.T, stub *shimtest.MockStub) DetectionEvent {
	t.Helper()
	select {
	case event := <-stub.ChaincodeEventsChannel:
		if event.EventName != "ARPDetectionEvent" {
			t.Fatalf("unexpected event name: %s", event.EventName)
		}
		var payload DetectionEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("failed to unmarshal event payload: %v", err)
		}
		return payload
	default:
		t.Fatal("expected chaincode event, got none")
	}
	return DetectionEvent{}
}

func TestRecordARPEntryRejectsSpoofingWithoutOverwritingAuthoritativeMapping(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	beginTx(t, stub, "tx-new", "trial-new")
	if err := contract.RecordARPEntry(ctx, "10.5.0.10", "aa:bb:cc:dd:ee:01", "eth0", "node1", "dynamic", "reachable", "router"); err != nil {
		t.Fatalf("RecordARPEntry new failed: %v", err)
	}
	endTx(stub, "tx-new")
	event := readEvent(t, stub)
	if event.EventType != "new" || event.TrialID != "trial-new" {
		t.Fatalf("new event mismatch: %#v", event)
	}

	beginTx(t, stub, "tx-spoof", "trial-spoof")
	if err := contract.RecordARPEntry(ctx, "10.5.0.10", "aa:bb:cc:dd:ee:99", "eth0", "attacker", "dynamic", "reachable", "router"); err != nil {
		t.Fatalf("RecordARPEntry spoof failed: %v", err)
	}
	endTx(stub, "tx-spoof")
	event = readEvent(t, stub)
	if event.EventType != "spoofing" || event.PreviousMAC != "aa:bb:cc:dd:ee:01" || event.TrialID != "trial-spoof" {
		t.Fatalf("spoofing event mismatch: %#v", event)
	}

	current := readCurrentEntry(t, contract, ctx, "10.5.0.10")
	if current.MACAddress != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("spoofing overwrote authoritative MAC: got %s", current.MACAddress)
	}

	history, err := contract.GetARPHistory(ctx, "10.5.0.10")
	if err != nil {
		t.Fatalf("GetARPHistory failed: %v", err)
	}
	if len(history.Entries) != 2 {
		t.Fatalf("expected history to preserve legitimate and spoofed observations, got %d entries", len(history.Entries))
	}
	if history.Entries[1].MACAddress != "aa:bb:cc:dd:ee:99" {
		t.Fatalf("expected spoofed observation in history, got %s", history.Entries[1].MACAddress)
	}
}

func TestExpireARPEntryFiltersColdStartResultsAndEmitsExpiredEvent(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	beginTx(t, stub, "tx-new", "trial-new")
	if err := contract.RecordARPEntry(ctx, "10.5.0.11", "aa:bb:cc:dd:ee:02", "eth0", "node2", "dynamic", "reachable", "router"); err != nil {
		t.Fatalf("RecordARPEntry new failed: %v", err)
	}
	endTx(stub, "tx-new")
	_ = readEvent(t, stub)

	beginTx(t, stub, "tx-expire", "trial-expire")
	if err := contract.ExpireARPEntry(ctx, "10.5.0.11"); err != nil {
		t.Fatalf("ExpireARPEntry failed: %v", err)
	}
	endTx(stub, "tx-expire")
	event := readEvent(t, stub)
	if event.EventType != "expired" || event.IPAddress != "10.5.0.11" || event.TrialID != "trial-expire" {
		t.Fatalf("expired event mismatch: %#v", event)
	}

	current := readCurrentEntry(t, contract, ctx, "10.5.0.11")
	if !current.IsExpired || current.ExpiredAt == "" {
		t.Fatalf("expected current entry to be marked expired, got %#v", current)
	}

	active, err := contract.GetAllARPEntries(ctx)
	if err != nil {
		t.Fatalf("GetAllARPEntries failed: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected expired entry to be excluded from cold-start query, got %d entries", len(active))
	}

	audit, err := contract.GetAllARPEntriesIncludingExpired(ctx)
	if err != nil {
		t.Fatalf("GetAllARPEntriesIncludingExpired failed: %v", err)
	}
	if len(audit) != 1 || !audit[0].IsExpired {
		t.Fatalf("expected audit query to include expired entry, got %#v", audit)
	}
}

func TestRecordARPEntryReregistersExpiredMappingAsNewAuthoritativeEntry(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	beginTx(t, stub, "tx-new", "trial-new")
	if err := contract.RecordARPEntry(ctx, "10.5.0.12", "aa:bb:cc:dd:ee:03", "eth0", "node3", "dynamic", "reachable", "router"); err != nil {
		t.Fatalf("RecordARPEntry new failed: %v", err)
	}
	endTx(stub, "tx-new")
	_ = readEvent(t, stub)

	beginTx(t, stub, "tx-expire", "trial-expire")
	if err := contract.ExpireARPEntry(ctx, "10.5.0.12"); err != nil {
		t.Fatalf("ExpireARPEntry failed: %v", err)
	}
	endTx(stub, "tx-expire")
	_ = readEvent(t, stub)

	beginTx(t, stub, "tx-reregister", "trial-reregister")
	if err := contract.RecordARPEntry(ctx, "10.5.0.12", "aa:bb:cc:dd:ee:44", "eth0", "node3-new-nic", "dynamic", "reachable", "router"); err != nil {
		t.Fatalf("RecordARPEntry reregister failed: %v", err)
	}
	endTx(stub, "tx-reregister")
	event := readEvent(t, stub)
	if event.EventType != "new" || event.TrialID != "trial-reregister" {
		t.Fatalf("expected re-registration as new event, got %#v", event)
	}

	current := readCurrentEntry(t, contract, ctx, "10.5.0.12")
	if current.IsExpired || current.ExpiredAt != "" || current.MACAddress != "aa:bb:cc:dd:ee:44" {
		t.Fatalf("expected fresh authoritative entry after re-registration, got %#v", current)
	}
}
