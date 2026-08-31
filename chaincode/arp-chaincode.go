package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// Trusted public keys for the stub DHCP server and managed edge switch.
// In production these would be provisioned through the Fabric MSP or an
// admin-initialised chaincode state entry.
const dhcpServerPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAELjLGJAwUU0BrntBbv/8sc4gzqSX+
XfRjuIMiqVAyqoO26WSsWXR9nKozRUVd8UJgmQkigeb3arQdDIhjZMX9Tw==
-----END PUBLIC KEY-----`

const switchPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEu+y6LE3yLnJsmTCzoji7eyGtF/eK
TqRRklAqHINAWm8VZBhP3ZZjH3sxO4p5ZrbppTWRewRptzpA2ZuGyB7NXg==
-----END PUBLIC KEY-----`

// switchFreshnessInterval is Δ_S from Section IV: maximum age of a switch
// observation in seconds. Observations older than this are rejected.
const switchFreshnessInterval = int64(60)

type SmartContract struct {
	contractapi.Contract
}

// DHCPObservation is O_D^H = (IP_H, MAC_H, N_H, T_lease) from Section IV-C.
// LeaseExpiry == 0 skips the lease-active check (used in unit tests).
type DHCPObservation struct {
	IPAddress   string `json:"ipAddress"`
	MACAddress  string `json:"macAddress"`
	Subnet      string `json:"subnet"`
	LeaseExpiry int64  `json:"leaseExpiry"` // Unix timestamp; 0 = no check
	Signature   string `json:"signature"`   // base64-encoded ASN.1 ECDSA-P256-SHA256
}

// SwitchObservation is O_{S_i}^H = (MAC_H, P_H, V_H, F_H) from Section IV-C.
// Timestamp == 0 skips the freshness check (used in unit tests).
type SwitchObservation struct {
	MACAddress string `json:"macAddress"`
	Port       string `json:"port"`
	VLAN       string `json:"vlan"`
	Timestamp  int64  `json:"timestamp"` // Unix timestamp; 0 = no check
	Signature  string `json:"signature"` // base64-encoded ASN.1 ECDSA-P256-SHA256
}

type ARPEntry struct {
	IPAddress  string    `json:"ipAddress"`
	MACAddress string    `json:"macAddress"`
	// Fields populated by RegisterBinding (Section IV protocol)
	Subnet          string `json:"subnet,omitempty" metadata:",optional"`
	LeaseExpiry     int64  `json:"leaseExpiry,omitempty" metadata:",optional"`
	SwitchPort      string `json:"switchPort,omitempty" metadata:",optional"`
	VLAN            string `json:"vlan,omitempty" metadata:",optional"`
	SwitchTimestamp int64  `json:"switchTimestamp,omitempty" metadata:",optional"`
	// Fields populated by legacy RecordARPEntry
	Interface  string    `json:"interface,omitempty" metadata:",optional"`
	Hostname   string    `json:"hostname,omitempty" metadata:",optional"`
	Timestamp  time.Time `json:"timestamp"`
	EntryType  string    `json:"entryType,omitempty" metadata:",optional"`
	State      string    `json:"state,omitempty" metadata:",optional"`
	RecordedBy string    `json:"recordedBy,omitempty" metadata:",optional"`
	IsExpired  bool      `json:"isExpired"`
	ExpiredAt  string    `json:"expiredAt,omitempty" metadata:",optional"`
}

type ARPHistory struct {
	IPAddress string     `json:"ipAddress"`
	Entries   []ARPEntry `json:"entries"`
}

type MACChangeResult struct {
	Changed     bool   `json:"changed"`
	PreviousMAC string `json:"previousMAC"`
}

