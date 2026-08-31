package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// Test private keys — the corresponding public keys are embedded in arp-chaincode.go.
const testDHCPPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgj7Es4NtY7dWBCpXu
6NawD+9SpBFvO85UerOqIqPgFL+hRANCAAQuMsYkDBRTQGue0Fu//yxziDOpJf5d
9GO4gyKpUDKqg7bpZKxZdH2cqjNFRV3xQmCZCSKB5vdqtB0MiGNkxf1P
-----END PRIVATE KEY-----`

const testSwitchPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgJQ3c8Kpj9Mrf27Va
fgmbAW1qcAksDAkAZ11P5p9NuhyhRANCAAS77LosTfIucmyZMLOiOLt7Ia0X94pO
pFGSUCocg0BabxVkGE/dlmMfezE7inlmtumlNZF7BGm3OkDZm4bIHs1e
-----END PRIVATE KEY-----`

// signObservation signs content with the PKCS8 ECDSA private key in privKeyPEM
// and returns the base64-encoded ASN.1 DER (r, s) signature.
func signObservation(t *testing.T, privKeyPEM, content string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(privKeyPEM))
	if block == nil {
		t.Fatal("failed to decode private key PEM")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	ecPriv, ok := priv.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatal("private key is not ECDSA")
	}
	hash := sha256.Sum256([]byte(content))
	r, s, err := ecdsa.Sign(rand.Reader, ecPriv, hash[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	derBytes, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
	if err != nil {
		t.Fatalf("failed to marshal signature: %v", err)
	}
	return base64.StdEncoding.EncodeToString(derBytes)
}

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

// makeSignedDHCPObs builds and signs a DHCPObservation for testing.
// LeaseExpiry == 0 means no expiry check in the chaincode.
func makeSignedDHCPObs(t *testing.T, ip, mac, subnet string, leaseExpiry int64) string {
	t.Helper()
	obs := DHCPObservation{
		IPAddress:   ip,
		MACAddress:  mac,
		Subnet:      subnet,
		LeaseExpiry: leaseExpiry,
	}
	obs.Signature = signObservation(t, testDHCPPrivateKeyPEM, dhcpObsContent(obs))
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal DHCPObservation: %v", err)
	}
	return string(b)
}

// makeSignedSwitchObs builds and signs a SwitchObservation for testing.
// Timestamp == 0 means no freshness check in the chaincode.
func makeSignedSwitchObs(t *testing.T, mac, port, vlan string, timestamp int64) string {
	t.Helper()
	obs := SwitchObservation{
		MACAddress: mac,
		Port:       port,
		VLAN:       vlan,
		Timestamp:  timestamp,
	}
	obs.Signature = signObservation(t, testSwitchPrivateKeyPEM, switchObsContent(obs))
	b, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal SwitchObservation: %v", err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Legacy RecordARPEntry tests (unchanged behaviour)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// RegisterBinding tests (Section IV protocol)
// ---------------------------------------------------------------------------

func TestRegisterBindingAcceptsValidObservations(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	dhcpJSON := makeSignedDHCPObs(t, "10.5.0.20", "aa:bb:cc:dd:ee:10", "10.5.0.0/24", 0)
	switchJSON := makeSignedSwitchObs(t, "aa:bb:cc:dd:ee:10", "GigabitEthernet0/1", "100", 0)

	beginTx(t, stub, "tx-register", "trial-register")
	if err := contract.RegisterBinding(ctx, dhcpJSON, switchJSON); err != nil {
		t.Fatalf("RegisterBinding failed: %v", err)
	}
	endTx(stub, "tx-register")

	event := readEvent(t, stub)
	if event.EventType != "new" || event.TrialID != "trial-register" {
		t.Fatalf("expected new event, got %#v", event)
	}

	entry := readCurrentEntry(t, contract, ctx, "10.5.0.20")
	if entry.MACAddress != "aa:bb:cc:dd:ee:10" {
		t.Fatalf("unexpected MAC: %s", entry.MACAddress)
	}
	if entry.SwitchPort != "GigabitEthernet0/1" || entry.VLAN != "100" {
		t.Fatalf("switch observation fields missing: port=%s vlan=%s", entry.SwitchPort, entry.VLAN)
	}
	if entry.Subnet != "10.5.0.0/24" {
		t.Fatalf("subnet missing: %s", entry.Subnet)
	}
	if entry.RecordedBy != "observations" {
		t.Fatalf("unexpected RecordedBy: %s", entry.RecordedBy)
	}
}

func TestRegisterBindingRejectsMACMismatch(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	dhcpJSON := makeSignedDHCPObs(t, "10.5.0.21", "aa:bb:cc:dd:ee:11", "10.5.0.0/24", 0)
	// Switch observation has a different MAC — should be rejected before signatures are checked.
	switchJSON := makeSignedSwitchObs(t, "aa:bb:cc:dd:ee:99", "GigabitEthernet0/1", "100", 0)

	beginTx(t, stub, "tx-mismatch", "trial-mismatch")
	err := contract.RegisterBinding(ctx, dhcpJSON, switchJSON)
	endTx(stub, "tx-mismatch")

	if err == nil {
		t.Fatal("expected RegisterBinding to reject MAC address mismatch")
	}
}

func TestRegisterBindingDetectsSpoofing(t *testing.T) {
	contract := new(SmartContract)
	ctx, stub := newTestContext()

	// Register the legitimate binding.
	dhcpJSON := makeSignedDHCPObs(t, "10.5.0.22", "aa:bb:cc:dd:ee:20", "10.5.0.0/24", 0)
	switchJSON := makeSignedSwitchObs(t, "aa:bb:cc:dd:ee:20", "GigabitEthernet0/1", "100", 0)

	beginTx(t, stub, "tx-legit", "trial-legit")
	if err := contract.RegisterBinding(ctx, dhcpJSON, switchJSON); err != nil {
		t.Fatalf("RegisterBinding (legit) failed: %v", err)
	}
	endTx(stub, "tx-legit")
	_ = readEvent(t, stub)

	// Attempt spoofing: different MAC for the same IP, both observations consistent.
	dhcpJSON2 := makeSignedDHCPObs(t, "10.5.0.22", "aa:bb:cc:dd:ee:ff", "10.5.0.0/24", 0)
	switchJSON2 := makeSignedSwitchObs(t, "aa:bb:cc:dd:ee:ff", "GigabitEthernet0/2", "100", 0)

	beginTx(t, stub, "tx-spoof", "trial-spoof")
	if err := contract.RegisterBinding(ctx, dhcpJSON2, switchJSON2); err != nil {
		t.Fatalf("RegisterBinding (spoof) should not return error (event is emitted): %v", err)
	}
	endTx(stub, "tx-spoof")

	event := readEvent(t, stub)
	if event.EventType != "spoofing" || event.PreviousMAC != "aa:bb:cc:dd:ee:20" {
		t.Fatalf("expected spoofing event, got %#v", event)
	}

	// Authoritative binding must not be overwritten.
	current := readCurrentEntry(t, contract, ctx, "10.5.0.22")
	if current.MACAddress != "aa:bb:cc:dd:ee:20" {
		t.Fatalf("spoofing overwrote authoritative binding: got %s", current.MACAddress)
	}
}
