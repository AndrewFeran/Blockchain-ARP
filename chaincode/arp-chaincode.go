package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct {
	contractapi.Contract
}

type ARPEntry struct {
	IPAddress  string    `json:"ipAddress"`
	MACAddress string    `json:"macAddress"`
	Interface  string    `json:"interface"`
	Hostname   string    `json:"hostname"`
	Timestamp  time.Time `json:"timestamp"`
	EntryType  string    `json:"entryType"`  // static, dynamic
	State      string    `json:"state"`      // reachable, stale, delay, probe, failed
	RecordedBy string    `json:"recordedBy"` // which system recorded this
	IsExpired  bool      `json:"isExpired"`
	ExpiredAt  string    `json:"expiredAt"`
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

func emitEvent(ctx contractapi.TransactionContextInterface, event DetectionEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	err = ctx.GetStub().SetEvent("ARPDetectionEvent", eventJSON)
	if err != nil {
		return fmt.Errorf("failed to set event: %v", err)
	}

	return nil
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
		history = ARPHistory{
			IPAddress: entry.IPAddress,
			Entries:   []ARPEntry{entry},
		}
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

func (s *SmartContract) RecordARPEntry(ctx contractapi.TransactionContextInterface,
	ipAddress string, macAddress string, iface string, hostname string,
	entryType string, state string, recordedBy string) error {

	// Get transaction timestamp (deterministic across all peers)
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %v", err)
	}

	timestamp := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	key := fmt.Sprintf("ARP_%s", ipAddress)
	existingJSON, _ := ctx.GetStub().GetState(key)

	var event DetectionEvent
	event.IPAddress = ipAddress
	event.MACAddress = macAddress
	event.Hostname = hostname
	event.RecordedBy = recordedBy
	event.Timestamp = timestamp
	event.TrialID = getTrialID(ctx)

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

	if existingJSON == nil {
		// NEW DEVICE DETECTED
		event.EventType = "new"
		event.Message = fmt.Sprintf("New device: %s -> %s", ipAddress, macAddress)
	} else {
		// Check for MAC change
		var existingEntry ARPEntry
		if err := json.Unmarshal(existingJSON, &existingEntry); err != nil {
			return err
		}

		if existingEntry.IsExpired {
			event.EventType = "new"
			event.Message = fmt.Sprintf("Re-registered expired device: %s -> %s", ipAddress, macAddress)
		} else if existingEntry.MACAddress != macAddress {
			// ARP SPOOFING DETECTED!
			event.EventType = "spoofing"
			event.PreviousMAC = existingEntry.MACAddress
			event.Message = fmt.Sprintf("MAC CHANGED! %s: %s -> %s", ipAddress, existingEntry.MACAddress, macAddress)
		} else {
			// Match - normal update
			event.EventType = "match"
			event.Message = fmt.Sprintf("Valid update: %s -> %s", ipAddress, macAddress)
		}
	}

	fmt.Printf("BENCHMARK_EVENT stage=chaincode_processed trial=%s ip=%s mac=%s ts=%d\n",
		event.TrialID, ipAddress, macAddress, time.Now().UnixMilli())

	err = emitEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to emit event: %v", err)
	}

	// A spoofing event is preserved in history, but does not replace the
	// current authoritative mapping used by cold-start sync.
	if event.EventType == "spoofing" {
		return appendARPHistory(ctx, entry)
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(key, entryJSON)
	if err != nil {
		return err
	}

	return appendARPHistory(ctx, entry)
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
	err = json.Unmarshal(entryJSON, &entry)
	if err != nil {
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
	err = json.Unmarshal(historyJSON, &history)
	if err != nil {
		return nil, err
	}

	return &history, nil
}

func (s *SmartContract) GetAllARPEntries(ctx contractapi.TransactionContextInterface) ([]*ARPEntry, error) {
	return s.getAllARPEntries(ctx, false)
}

// GetAllARPEntriesIncludingExpired is the audit path - includes expired entries that GetAllARPEntries skips.
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
		err = json.Unmarshal(queryResponse.Value, &entry)
		if err != nil {
			return nil, err
		}
		if entry.IsExpired && !includeExpired {
			continue
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// ExpireARPEntry marks an ARP entry as no longer authoritative while keeping
// the record and history available for audit.
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

	result := &MACChangeResult{
		Changed:     entry.MACAddress != currentMAC,
		PreviousMAC: entry.MACAddress,
	}

	return result, nil
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
		err = json.Unmarshal(queryResponse.Value, &entry)
		if err != nil {
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
