package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	mspID         = os.Getenv("MSP_ID")
	peerEndpoint  = os.Getenv("PEER_ENDPOINT")
	gatewayPeer   = os.Getenv("GATEWAY_PEER")
	channelName   = os.Getenv("CHANNEL_NAME")
	chaincodeName = os.Getenv("CHAINCODE_NAME")
	nodeName      = os.Getenv("NODE_NAME")
	role          = os.Getenv("ROLE")

	cryptoPath  = "/fabric-config/organizations/peerOrganizations/org2.example.com"
	certPath    = cryptoPath + "/users/User1@org2.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org2.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org2.example.com/tls/ca.crt"

	preventionMode    bool
	preventionTimeout time.Duration
	pendingEntries    = make(map[string]DetectionEvent)

	localIPLookup = defaultLocalIPLookup
	arpReplace    = defaultARPReplace
	arpDelete     = defaultARPDelete
)

type ARPEntry struct {
	IPAddress  string `json:"ipAddress"`
	MACAddress string `json:"macAddress"`
	IsExpired  bool   `json:"isExpired"`
}

type DetectionEvent struct {
	EventType   string    `json:"eventType"`
	IPAddress   string    `json:"ipAddress"`
	MACAddress  string    `json:"macAddress"`
	PreviousMAC string    `json:"previousMAC,omitempty"`
	Hostname    string    `json:"hostname"`
	RecordedBy  string    `json:"recordedBy"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	TrialID     string    `json:"trialId,omitempty"`
}

func main() {
	flag.BoolVar(&preventionMode, "prevention-mode", false, "simulate prevention-mode cache holds before ledger confirmation")
	flag.DurationVar(&preventionTimeout, "prevention-timeout", 500*time.Millisecond, "maximum hold time for prevention-mode pending entries")
	flag.Parse()

	log.Println("============================================================")
	log.Printf("  LAN NODE - %s (%s)", nodeName, role)
	log.Println("============================================================")
	log.Printf("Peer: %s (%s)\n", gatewayPeer, peerEndpoint)
	log.Printf("Channel: %s, Chaincode: %s\n", channelName, chaincodeName)
	log.Printf("Prevention mode: %t\n", preventionMode)
	log.Println()

	log.Println("Waiting 15s for Fabric network and router...")
	time.Sleep(15 * time.Second)

	network := connectToFabric()
	defer network.Disconnect()

	log.Println("Connected to blockchain")
	log.Println()

	// Install current authoritative state before listening for live events.
	log.Println("Performing cold-start ARP sync from blockchain...")
	if err := coldStartSync(network.contract); err != nil {
		log.Printf("Cold-start sync failed: %v", err)
	}

	if backgroundTrafficEnabled() {
		go generateTraffic()
	} else {
		log.Println("Background traffic generation disabled")
	}

	log.Println("Subscribing to ARP events...")
	log.Println()
	subscribeToEvents(network)
}

func connectToFabric() *FabricNetwork {
	clientConnection := newGrpcConnection()
	id := newIdentity()
	sign := newSign()

	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	return &FabricNetwork{
		gateway:  gw,
		contract: contract,
		conn:     clientConnection,
	}
}

type FabricNetwork struct {
	gateway  *client.Gateway
	contract *client.Contract
	conn     *grpc.ClientConn
}

func (fn *FabricNetwork) Disconnect() {
	fn.gateway.Close()
	fn.conn.Close()
}

func coldStartSync(contract *client.Contract) error {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := contract.EvaluateWithContext(ctx, "GetAllARPEntries")
	if err != nil {
		return fmt.Errorf("failed to get ARP entries: %w", err)
	}

	entries, err := parseColdStartEntries(result)
	if err != nil {
		return fmt.Errorf("failed to parse ARP entries: %w", err)
	}

	log.Printf("Found %d existing ARP entries", len(entries))

	installed := installColdStartEntries(entries)

	log.Printf("COLD_START_COMPLETE entries=%d elapsed_ms=%d", installed, time.Since(startTime).Milliseconds())
	log.Println()
	return nil
}

func parseColdStartEntries(result []byte) ([]ARPEntry, error) {
	if len(result) == 0 {
		return []ARPEntry{}, nil
	}

	var entries []ARPEntry
	if err := json.Unmarshal(result, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func installColdStartEntries(entries []ARPEntry) int {
	installed := 0
	for _, entry := range entries {
		if entry.IsExpired || entry.IPAddress == "" || entry.MACAddress == "" {
			continue
		}
		if strings.HasPrefix(entry.IPAddress, "10.5.0.") && !isLocalIP(entry.IPAddress) {
			if err := addARPEntry(entry.IPAddress, entry.MACAddress); err != nil {
				log.Printf("Failed to add ARP entry %s -> %s: %v", entry.IPAddress, entry.MACAddress, err)
				continue
			}
			installed++
			log.Printf("Added: %s -> %s", entry.IPAddress, entry.MACAddress)
		}
	}
	return installed
}

func subscribeToEvents(network *FabricNetwork) {
	for {
		if err := listenToEvents(network); err != nil {
			log.Printf("Event subscription ended: %v", err)
		}
		log.Println("Retrying event subscription in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func listenToEvents(network *FabricNetwork) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := network.gateway.GetNetwork(channelName).ChaincodeEvents(ctx, chaincodeName)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	log.Println("Listening for ARP events from blockchain...")
	log.Println()

	for event := range events {
		if event.EventName == "ARPDetectionEvent" {
			var detectionEvent DetectionEvent
			err := json.Unmarshal(event.Payload, &detectionEvent)
			if err != nil {
				log.Printf("Failed to parse event: %v", err)
				continue
			}

			logBenchmarkEvent("node_received", detectionEvent)
			handleARPEvent(detectionEvent)
		}
	}

	return fmt.Errorf("event stream closed")
}

func handleARPEvent(event DetectionEvent) {
	timestamp := event.Timestamp.Format("15:04:05")

	switch event.EventType {
	case "new":
		log.Printf("[%s] NEW DEVICE: %s -> %s", timestamp, event.IPAddress, event.MACAddress)
		if err := confirmARPEntry(event); err != nil {
			log.Printf("Failed to update ARP cache: %v", err)
		}

	case "spoofing":
		log.Printf("[%s] SPOOFING DETECTED!", timestamp)
		log.Printf("         IP: %s", event.IPAddress)
		log.Printf("         Old MAC: %s", event.PreviousMAC)
		log.Printf("         New MAC: %s", event.MACAddress)
		log.Printf("         REJECTING malicious entry!")
		// Do not add to ARP cache; keep the legitimate permanent entry.

	case "match":
		log.Printf("[%s] Valid update: %s -> %s", timestamp, event.IPAddress, event.MACAddress)
		if err := confirmARPEntry(event); err != nil {
			log.Printf("Failed to update ARP cache: %v", err)
		}

	case "expired":
		if err := removeARPEntry(event.IPAddress); err != nil {
			log.Printf("Failed to remove expired ARP entry: %v", err)
		} else {
			log.Printf("ENTRY_EXPIRED ip=%s mac=%s", event.IPAddress, event.MACAddress)
			logBenchmarkEvent("node_cache_updated", event)
		}

	default:
		log.Printf("[%s] Unknown event: %s", timestamp, event.EventType)
	}

	log.Println()
}

func confirmARPEntry(event DetectionEvent) error {
	if preventionMode {
		pendingEntries[event.IPAddress] = event
		timer := time.NewTimer(preventionTimeout)
		<-timer.C
		delete(pendingEntries, event.IPAddress)
	}

	if err := addARPEntry(event.IPAddress, event.MACAddress); err != nil {
		return err
	}
	logBenchmarkEvent("node_cache_updated", event)
	return nil
}

func addARPEntry(ip, mac string) error {
	if isLocalIP(ip) {
		return nil
	}
	return arpReplace(ip, mac)
}

func removeARPEntry(ip string) error {
	if isLocalIP(ip) {
		return nil
	}

	return arpDelete(ip)
}

func logBenchmarkEvent(stage string, event DetectionEvent) {
	log.Printf("BENCHMARK_EVENT stage=%s trial=%s ip=%s mac=%s ts=%d",
		stage, event.TrialID, event.IPAddress, event.MACAddress, time.Now().UnixMilli())
}

func isLocalIP(ip string) bool {
	localIP, err := localIPLookup()
	if err != nil {
		return false
	}
	return ip == localIP
}

func defaultLocalIPLookup() (string, error) {
	cmd := exec.Command("hostname", "-i")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func defaultARPReplace(ip, mac string) error {
	cmd := exec.Command("ip", "neigh", "replace", ip, "lladdr", mac, "dev", "eth0", "nud", "permanent")
	return cmd.Run()
}

func defaultARPDelete(ip string) error {
	cmd := exec.Command("ip", "neigh", "del", ip, "dev", "eth0")
	return cmd.Run()
}

func backgroundTrafficEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("DISABLE_BACKGROUND_TRAFFIC")))
	return value != "1" && value != "true" && value != "yes"
}

func generateTraffic() {
	time.Sleep(20 * time.Second)

	log.Println("Starting background traffic generation...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Ping router to generate ARP traffic
		cmd := exec.Command("ping", "-c", "1", "-W", "1", "10.5.0.1")
		cmd.Run() // Ignore errors

		// Ping other nodes
		for i := 10; i <= 12; i++ {
			ip := fmt.Sprintf("10.5.0.%d", i)
			if !isLocalIP(ip) {
				cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
				cmd.Run()
			}
		}
	}
}

func newGrpcConnection() *grpc.ClientConn {
	cert, err := loadCertificate(tlsCertPath)
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		log.Fatalf("Failed to create gRPC connection: %v", err)
	}

	return connection
}

func newIdentity() *identity.X509Identity {
	cert, err := loadCertificate(certPath)
	if err != nil {
		log.Fatalf("Failed to load certificate: %v", err)
	}

	id, err := identity.NewX509Identity(mspID, cert)
	if err != nil {
		log.Fatalf("Failed to create identity: %v", err)
	}

	return id
}

func newSign() identity.Sign {
	files, err := ioutil.ReadDir(keyPath)
	if err != nil {
		log.Fatalf("Failed to read private key directory: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("No private key found in %s", keyPath)
	}

	privateKeyPEM, err := ioutil.ReadFile(filepath.Join(keyPath, files[0].Name()))
	if err != nil {
		log.Fatalf("Failed to read private key file: %v", err)
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
	}

	return sign
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}