type DetectionEvent struct {
	EventType   string    `json:"eventType"` // "new", "match", "spoofing", "expired"
	IPAddress   string    `json:"ipAddress"`
	MACAddress  string    `json:"macAddress"`
	PreviousMAC string    `json:"previousMAC,omitempty"`
	Hostname    string    `json:"hostname"`
	RecordedBy  string    `json:"recordedBy"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	TrialID     string    `json:"trialId,omitempty"`
}

// dhcpObsContent returns the canonical byte string that the DHCP server signs.
func dhcpObsContent(obs DHCPObservation) string {
	return "dhcp|" + obs.IPAddress + "|" + obs.MACAddress + "|" + obs.Subnet + "|" + strconv.FormatInt(obs.LeaseExpiry, 10)
}

// switchObsContent returns the canonical byte string that the edge switch signs.
func switchObsContent(obs SwitchObservation) string {
	return "switch|" + obs.MACAddress + "|" + obs.Port + "|" + obs.VLAN + "|" + strconv.FormatInt(obs.Timestamp, 10)
}

// verifyECDSA verifies an ECDSA-P256-SHA256 signature over content.
// sig must be a base64-encoded ASN.1 DER (r, s) pair.
func verifyECDSA(pubKeyPEM, content, sig string) error {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return fmt.Errorf("failed to decode public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	var rs struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(sigBytes, &rs); err != nil {
		return fmt.Errorf("failed to unmarshal signature: %v", err)
	}

	hash := sha256.Sum256([]byte(content))
	if !ecdsa.Verify(ecPub, hash[:], rs.R, rs.S) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func emitEvent(ctx contractapi.TransactionContextInterface, event DetectionEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	}
	return ctx.GetStub().SetEvent("ARPDetectionEvent", eventJSON)
}

func getTrialID(ctx contractapi.TransactionContextInterface) string {
	transient, err := ctx.GetStub().GetTransient()
	if err != nil {
		return ""
	}
	return string(transient["trialID"])
}

func appendARPHistory(ctx contractapi.TransactionContextInterface, entry ARPEntry) error {
	historyKey := fmt.Sprintf("HISTORY_%s", entry.IPAddress)
	historyJSON, err := ctx.GetStub().GetState(historyKey)
	if err != nil {
		return err
	}

	var history ARPHistory
	if historyJSON == nil {
		history = ARPHistory{IPAddress: entry.IPAddress, Entries: []ARPEntry{entry}}
	} else {
		if err := json.Unmarshal(historyJSON, &history); err != nil {
			return err
		}
		history.Entries = append(history.Entries, entry)
	}

	historyJSON, err = json.Marshal(history)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(historyKey, historyJSON)
}

// commitBinding classifies the binding attempt (new / match / spoofing), emits
// an ARPDetectionEvent, and writes the authoritative entry to the ledger.
// Spoofing attempts are recorded in history only; they do not overwrite the
// existing authoritative mapping.
func (s *SmartContract) commitBinding(ctx contractapi.TransactionContextInterface, entry ARPEntry) error {
	key := fmt.Sprintf("ARP_%s", entry.IPAddress)
	existingJSON, _ := ctx.GetStub().GetState(key)

	var event DetectionEvent
	event.IPAddress = entry.IPAddress
	event.MACAddress = entry.MACAddress
	event.Hostname = entry.Hostname
	event.RecordedBy = entry.RecordedBy
	event.Timestamp = entry.Timestamp
	event.TrialID = getTrialID(ctx)

	if existingJSON == nil {
		event.EventType = "new"
		event.Message = fmt.Sprintf("New device: %s -> %s", entry.IPAddress, entry.MACAddress)
	} else {
		var existing ARPEntry
		if err := json.Unmarshal(existingJSON, &existing); err != nil {
			return err
		}
		if existing.IsExpired {
			event.EventType = "new"
			event.Message = fmt.Sprintf("Re-registered expired device: %s -> %s", entry.IPAddress, entry.MACAddress)
		} else if existing.MACAddress != entry.MACAddress {
			event.EventType = "spoofing"
			event.PreviousMAC = existing.MACAddress
			event.Message = fmt.Sprintf("MAC CHANGED! %s: %s -> %s", entry.IPAddress, existing.MACAddress, entry.MACAddress)
		} else {
			event.EventType = "match"
			event.Message = fmt.Sprintf("Valid update: %s -> %s", entry.IPAddress, entry.MACAddress)
		}
	}

	fmt.Printf("BENCHMARK_EVENT stage=chaincode_processed trial=%s ip=%s mac=%s ts=%d\n",
		event.TrialID, entry.IPAddress, entry.MACAddress, time.Now().UnixMilli())

	if err := emitEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to emit event: %v", err)
	}

	if event.EventType == "spoofing" {
		return appendARPHistory(ctx, entry)
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := ctx.GetStub().PutState(key, entryJSON); err != nil {
		return err
	}
	return appendARPHistory(ctx, entry)
}

// RegisterBinding is the Section IV binding-registration entry point.
// It validates both signed observations from the DHCP server and managed edge
// switch before committing the authoritative IP-to-MAC binding.
func (s *SmartContract) RegisterBinding(ctx contractapi.TransactionContextInterface,
	dhcpObsJSON string, switchObsJSON string) error {

	var dhcpObs DHCPObservation
	if err := json.Unmarshal([]byte(dhcpObsJSON), &dhcpObs); err != nil {
		return fmt.Errorf("invalid DHCP observation: %v", err)
	}

	var switchObs SwitchObservation
	if err := json.Unmarshal([]byte(switchObsJSON), &switchObs); err != nil {
		return fmt.Errorf("invalid switch observation: %v", err)
	}

	if dhcpObs.MACAddress != switchObs.MACAddress {
		return fmt.Errorf("MAC address mismatch between DHCP and switch observations: %s != %s",
			dhcpObs.MACAddress, switchObs.MACAddress)
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %v", err)
	}
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	if dhcpObs.LeaseExpiry > 0 && now.Unix() > dhcpObs.LeaseExpiry {
		return fmt.Errorf("DHCP lease has expired")
	}

	if switchObs.Timestamp > 0 && now.Unix()-switchObs.Timestamp > switchFreshnessInterval {
		return fmt.Errorf("switch observation is stale (%ds ago, limit %ds)",
			now.Unix()-switchObs.Timestamp, switchFreshnessInterval)
	}

	if err := verifyECDSA(dhcpServerPublicKeyPEM, dhcpObsContent(dhcpObs), dhcpObs.Signature); err != nil {
		return fmt.Errorf("DHCP observation signature invalid: %v", err)
	}

	if err := verifyECDSA(switchPublicKeyPEM, switchObsContent(switchObs), switchObs.Signature); err != nil {
		return fmt.Errorf("switch observation signature invalid: %v", err)
	}

	entry := ARPEntry{
		IPAddress:       dhcpObs.IPAddress,
		MACAddress:      dhcpObs.MACAddress,
		Subnet:          dhcpObs.Subnet,
		LeaseExpiry:     dhcpObs.LeaseExpiry,
		SwitchPort:      switchObs.Port,
		VLAN:            switchObs.VLAN,
		SwitchTimestamp: switchObs.Timestamp,
		Timestamp:       now,
		RecordedBy:      "observations",
	}
	return s.commitBinding(ctx, entry)
}

// RecordARPEntry is the legacy entry point kept for backward compatibility.
func (s *SmartContract) RecordARPEntry(ctx contractapi.TransactionContextInterface,
	ipAddress string, macAddress string, iface string, hostname string,
	entryType string, state string, recordedBy string) error {

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %v", err)
	}
	timestamp := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	entry := ARPEntry{
		IPAddress:  ipAddress,
		MACAddress: macAddress,
		Interface:  iface,
		Hostname:   hostname,
		Timestamp:  timestamp,
		EntryType:  entryType,
		State:      state,
		RecordedBy: recordedBy,
	}
	return s.commitBinding(ctx, entry)
}

func (s *SmartContract) GetCurrentARPEntry(ctx contractapi.TransactionContextInterface,
	ipAddress string) (*ARPEntry, error) {

	key := fmt.Sprintf("ARP_%s", ipAddress)
	entryJSON, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read from ledger: %v", err)
	}
	if entryJSON == nil {
		return nil, fmt.Errorf("ARP entry for %s does not exist", ipAddress)
	}

	var entry ARPEntry
	if err := json.Unmarshal(entryJSON, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *SmartContract) GetARPHistory(ctx contractapi.TransactionContextInterface,
	ipAddress string) (*ARPHistory, error) {

	historyKey := fmt.Sprintf("HISTORY_%s", ipAddress)
	historyJSON, err := ctx.GetStub().GetState(historyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %v", err)
	}
	if historyJSON == nil {
		return nil, fmt.Errorf("no history found for IP %s", ipAddress)
	}

	var history ARPHistory
	if err := json.Unmarshal(historyJSON, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

func (s *SmartContract) GetAllARPEntries(ctx contractapi.TransactionContextInterface) ([]*ARPEntry, error) {
	return s.getAllARPEntries(ctx, false)
}

// GetAllARPEntriesIncludingExpired is the audit path; includes expired entries.
func (s *SmartContract) GetAllARPEntriesIncludingExpired(ctx contractapi.TransactionContextInterface) ([]*ARPEntry, error) {
	return s.getAllARPEntries(ctx, true)
}

func (s *SmartContract) getAllARPEntries(ctx contractapi.TransactionContextInterface, includeExpired bool) ([]*ARPEntry, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("ARP_", "ARP_~")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var entries []*ARPEntry
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var entry ARPEntry
		if err := json.Unmarshal(queryResponse.Value, &entry); err != nil {
			return nil, err
		}
		if entry.IsExpired && !includeExpired {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// ExpireARPEntry marks an ARP entry as no longer authoritative while preserving
// the record and history for audit.
func (s *SmartContract) ExpireARPEntry(ctx contractapi.TransactionContextInterface, ipAddress string) error {
	key := fmt.Sprintf("ARP_%s", ipAddress)
	entryJSON, err := ctx.GetStub().GetState(key)
	if err != nil {
		return fmt.Errorf("failed to read from ledger: %v", err)
	}
	if entryJSON == nil {
		return fmt.Errorf("ARP entry for %s does not exist", ipAddress)
	}

	var entry ARPEntry
	if err := json.Unmarshal(entryJSON, &entry); err != nil {
		return err
	}

	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %v", err)
	}
	timestamp := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))
	entry.IsExpired = true
	entry.ExpiredAt = timestamp.Format(time.RFC3339Nano)

	event := DetectionEvent{
		EventType:  "expired",
		IPAddress:  entry.IPAddress,
		MACAddress: entry.MACAddress,
		Hostname:   entry.Hostname,
		RecordedBy: entry.RecordedBy,
		Timestamp:  timestamp,
		Message:    fmt.Sprintf("Expired device mapping: %s -> %s", entry.IPAddress, entry.MACAddress),
		TrialID:    getTrialID(ctx),
	}

	fmt.Printf("BENCHMARK_EVENT stage=chaincode_processed trial=%s ip=%s mac=%s ts=%d\n",
		event.TrialID, event.IPAddress, event.MACAddress, time.Now().UnixMilli())

	if err := emitEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to emit event: %v", err)
	}

	entryJSON, err = json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := ctx.GetStub().PutState(key, entryJSON); err != nil {
		return err
	}
	return appendARPHistory(ctx, entry)
}

func (s *SmartContract) DetectMACChange(ctx contractapi.TransactionContextInterface,
	ipAddress string, currentMAC string) (*MACChangeResult, error) {

	entry, err := s.GetCurrentARPEntry(ctx, ipAddress)
	if err != nil {
		return nil, err
	}
	return &MACChangeResult{
		Changed:     entry.MACAddress != currentMAC,
		PreviousMAC: entry.MACAddress,
	}, nil
}

func (s *SmartContract) QueryARPByMAC(ctx contractapi.TransactionContextInterface,
	macAddress string) ([]*ARPEntry, error) {

	queryString := fmt.Sprintf(`{"selector":{"macAddress":"%s"}}`, macAddress)
	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var entries []*ARPEntry
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var entry ARPEntry
		if err := json.Unmarshal(queryResponse.Value, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (s *SmartContract) DeleteARPEntry(ctx contractapi.TransactionContextInterface,
	ipAddress string) error {

	key := fmt.Sprintf("ARP_%s", ipAddress)
	return ctx.GetStub().DelState(key)
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		fmt.Printf("Error creating ARP chaincode: %v\n", err)
		return
	}
	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting ARP chaincode: %v\n", err)
	}
}
