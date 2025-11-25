package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct {
	contractapi.Contract
}

// ARPEntry represents an ARP table entry
type ARPEntry struct {
	IPAddress  string    `json:"ipAddress"`
	MACAddress string    `json:"macAddress"`
	Interface  string    `json:"interface"`
	Hostname   string    `json:"hostname"`
	Timestamp  time.Time `json:"timestamp"`
	EntryType  string    `json:"entryType"`  // static, dynamic
	State      string    `json:"state"`      // reachable, stale, delay, probe, failed
	RecordedBy string    `json:"recordedBy"` // which system recorded this
}

// ARPHistory tracks changes to an IP's MAC address over time
type ARPHistory struct {
	IPAddress string     `json:"ipAddress"`
	Entries   []ARPEntry `json:"entries"`
}

// MACChangeResult holds the result of MAC change detection
type MACChangeResult struct {
	Changed     bool   `json:"changed"`
	PreviousMAC string `json:"previousMAC"`
}

// DetectionEvent represents an ARP event for the dashboard
type DetectionEvent struct {
	EventType   string    `json:"eventType"`   // "new", "match", "spoofing"
	IPAddress   string    `json:"ipAddress"`
	MACAddress  string    `json:"macAddress"`
	PreviousMAC string    `json:"previousMAC,omitempty"`
	Hostname    string    `json:"hostname"`
	RecordedBy  string    `json:"recordedBy"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
}

// notifyDashboard sends detection events to Flask dashboard
func notifyDashboard(event DetectionEvent) {
	dashboardURL := "http://localhost:5000/api/event"

	jsonData, err := json.Marshal(event)
	if err != nil {
		return // Silently fail, don't break chaincode
	}

	go func() {
		_, _ = http.Post(dashboardURL, "application/json", bytes.NewBuffer(jsonData))
	}()
}

// RecordARPEntry adds a new ARP entry to the ledger
func (s *SmartContract) RecordARPEntry(ctx contractapi.TransactionContextInterface,
	ipAddress string, macAddress string, iface string, hostname string,
	entryType string, state string, recordedBy string) error {

	// Get transaction timestamp (deterministic across all peers)
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("failed to get transaction timestamp: %v", err)
	}

	timestamp := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	// Check if entry already exists (for detection)
	key := fmt.Sprintf("ARP_%s", ipAddress)
	existingJSON, _ := ctx.GetStub().GetState(key)

	var event DetectionEvent
	event.IPAddress = ipAddress
	event.MACAddress = macAddress
	event.Hostname = hostname
	event.RecordedBy = recordedBy
	event.Timestamp = timestamp

	if existingJSON == nil {
		// NEW DEVICE DETECTED
		event.EventType = "new"
		event.Message = fmt.Sprintf("New device: %s -> %s", ipAddress, macAddress)
	} else {
		// Check for MAC change
		var existingEntry ARPEntry
		json.Unmarshal(existingJSON, &existingEntry)

		if existingEntry.MACAddress != macAddress {
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

	// Send notification to dashboard
	notifyDashboard(event)

	// Store the current entry
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

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(key, entryJSON)
	if err != nil {
		return err
	}

	// Also add to history
	historyKey := fmt.Sprintf("HISTORY_%s", ipAddress)
	historyJSON, err := ctx.GetStub().GetState(historyKey)

	var history ARPHistory
	if historyJSON == nil {
		// First entry for this IP
		history = ARPHistory{
			IPAddress: ipAddress,
			Entries:   []ARPEntry{entry},
		}
	} else {
		err = json.Unmarshal(historyJSON, &history)
		if err != nil {
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

// GetCurrentARPEntry retrieves the current ARP entry for an IP
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

// GetARPHistory retrieves all historical entries for an IP
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

// GetAllARPEntries retrieves all current ARP entries
func (s *SmartContract) GetAllARPEntries(ctx contractapi.TransactionContextInterface) ([]*ARPEntry, error) {
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
		entries = append(entries, &entry)
	}

	return entries, nil
}

// DetectMACChange checks if a MAC address has changed for an IP
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

// QueryARPByMAC finds all IPs associated with a MAC address
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

// DeleteARPEntry removes an ARP entry (for cleanup)
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
